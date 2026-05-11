package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os/signal"
	"syscall"
	"time"

	"github.com/jacksontj/dnms/aggregator"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/memberlist"
	"github.com/sirupsen/logrus"
)

func main() {
	// Some CLI args for better testing

	advertiseStr := flag.String("gossipAddr", "", "address to advertise gossip on")
	peerStr := flag.String("peer", "", "address to gossip with")
	aggNode := flag.Bool("aggregator", false, "are you an aggregator node?")
	httpAddr := flag.String("httpAddr", ":12345", "address for HTTP API")
	pingPort := flag.Int("pingPort", 33435, "UDP port for ping/ack transport (must be the same on every cluster node)")

	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// #TODO: load from a config file
	cfg := memberlist.DefaultLANConfig()

	// TODO: load from config
	cfg.BindPort = 33434
	cfg.AdvertisePort = 33434
	if *advertiseStr != "" {
		cfg.AdvertiseAddr = *advertiseStr
	} else {
		i, err := GetLocalIP()
		if err != nil {
			logrus.Fatalf("Err: %v", err)
		}
		cfg.AdvertiseAddr = i
	}
	logrus.Infof("AdvertiseAddr: %v", cfg.AdvertiseAddr)

	// Start the mapper (at this point no peers-- so it will do nothing)
	m := mapper.NewMapper(ctx, cfg.AdvertiseAddr)
	m.Start(ctx)

	// TODO pass additional config
	// Start HTTP APIs
	mux := http.NewServeMux()
	api := NewHTTPApi(m)
	api.Start(ctx, mux)

	// If we are an aggregator start that
	var aggMap *aggregator.AggGraphMap
	if *aggNode {
		aggMap = aggregator.NewAggGraphMap(ctx)
		aggApi := aggregator.NewHTTPApi(aggMap)
		aggApi.Start(ctx, mux)
		// TODO: through something better than http, it is local after all
		// subscribe to ourself
		aggMap.AddPeer("127.0.0.1")
	}

	srv := &http.Server{Addr: *httpAddr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.Errorf("HTTP server error: %v", err)
		}
	}()

	// Wire up the delegate-- he'll handle pings and node up/down events
	delegate := NewDNMSDelegate(m, aggMap)
	cfg.Delegate = delegate
	cfg.Events = delegate

	// Create the memberlist with the config we just made
	mlist, err := memberlist.Create(cfg)
	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}
	delegate.SetMlist(mlist)

	// TODO: background thing to join if we end up alone?
	// or if people disconnect
	// Join if we can
	if *peerStr != "" {
		mlist.Join([]string{*peerStr})
	}

	// start the dedicated ping transport
	transport := NewPingTransport(*pingPort)
	if err := transport.Listen(ctx); err != nil {
		logrus.Fatalf("Unable to start ping transport: %v", err)
	}

	// start the pinger
	p := Pinger{
		M: m,
		T: transport,
		Self: mapper.Peer{
			Name: mlist.LocalNode().Addr.String(),
			Port: int(mlist.LocalNode().Port),
		},
	}
	p.Start(ctx)

	// print state of the world for ease of debugging
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logrus.Infof("peers=%d nodes=%d links=%d routes=%d",
					mlist.NumMembers()-1,
					m.Graph.GetNodeCount(),
					m.Graph.GetLinkCount(),
					m.Graph.GetRouteCount(),
				)
			}
		}
	}()

	<-ctx.Done()
	logrus.Infof("shutdown requested, draining...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logrus.Warnf("HTTP server shutdown: %v", err)
	}
	if err := mlist.Leave(2 * time.Second); err != nil {
		logrus.Warnf("memberlist leave: %v", err)
	}
	if err := mlist.Shutdown(); err != nil {
		logrus.Warnf("memberlist shutdown: %v", err)
	}
}

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

	"github.com/hashicorp/memberlist"
	"github.com/jacksontj/dnms/aggregator"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/dnms/webui"
	"github.com/sirupsen/logrus"
)

func main() {
	// Some CLI args for better testing

	advertiseStr := flag.String("gossipAddr", "", "address to advertise gossip on")
	peerStr := flag.String("peer", "", "address to gossip with")
	aggNode := flag.Bool("aggregator", false, "are you an aggregator node?")
	httpAddr := flag.String("httpAddr", ":12345", "address for HTTP API")
	pingPort := flag.Int("pingPort", 33435, "UDP port for ping/ack transport (must be the same on every cluster node)")
	mapInterval := flag.Duration("mapInterval", time.Second, "pause between traceroutes inside a single source-port sweep")
	mapSrcPortStart := flag.Int("mapSrcPortStart", 0, "first source port for traceroute mapping; 0 → pingPort+1")
	mapSrcPortEnd := flag.Int("mapSrcPortEnd", 0, "last source port (exclusive) for traceroute mapping; 0 → mapSrcPortStart+10")
	pingPeerInterval := flag.Duration("pingPeerInterval", 100*time.Millisecond, "pause between successive peers in a ping sweep")
	pingRouteInterval := flag.Duration("pingRouteInterval", time.Second, "pause between successive routes when pinging one peer")
	pingTimeout := flag.Duration("pingTimeout", time.Second, "ack timeout for a single ping")
	metricRingSize := flag.Int("metricRingSize", 100, "number of recent ping samples retained per route")
	httpAllowOrigin := flag.String("httpAllowOrigin", "*", "value for Access-Control-Allow-Origin; set to a specific origin to lock down CORS")
	httpToken := flag.String("httpToken", "", "if non-empty, require Authorization: Bearer <token> on every HTTP API request")
	aggToken := flag.String("aggregatorToken", "", "token the aggregator sends in Authorization headers when subscribing to peers; defaults to -httpToken")

	flag.Parse()

	if *mapSrcPortStart == 0 {
		*mapSrcPortStart = *pingPort + 1
	}
	if *mapSrcPortEnd == 0 {
		*mapSrcPortEnd = *mapSrcPortStart + 10
	}
	if *mapSrcPortStart <= *pingPort && *pingPort < *mapSrcPortEnd {
		logrus.Fatalf("mapSrcPort range [%d,%d) overlaps the ping transport listen port %d; pick a non-overlapping range",
			*mapSrcPortStart, *mapSrcPortEnd, *pingPort)
	}
	graph.MetricRingSize = *metricRingSize

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
	m.Interval = *mapInterval
	m.SrcPortStart = *mapSrcPortStart
	m.SrcPortEnd = *mapSrcPortEnd
	m.Start(ctx)

	// TODO pass additional config
	// Start HTTP APIs
	mux := http.NewServeMux()
	api := NewHTTPApi(m)
	api.Start(ctx, mux)

	// Browser dashboard. http.FileServer is registered with the trailing
	// slash so it matches everything not claimed by /v1 routes — index.html
	// is served at the root, assets resolve by name.
	mux.Handle("/", webui.Handler())

	// If we are an aggregator start that
	var aggMap *aggregator.AggGraphMap
	if *aggNode {
		token := *aggToken
		if token == "" {
			token = *httpToken
		}
		aggMap = aggregator.NewAggGraphMap(ctx)
		aggMap.SubscribeToken = token
		aggApi := aggregator.NewHTTPApi(aggMap)
		aggApi.Start(ctx, mux)
		// TODO: through something better than http, it is local after all
		// subscribe to ourself
		aggMap.AddPeer("127.0.0.1")
	}

	// Only gate the JSON API behind the token, not the static dashboard. The
	// JS itself isn't sensitive (it's the same code the operator can browse
	// directly) and must load before it can prompt the user for a token.
	sec := HTTPSecurity{
		AllowOrigin:       *httpAllowOrigin,
		Token:             *httpToken,
		ProtectedPrefixes: []string{"/v1/"},
	}
	srv := &http.Server{Addr: *httpAddr, Handler: sec.Wrap(mux)}
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
		PeerInterval:  *pingPeerInterval,
		RouteInterval: *pingRouteInterval,
		Timeout:       *pingTimeout,
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

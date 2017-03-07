package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/aggregator"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/memberlist"
)

func main() {
	// Some CLI args for better testing

	advertiseStr := flag.String("gossipAddr", "", "address to advertise gossip on")
	peerStr := flag.String("peer", "", "address to gossip with")
	aggNode := flag.Bool("aggregator", false, "are you an aggregator node?")

	flag.Parse()

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
	m := mapper.NewMapper(cfg.AdvertiseAddr)
	m.Start()

	// TODO pass additional config
	// Start HTTP APIs
	mux := http.NewServeMux()
	api := NewHTTPApi(m)
	api.Start(mux)

	// If we are an aggregator start that
	var aggMap *aggregator.AggGraphMap
	if *aggNode {
		aggMap = aggregator.NewAggGraphMap()
		api := aggregator.NewHTTPApi(aggMap)
		api.Start(mux)
	}

	go http.ListenAndServe(":12345", mux)
	if *aggNode {
		// TODO: through something better than http, it is local after all
		// subscribe to ourself
		aggMap.AddPeer("127.0.0.1")
	}

	// Wire up the delegate-- he'll handle pings and node up/down events
	delegate := NewDNMSDelegate(m, aggMap)
	cfg.Delegate = delegate
	cfg.Events = delegate

	// Create the memberlist with the config we just made
	mlist, err := memberlist.Create(cfg)
	delegate.Mlist = mlist
	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}

	// TODO: background thing to join if we end up alone?
	// or if people disconnect
	// Join if we can
	mlist.Join([]string{*peerStr})

	// start the pinger
	p := Pinger{
		M: m,
		Self: mapper.Peer{
			Name: mlist.LocalNode().Addr.String(),
			Port: int(mlist.LocalNode().Port),
		},
	}
	p.Start()

	// print state of the world for ease of debugging
	for {
		time.Sleep(time.Second)
		logrus.Infof("peers=%d nodes=%d links=%d routes=%d",
			mlist.NumMembers()-1,
			m.Graph.GetNodeCount(),
			m.Graph.GetLinkCount(),
			m.Graph.GetRouteCount(),
		)
	}

}

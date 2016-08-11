package main

import (
	"flag"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/memberlist"
)

func main() {
	// Some CLI args for better testing

	advertiseStr := flag.String("gossipAddr", "", "address to advertise gossip on")
	peerStr := flag.String("peer", "", "address to gossip with")

	flag.Parse()

	// Start the mapper (at this point no peers-- so it will do nothing)
	m := mapper.NewMapper()
	m.Start()

	// #TODO: load from a config file
	cfg := memberlist.DefaultLANConfig()

	// Wire up the delegate-- he'll handle pings and node up/down events
	delegate := NewDNMSDelegate(m)
	cfg.Delegate = delegate
	cfg.Events = delegate

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

	// Create the memberlist with the config we just made
	mlist, err := memberlist.Create(cfg)
	delegate.Mlist = mlist
	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}

	// TODO: background thing to join if we end up alone?
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

	// TODO pass additional config
	// Start HTTP API
	api := NewHTTPApi(m)
	api.Start()

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

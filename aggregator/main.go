package main

import (
	"encoding/json"
	"flag"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/eventsource"
	"github.com/jacksontj/memberlist"
)

// Subscripe to dest, consuming events into PeerGraphMap.
// We'll return a bool channel which can be used to cancel the subscription
func subscribe(p *PeerGraphMap, dest string) chan bool {
	exitChan := make(chan bool)
	go func() {
		stream, err := eventsource.Subscribe(dest, "")
		if err != nil {
			logrus.Fatalf("Error subscribing: %v", err)
		}
		for {
			select {
			case ev := <-stream.Events:
				// TODO: care about more events
				switch ev.Event() {
				case "addRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					p.addRoute("peer", &r)
				case "removeRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					p.removeRoute("peer", &r)
				}
			case <-exitChan:
				return
			}
		}
	}()
	return exitChan
}

// The goal here is to create an agent that will aggregate all of the network data
// the various agents gather.
//
// To do this, the aggregator will join the memberlist with a specific nodemeta flag
// saying he is an aggregator. Then the various nodes will start pushing events to
// this aggregator. This aggregator will keep track of which routes etc. where
// added by which peers-- so in the event of a peer loss, we can simply remove all
// the things only owned by that now dead peer
// TODO: push from agent
func main() {
	p := NewPeerGraphMap()
	logrus.Infof("Aggregator!")

	api := NewHTTPApi(p)
	api.Start()

	// TODO: gossip to get peers
	// join memberlist
	advertiseStr := flag.String("gossipAddr", "", "address to advertise gossip on")
	peerStr := flag.String("peer", "", "address to gossip with")

	flag.Parse()

	// #TODO: load from a config file
	cfg := memberlist.DefaultLANConfig()

	// TODO: load from config
	cfg.BindPort = 33334
	cfg.AdvertisePort = 33334
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

	// Wire up the delegate-- he'll handle pings and node up/down events
	delegate := NewAggregatorDelegate(p)
	cfg.Delegate = delegate
	cfg.Events = delegate

	// TODO: can't conflict with our other name...
	cfg.Name = "aggregator"

	// Create the memberlist with the config we just made
	mlist, err := memberlist.Create(cfg)
	delegate.Mlist = mlist
	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}

	// TODO: set aggregator flag in nodemeta

	// TODO: background thing to join if we end up alone?
	// Join if we can
	mlist.Join([]string{*peerStr})

	//subscribe(p, "http://127.0.0.1:12345/v1/events/graph")
	// print state of the world for ease of debugging
	for {
		time.Sleep(time.Second)
		logrus.Infof("peers=%d nodes=%d links=%d routes=%d",
			mlist.NumMembers()-1,
			p.Graph.GetNodeCount(),
			p.Graph.GetLinkCount(),
			p.Graph.GetRouteCount(),
		)
	}

}

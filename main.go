package main

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/traceroute"
)

func tracerouteExample() {
	options := traceroute.TracerouteOptions{}
	options.SetDstPort(33434) // TODO: config

	ret, err := traceroute.Traceroute(
		"www.google.com",
		&options,
	)
	if err != nil {
		logrus.Infof("Traceroute err: %v", err)
	} else {
		logrus.Infof("Traceroute: %v", ret)

	}
}

func mapper(g *graph.NetworkGraph, mlist *memberlist.Memberlist) {
	for {
		nodes := mlist.Members()

		for _, node := range nodes {
			// if this is ourselves, skip!
			if node == mlist.LocalNode() {
				continue
			}

			// Otherwise, lets do some stuff
			logrus.Infof("get routes to peer: %v %v", node.Addr, node)

			options := traceroute.TracerouteOptions{}
			options.SetSrcPort(33435) // TODO: config
			options.SetDstPort(33434) // TODO: config

			ret, err := traceroute.Traceroute(
				"173.194.72.147",  // a specific IP-- so we can test
				&options,
			)
			if err != nil {
				logrus.Infof("Traceroute err: %v", err)
				continue
			}

			logrus.Info("Traceroute: complete")

			path := make([]string, 0, len(ret.Hops))

			for _, hop := range ret.Hops {
				path = append(path, hop.AddressString())
			}

			//src, _ := net.ResolveUDPAddr("udp", "localhost:33434")
			//dst, _ := net.ResolveUDPAddr("udp", "www.google.com:33434")

			// TODO: store src/dst -> return here
			// This will be our map of how we can send stuff across the network
			g.IncrRoute(path)

			// TODO configurable rate
			time.Sleep(time.Second * 5)
		}
	}

}

func main() {
	g := graph.Create()

	/*
		This is the main daemon. Which has the following responsibilities:
			- traceroute: Graph the network
			- ping: keep track of latency/jitter/loss across links
			- aggregate: to centralized location for better fault detection
			- coordinate: split the above work to scale better
	*/

	// #TODO: load from a config file
	cfg := memberlist.DefaultLANConfig()

	mlist, err := memberlist.Create(cfg)

	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}

	mlist.Join([]string{"127.0.0.1:55555"})

	go mapper(g, mlist)

	for {
		time.Sleep(time.Second)
		logrus.Infof("peers=%d nodes=%d links=%d routes=%d",
			mlist.NumMembers()-1,
			g.GetNodeCount(),
			g.GetLinkCount(),
			g.GetRouteCount(),
		)
	}

}

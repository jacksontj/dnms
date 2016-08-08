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

func mapper(routeMap *RouteMap, g *graph.NetworkGraph, mlist *memberlist.Memberlist) {
	srcPort := 33435
	dstName := "173.194.72.147" // a specific IP-- so we can test
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
			options.SetSrcPort(srcPort) // TODO: config
			options.SetDstPort(33434)   // TODO: config

			ret, err := traceroute.Traceroute(
				dstName,
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

			currRoute := routeMap.GetRouteOption(srcPort, dstName)

			// If we don't have a current route, or the paths differ-- lets update
			if currRoute == nil || !currRoute.SamePath(path) {
				// Add new one
				routeMap.UpdateRouteOption(srcPort, dstName, g.IncrRoute(path))

				// Remove old one if it exists
				if currRoute != nil {
					g.DecrRoute(currRoute.Hops())
				}
			}

			// TODO configurable rate
			time.Sleep(time.Second * 5)
		}
	}

}

func main() {
	g := graph.Create()
	routeMap := NewRouteMap()

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

	go mapper(routeMap, g, mlist)

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

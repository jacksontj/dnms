package main

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"
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

func mapper(mlist *memberlist.Memberlist) {
	for {
		nodes := mlist.Members()

		for _, node := range nodes {
			// if this is ourselves, skip!
			if node == mlist.LocalNode() {
				continue
			}

			// Otherwise, lets do some stuff
			logrus.Infof("get routes to peer: %v %v", node.Addr, node)
		}
		// TODO configurable rate
		time.Sleep(time.Second * 5)
	}

}

func main() {
	// TODO: remove
	tracerouteExample()

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

	go mapper(mlist)

	for {
		time.Sleep(time.Second)
		logrus.Infof("peers: count=%d", mlist.NumMembers()-1)
	}

}

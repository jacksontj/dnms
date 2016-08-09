package main

import (
	"encoding/json"
	"flag"
	"net/http"
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

	m := mapper.NewMapper()
	m.Start()

	// #TODO: load from a config file
	cfg := memberlist.DefaultLANConfig()

	delegate := NewDNMSDelegate(m)
	cfg.Delegate = delegate
	cfg.Events = delegate

	// TODO: load from config
	cfg.BindPort = 33434
	cfg.AdvertisePort = 33434

	if *advertiseStr != "" {
		cfg.AdvertiseAddr = *advertiseStr
		logrus.Infof("addr: %v", *advertiseStr)
	}

	mlist, err := memberlist.Create(cfg)
	delegate.Mlist = mlist

	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}

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

	// TODO: API endpoint
	// Create helpful HTTP endpoint for debugging
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ret, err := json.Marshal(m.RouteMap.NodeRouteMap)
		if err != nil {
			logrus.Errorf("Unable to marshal graph: %v", err)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(ret)
		}
	})

	go http.ListenAndServe(":12345", nil)

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

package main

import (
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hashicorp/memberlist"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/traceroute"
)

// This goroutine is responsible for mapping app peers on the network
func mapper(routeMap *RouteMap, g *graph.NetworkGraph, mlist *memberlist.Memberlist) {
	srcPortStart := 33435
	srcPortEnd := 33450

	for {
		nodes := mlist.Members()

		for _, node := range nodes {
			// if this is ourselves, skip!
			if node == mlist.LocalNode() {
				continue
			}

			// Otherwise, lets do some stuff
			logrus.Infof("get routes to peer: %v %v", node.Addr, node)

			for srcPort := srcPortStart; srcPort < srcPortEnd; srcPort++ {

				options := traceroute.TracerouteOptions{}
				options.SetSrcPort(srcPort)        // TODO: config
				options.SetDstPort(int(node.Port)) // TODO: config

				ret, err := traceroute.Traceroute(
					node.Addr.String(), // TODO: take the IP direct
					&options,
				)
				if err != nil {
					logrus.Infof("Traceroute err: %v", err)
					continue
				}

				logrus.Infof("Traceroute %d -> %s: complete", srcPort, node.Addr.String())

				path := make([]string, 0, len(ret.Hops))

				for _, hop := range ret.Hops {
					path = append(path, hop.AddressString())
				}

				currRoute := routeMap.GetRouteOption(srcPort, node.Addr.String())

				// If we don't have a current route, or the paths differ-- lets update
				if currRoute == nil || !currRoute.SamePath(path) {
					// Add new one
					routeMap.UpdateRouteOption(srcPort, node.Addr.String(), g.IncrRoute(path))

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
}

func pinger(routeMap *RouteMap, mlist *memberlist.Memberlist) {
	for {
		time.Sleep(time.Second)
		nodes := mlist.Members()

		for _, node := range nodes {
			// if this is ourselves, skip!
			if node == mlist.LocalNode() {
				continue
			}
			c := make(chan string)
			go routeMap.IterRoutes(node.Addr.String(), c)
			for routeKey := range c {
				routeKeyParts := strings.SplitN(routeKey, ":", 2)
				srcPort, _ := strconv.Atoi(routeKeyParts[0])
				logrus.Infof("Ping srcPort=%d dst=%s", srcPort, routeKeyParts[1])

				p := ping{
					Name: node.Addr.String(),
					Path: routeMap.GetRoute(routeKey).Hops(),
				}
				// TODO: major cleanup to encapsulate all this message sending
				// Encode as a user message
				encodedBuf, err := encode(pingMsg, p)
				if err != nil {
					logrus.Infof("Unable to encode pingMsg: %v", err)
					continue
				}
				msg := encodedBuf.Bytes()
				buf := make([]byte, 1, len(msg)+1)
				buf[0] = byte(8) // TODO: add sendFrom API to memberlist
				buf = append(buf, msg...)

				LocalAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+routeKeyParts[0])
				if err != nil {
					logrus.Errorf("unable to resolve source addr %v", err)
				}
				RemoteEP := net.UDPAddr{IP: node.Addr, Port: int(node.Port)}
				conn, err := net.DialUDP("udp", LocalAddr, &RemoteEP)
				if err != nil {
					// handle error
					logrus.Errorf("unable to connect to peer: %v", err)
					continue
				}
				// TODO: configurable time
				//conn.SetDeadline(time.Now().Add(time.Second))

				// TODO: figure out how to get the message back...
				// seems that I might have to add some methods to get at `WriteToUDP`
				// in memberlist
				// fmt.FprintF Invokes the conn.Write() method and converts the string to a byte slice
				conn.Write(buf)

				// TODO: get a response from the ping
				//fmt.Println(ioutil.ReadAll(conn))
				conn.Close()
				time.Sleep(time.Second)
			}
		}
	}

}

func main() {
	// Some CLI args for better testing

	advertiseStr := flag.String("gossipAddr", "", "address to advertise gossip on")
	peerStr := flag.String("peer", "", "address to gossip with")

	flag.Parse()

	g := graph.Create()
	routeMap := NewRouteMap()

	// TODO: wire up delegate to delete all entries in routeMap for node that has left

	/*
		This is the main daemon. Which has the following responsibilities:
			- traceroute: Graph the network
			- ping: keep track of latency/jitter/loss across links
			- aggregate: to centralized location for better fault detection
			- coordinate: split the above work to scale better
	*/

	// #TODO: load from a config file
	cfg := memberlist.DefaultLANConfig()

	delegate := NewDNMSDelegate(g, routeMap)
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

	if err != nil {
		logrus.Fatalf("Unable to create memberlist: %v", err)
	}

	mlist.Join([]string{*peerStr})

	go mapper(routeMap, g, mlist)
	go pinger(routeMap, mlist)

	// TODO: API endpoint
	// Create helpful HTTP endpoint for debugging
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ret, err := json.Marshal(routeMap.NodeRouteMap)
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
			g.GetNodeCount(),
			g.GetLinkCount(),
			g.GetRouteCount(),
		)
	}

}

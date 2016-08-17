package aggregator

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/eventsource"
)

// Subscripe to dest, consuming events into PeerGraphMap.
// We'll return a bool channel which can be used to cancel the subscription
func Subscribe(p *AggGraphMap, peer string) chan bool {
	exitChan := make(chan bool)
	go func() {
		stream, err := eventsource.Subscribe("http://"+peer+":12345/v1/events/graph", "")
		if err != nil {
			logrus.Fatalf("Error subscribing: %v", err)
		}
		p.AddPeer(peer)
		defer p.RemovePeer(peer)

		peerMap := p.GetPeerMap(peer)
		for {
			select {
			case ev := <-stream.Events:
				// TODO: actually add/remove this correctly
				// TODO: care about more events
				switch ev.Event() {

				// Node events
				case "addNodeEvent":
					n := graph.NetworkNode{}
					json.Unmarshal([]byte(ev.Data()), &n)
					peerMap.AddNode(&n)
				// TODO: update event
				case "updateNodeEvent":
					n := graph.NetworkNode{}
					json.Unmarshal([]byte(ev.Data()), &n)
					node := p.Graph.GetNode(n.Name)
					// if the node is new-- its possible we get the updateEvent before
					// the route has been added
					if node != nil {
						// TODO: some sort of "merge" method
						node.DNSNames = n.DNSNames
					}
				case "removeNodeEvent":
					n := graph.NetworkNode{}
					json.Unmarshal([]byte(ev.Data()), &n)
					peerMap.RemoveNode(&n)

				// Link events
				case "addLinkEvent":
					l := graph.NetworkLink{}
					json.Unmarshal([]byte(ev.Data()), &l)
					peerMap.AddLink(&l)
				// TODO: update event
				case "updateLinkEvent":
					// TODO: implement
				case "removeLinkEvent":
					l := graph.NetworkLink{}
					json.Unmarshal([]byte(ev.Data()), &l)
					peerMap.RemoveLink(&l)

				// route events
				case "addRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					peerMap.AddRoute(&r)
				// TODO: update event
				case "updateRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					route := p.Graph.GetRoute(r.Hops())

					// TODO: some sort of "merge" method
					route.State = r.State
				case "removeRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					peerMap.RemoveRoute(&r)

				}
			case <-exitChan:
				return
			}
		}
	}()
	return exitChan
}

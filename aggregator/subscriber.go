package aggregator

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/eventsource"
)

// TODO: reconnect if the subsciber disconnects and we weren't asked to cancel
// Subscripe to dest, consuming events into PeerGraphMap.
// We'll return a bool channel which can be used to cancel the subscription
func Subscribe(p *PeerGraphMap) chan bool {
	exitChan := make(chan bool)
	go func() {
		stream, err := eventsource.Subscribe("http://"+p.Name+":12345/v1/events/graph", "")
		if err != nil {
			logrus.Fatalf("Error subscribing: %v", err)
		}
		// defer a removal in case the peer disconnects (or blips)
		defer p.cleanup()

		for {
			select {
			case ev := <-stream.Events:
				switch ev.Event() {

				// Node events
				case "addNodeEvent":
					n := graph.NetworkNode{}
					json.Unmarshal([]byte(ev.Data()), &n)
					p.AddNode(&n)
				case "updateNodeEvent":
					n := graph.NetworkNode{}
					json.Unmarshal([]byte(ev.Data()), &n)
					node := p.Graph.GetNode(n.Name)
					// TODO: some sort of "merge" method
					if node != nil {
						// TODO: some sort of "merge" method
						node.DNSNames = n.DNSNames
					}
				case "removeNodeEvent":
					n := graph.NetworkNode{}
					json.Unmarshal([]byte(ev.Data()), &n)
					p.RemoveNode(&n)

				// Link events
				case "addLinkEvent":
					l := graph.NetworkLink{}
					json.Unmarshal([]byte(ev.Data()), &l)
					p.AddLink(&l)
				// TODO: update event
				case "updateLinkEvent":
					// TODO: implement
					// TODO: some sort of "merge" method
				case "removeLinkEvent":
					l := graph.NetworkLink{}
					json.Unmarshal([]byte(ev.Data()), &l)
					p.RemoveLink(&l)

				// route events
				case "addRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					p.AddRoute(&r)
				case "updateRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					route := p.Graph.GetRoute(r.Hops())

					// TODO: some sort of "merge" method
					route.State = r.State
				case "removeRouteEvent":
					r := graph.NetworkRoute{}
					json.Unmarshal([]byte(ev.Data()), &r)
					p.RemoveRoute(&r)

				}
			case <-exitChan:
				return
			}
		}
	}()
	return exitChan
}

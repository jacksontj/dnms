package aggregator

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/eventsource"
)

// Subscripe to dest, consuming events into PeerGraphMap.
// We'll return a bool channel which can be used to cancel the subscription
func Subscribe(p *PeerGraphMap) chan bool {
	exitChan := make(chan bool)
	go func() {
		stream, err := eventsource.Subscribe("http://"+p.Name+":12345/v1/events/graph", "")
		if err != nil {
			logrus.Fatalf("Error subscribing: %v", err)
		}
		logrus.Infof("connecting to peer: %v", p.Name)
		// defer a removal in case the peer disconnects (or blips)
		defer p.cleanup()

		for {
			select {
			// handle errors-- all of these mean a disconnect/reconnect
			case err, ok := <-stream.Errors:
				logrus.Debugf("stream error, reconnecting: %v %v", err, ok)
				// we need to remove everything we know about this peer-- since
				// the new connection will re-seed on the new connection
				p.cleanup()
			case ev := <-stream.Events:
				//logrus.Infof("Got Event: %v", ev.Event())
				switch ev.Event() {

				// Node events
				case "addNodeEvent":
					n := graph.NetworkNode{}
					err := json.Unmarshal([]byte(ev.Data()), &n)
					if err != nil {
						logrus.Warningf("unable to unmarshal node: %v", err)
					}
					p.AddNode(&n)
				case "updateNodeEvent":
					n := graph.NetworkNode{}
					err := json.Unmarshal([]byte(ev.Data()), &n)
					if err != nil {
						logrus.Warningf("unable to unmarshal node: %v", err)
					}
					node := p.Graph.GetNode(n.Name)
					// TODO: some sort of "merge" method
					if node != nil {
						node.DNSNames = n.DNSNames
					}
				case "removeNodeEvent":
					n := graph.NetworkNode{}
					err := json.Unmarshal([]byte(ev.Data()), &n)
					if err != nil {
						logrus.Warningf("unable to unmarshal node: %v", err)
					}
					p.RemoveNode(&n)

				// Link events
				case "addLinkEvent":
					l := graph.NetworkLink{}
					err := json.Unmarshal([]byte(ev.Data()), &l)
					if err != nil {
						logrus.Warningf("unable to unmarshal link: %v", err)
					}
					p.AddLink(&l)
				// TODO: update event
				case "updateLinkEvent":
					// TODO: implement
					// TODO: some sort of "merge" method
				case "removeLinkEvent":
					l := graph.NetworkLink{}
					err := json.Unmarshal([]byte(ev.Data()), &l)
					if err != nil {
						logrus.Warningf("unable to unmarshal link: %v", err)
					}
					p.RemoveLink(&l)

				// route events
				case "addRouteEvent":
					r := graph.NetworkRoute{}
					err := json.Unmarshal([]byte(ev.Data()), &r)
					if err != nil {
						logrus.Warningf("unable to unmarshal route: %v", err)
					}
					p.AddRoute(&r)
				case "updateRouteEvent":
					r := graph.NetworkRoute{}
					err := json.Unmarshal([]byte(ev.Data()), &r)
					if err != nil {
						logrus.Warningf("unable to unmarshal route: %v", err)
					}
					route := p.Graph.GetRoute(r.Hops())

					// TODO: some sort of "merge" method
					route.State = r.State
				case "removeRouteEvent":
					r := graph.NetworkRoute{}
					err := json.Unmarshal([]byte(ev.Data()), &r)
					if err != nil {
						logrus.Warningf("unable to unmarshal route: %v", err)
					}
					p.RemoveRoute(&r)

				}
			case <-exitChan:
				return
			}
		}
	}()
	return exitChan
}

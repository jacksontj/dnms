package aggregator

import (
	"encoding/json"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/eventsource"
)

// Subscripe to dest, consuming events into PeerGraphMap.
// We'll return a bool channel which can be used to cancel the subscription
func Subscribe(p *PeerGraphMap, dest string) chan bool {
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

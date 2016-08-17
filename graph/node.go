// TODO: better name? network topology?
package graph

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/Sirupsen/logrus"
)

// TODO: differentiate between peers and L3devices in the middle
// TODO: handle addr '*' -- to compensate maybe we can just use a compound of the
// node on either side? so something like A -> * -> * -> B would become A*_B (for the second *)
// TODO: maintain pointers to NetworkLink for traversal
type NetworkNode struct {
	Name string `json:"name"`

	// asynchronously loaded
	DNSNames []string `json:"dns_names"`
	nLock    *sync.RWMutex

	refCount int

	updateChan chan *Event
}

func NewNetworkNode(name string, updateChan chan *Event) *NetworkNode {
	r := &NetworkNode{
		Name:       name,
		nLock:      &sync.RWMutex{},
		updateChan: updateChan,
	}

	// background load DNS names
	go func(n *NetworkNode) {
		names, err := net.LookupAddr(n.Name)
		if err != nil {
			logrus.Debugf("Unable to do reverse DNS lookup for %s", r.Name)
		} else {
			n.nLock.Lock()
			n.DNSNames = names
			n.nLock.Unlock()
		}
		n.updateChan <- &Event{
			E:    updateEvent,
			Item: n,
		}
	}(r)

	return r
}

// Fancy marshal method
func (n *NetworkNode) MarshalJSON() ([]byte, error) {
	n.nLock.RLock()
	defer n.nLock.RUnlock()

	type Alias NetworkNode
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(n),
	})
}

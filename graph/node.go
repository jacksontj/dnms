// TODO: better name? network topology?
package graph

import (
	"net"

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

	refCount int
}

func NewNetworkNode(name string) *NetworkNode {
	r := &NetworkNode{
		Name: name,
	}

	// background load DNS names
	go func(n *NetworkNode) {
		names, err := net.LookupAddr(n.Name)
		if err != nil {
			logrus.Debugf("Unable to do reverse DNS lookup for %s", r.Name)
		} else {
			n.DNSNames = names
		}
	}(r)

	return r
}

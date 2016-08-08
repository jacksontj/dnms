// TODO: better name? network topology?
package graph

import "net"

// TODO: handle addr '*' -- to compensate maybe we can just use a compound of the
// node on either side? so something like A -> * -> * -> B would become A*_B (for the second *)
// TODO: maintain pointers to NetworkLink for traversal
type NetworkNode struct {
	Name string
	// TODO: use? attempt to parse string
	Addr net.IP

	RefCount int
}

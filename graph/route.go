package graph

import "net"

type RouteKey struct {
	Src string
	Dst string
}

// TODO: RoundTripRoute? Right now the Route is a single direction since we only
// have one side of the traceroute. If the peers gossip about the reverse routes
// then we could potentially have both directions
// TODO: TTL for routes? If we just start up we don't want to have to re-ping the
// world before we are useful
type NetworkRoute struct {
	Src net.UDPAddr
	Dst net.UDPAddr

	Links []*NetworkLink
	// TODO don't store
	Hops []string

	RefCount int
}

func (r *NetworkRoute) SameHops(hops []string) bool {
	// check len
	if len(hops) != len(r.Hops) {
		return false
	}

	for i, hop := range hops {
		if hop != r.Hops[i] {
			return false
		}
	}
	return true
}

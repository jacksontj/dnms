package graph

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
	Path []*NetworkNode

	RefCount int
}

func (r *NetworkRoute) SamePath(path []string) bool {
	// check len
	if len(path) != len(r.Path) {
		return false
	}

	for i, hop := range path {
		if hop != r.Path[i].Name {
			return false
		}
	}
	return true
}

func (r *NetworkRoute) Hops() []string {
	hops := make([]string, 0, len(r.Path))
	for _, node := range r.Path {
		hops = append(hops, node.Name)
	}
	return hops
}

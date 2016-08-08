// TODO: better name? network topology?
package graph

import (
	"net"

	"github.com/Sirupsen/logrus"
)

// TODO: maintain some maps for easier lookup
type NetworkGraph struct {
	// nodeName -> Node
	nodesMap map[string]*NetworkNode

	// TODO: change linkKey to string
	// nodeName,nodeName -> NetworkLink
	linksMap map[NetworkLinkKey]*NetworkLink

	// TODO: change routeKey to string
	routesMap map[RouteKey]*NetworkRoute
}

func Create() *NetworkGraph {
	return &NetworkGraph{
		nodesMap:  make(map[string]*NetworkNode),
		linksMap:  make(map[NetworkLinkKey]*NetworkLink),
		routesMap: make(map[RouteKey]*NetworkRoute),
	}
}

func (g *NetworkGraph) IncrNode(name string) *NetworkNode {
	n, ok := g.nodesMap[name]
	// if this one doesn't exist, lets add it
	if !ok {
		n = &NetworkNode{
			Name: name,
		}
		g.nodesMap[name] = n
	}
	n.RefCount++
	return n
}

func (g *NetworkGraph) GetNode(name string) *NetworkNode {
	n, _ := g.nodesMap[name]
	return n
}

func (g *NetworkGraph) GetNodeCount() int {
	return len(g.nodesMap)
}

func (g *NetworkGraph) DecrNode(name string) {
	n, ok := g.nodesMap[name]

	if !ok {
		logrus.Warningf("Attempted to remove node with ip %v which wasn't in the graph", name)
		return
	}

	n.RefCount--
	if n.RefCount == 0 {
		delete(g.nodesMap, name)
	}
}

func (g *NetworkGraph) IncrLink(src, dst string) *NetworkLink {
	key := NetworkLinkKey{src, dst}
	l, ok := g.linksMap[key]
	if !ok {
		l = &NetworkLink{
			Src: g.IncrNode(src),
			Dst: g.IncrNode(dst),
		}
		g.linksMap[key] = l
	}
	l.RefCount++
	return l
}

func (g *NetworkGraph) GetLink(src, dst string) *NetworkLink {
	key := NetworkLinkKey{src, dst}
	l, _ := g.linksMap[key]
	return l
}

func (g *NetworkGraph) GetLinkCount() int {
	return len(g.linksMap)
}

func (g *NetworkGraph) DecrLink(src, dst string) {
	key := NetworkLinkKey{src, dst}
	l, ok := g.linksMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove link %v which wasn't in the graph", key)
		return
	}
	// Decrement our children
	g.DecrNode(src)
	g.DecrNode(dst)
	// decrement ourselves
	l.RefCount--
	if l.RefCount == 0 {
		delete(g.linksMap, key)
	}
}

// TODO: don't delete the old one until the new one is in-- to avoid flapping
func (g *NetworkGraph) IncrRoute(src, dst net.UDPAddr, hops []string) *NetworkRoute {
	key := RouteKey{src.String(), dst.String()}
	r, ok := g.routesMap[key]

	// If a matching route exists, but the path is different, we are going to replace it
	if ok {
		if !r.SameHops(hops) {
			ok = false
			g.DecrRoute(src, dst)
		} else {
			return r // same exact thing
		}
	}

	if !ok {
		// convert hops to links
		links := make([]*NetworkLink, 0)
		for i, hopIP := range hops {
			nextI := i + 1
			if nextI >= len(hops) {
				break
			}
			links = append(links, g.IncrLink(hopIP, hops[nextI]))
		}

		r = &NetworkRoute{
			Src:   src,
			Dst:   dst,
			Links: links,
			Hops:  hops,
		}
		g.routesMap[key] = r
	}
	r.RefCount++
	return r
}

func (g *NetworkGraph) GetRoute(src, dst net.UDPAddr) *NetworkRoute {
	key := RouteKey{src.String(), dst.String()}
	r, _ := g.routesMap[key]
	return r
}

func (g *NetworkGraph) GetRouteCount() int {
	return len(g.routesMap)
}

func (g *NetworkGraph) DecrRoute(src, dst net.UDPAddr) {
	key := RouteKey{src.String(), dst.String()}
	r, ok := g.routesMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove route %v which wasn't in the graph", key)
		return
	}

	// decrement all the links/nodes as well
	for _, link := range r.Links {
		g.DecrLink(link.Src.Name, link.Dst.Name)
	}

	r.RefCount--
	// TODO: fix this-- routes are a bit of a mess
	if r.RefCount == 0 || true {
		delete(g.routesMap, key)
	}
}

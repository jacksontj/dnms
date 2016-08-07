// TODO: better name? network topology?
package graph

import (
	"net"

	"github.com/Sirupsen/logrus"
)

// TODO: maintain some maps for easier lookup
type NetworkGraph struct {
	nodesMap map[string]*NetworkNode

	linksMap map[NetworkLinkKey]*NetworkLink

	routesMap map[RouteKey]*NetworkRoute
}

func Create() *NetworkGraph {
	return &NetworkGraph{
		nodesMap:  make(map[string]*NetworkNode),
		linksMap:  make(map[NetworkLinkKey]*NetworkLink),
		routesMap: make(map[RouteKey]*NetworkRoute),
	}
}

func (g *NetworkGraph) AddNode(ip net.IP) *NetworkNode {
	key := ip.String()
	n, ok := g.nodesMap[key]
	// if this one doesn't exist, lets add it
	if !ok {
		n = &NetworkNode{
			Addr: ip,
		}
		g.nodesMap[key] = n
	}
	n.RefCount++
	return n
}

func (g *NetworkGraph) GetNode(ip net.IP) *NetworkNode {
	key := ip.String()
	n, _ := g.nodesMap[key]
	return n
}

func (g *NetworkGraph) RemoveNode(ip net.IP) {
	key := ip.String()
	n, ok := g.nodesMap[key]

	if !ok {
		logrus.Warning("Attempted to remove node with ip %v which wasn't in the graph", key)
		return
	}

	n.RefCount--
	if n.RefCount == 0 {
		delete(g.nodesMap, key)
	}
}

func (g *NetworkGraph) AddLink(src, dst net.IP) *NetworkLink {
	key := NetworkLinkKey{src.String(), dst.String()}
	l, ok := g.linksMap[key]
	if !ok {
		l = &NetworkLink{
			Src: g.AddNode(src),
			Dst: g.AddNode(dst),
		}
		g.linksMap[key] = l
	}
	l.RefCount++
	return l
}

func (g *NetworkGraph) GetLink(src, dst net.IP) *NetworkLink {
	key := NetworkLinkKey{src.String(), dst.String()}
	l, _ := g.linksMap[key]
	return l
}

func (g *NetworkGraph) RemoveLink(src, dst net.IP) {
	key := NetworkLinkKey{src.String(), dst.String()}
	l, ok := g.linksMap[key]
	if !ok {
		logrus.Warning("Attempted to remove link %v which wasn't in the graph", key)
		return
	}
	l.RefCount--
}

// TODO: don't delete the old one until the new one is in-- to avoid flapping
func (g *NetworkGraph) AddRoute(src, dst net.UDPAddr, hops []net.IP) *NetworkRoute {
	key := RouteKey{src.String(), dst.String()}
	r, ok := g.routesMap[key]

	// If a matching route exists, but the path is different, we are going to replace it
	if ok && !r.SameHops(hops) {
		ok = false
		g.RemoveRoute(src, dst)
	}

	if !ok {
		// convert hops to links
		links := make([]*NetworkLink, 0)
		for i, hopIP := range hops {
			nextI := i + 1
			if nextI >= len(hops) {
				break
			}
			links = append(links, g.AddLink(hopIP, hops[nextI]))
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

func (g *NetworkGraph) RemoveRoute(src, dst net.UDPAddr) {
	key := RouteKey{src.String(), dst.String()}
	r, ok := g.routesMap[key]
	if !ok {
		logrus.Warning("Attempted to remove route %v which wasn't in the graph", key)
		return
	}
	r.RefCount--
}

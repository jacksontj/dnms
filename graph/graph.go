// TODO: better name? network topology?
package graph

import (
	"crypto/md5"
	"encoding/hex"
	"io"

	"github.com/Sirupsen/logrus"
)

// TODO: maintain some maps for easier lookup
type NetworkGraph struct {
	// nodeName -> Node
	nodesMap map[string]NetworkNode

	// TODO: change linkKey to string
	// nodeName,nodeName -> NetworkLink
	linksMap map[NetworkLinkKey]NetworkLink

	routesMap map[string]NetworkRoute
}

func Create() *NetworkGraph {
	return &NetworkGraph{
		nodesMap:  make(map[string]NetworkNode),
		linksMap:  make(map[NetworkLinkKey]NetworkLink),
		routesMap: make(map[string]NetworkRoute),
	}
}

func (g *NetworkGraph) IncrNode(name string) *NetworkNode {
	n, ok := g.nodesMap[name]
	// if this one doesn't exist, lets add it
	if !ok {
		n = NetworkNode{
			Name: name,
		}
		g.nodesMap[name] = n
	}
	n.RefCount++
	return &n
}

func (g *NetworkGraph) GetNode(name string) *NetworkNode {
	n, _ := g.nodesMap[name]
	return &n
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
		l = NetworkLink{
			Src: g.IncrNode(src),
			Dst: g.IncrNode(dst),
		}
		g.linksMap[key] = l
	}
	l.RefCount++
	return &l
}

func (g *NetworkGraph) GetLink(src, dst string) *NetworkLink {
	key := NetworkLinkKey{src, dst}
	l, _ := g.linksMap[key]
	return &l
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

func (g *NetworkGraph) pathKey(hops []string) string {
	h := md5.New()
	for _, hop := range hops {
		io.WriteString(h, hop)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// TODO: don't delete the old one until the new one is in-- to avoid flapping
func (g *NetworkGraph) IncrRoute(hops []string) *NetworkRoute {
	key := g.pathKey(hops)

	// check if we have a route for this already
	route, ok := g.routesMap[key]
	// if we don't have it, lets make it
	if !ok {
		logrus.Infof("New Route: key=%s %v", key, hops)
		path := make([]*NetworkNode, 0, len(hops))
		for i, hop := range hops {
			path = append(path, g.IncrNode(hop))
			// If there was something prior-- lets add the link as well
			if i-1 > 0 {
				g.IncrLink(hops[i-1], hop)
			}
		}
		route = NetworkRoute{
			Path: path,
		}
		g.routesMap[key] = route
	}

	// increment route's refcount
	route.RefCount++

	return &route
}

func (g *NetworkGraph) GetRoute(hops []string) *NetworkRoute {
	r, _ := g.routesMap[g.pathKey(hops)]
	return &r
}

func (g *NetworkGraph) GetRouteCount() int {
	return len(g.routesMap)
}

func (g *NetworkGraph) DecrRoute(hops []string) {
	key := g.pathKey(hops)
	r, ok := g.routesMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove route %v which wasn't in the graph", key)
		return
	}

	// decrement all the links/nodes as well
	for i, node := range r.Path {
		g.DecrNode(node.Name)
		if i-1 > 0 {
			g.DecrLink(r.Path[i-1].Name, node.Name)
		}
	}

	r.RefCount--
	if r.RefCount == 0 {
		delete(g.routesMap, key)
	}
}

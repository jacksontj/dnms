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
	NodesMap map[string]*NetworkNode

	// TODO: change linkKey to string
	// nodeName,nodeName -> NetworkLink
	LinksMap map[NetworkLinkKey]*NetworkLink

	RoutesMap map[string]*NetworkRoute
}

func Create() *NetworkGraph {
	return &NetworkGraph{
		NodesMap:  make(map[string]*NetworkNode),
		LinksMap:  make(map[NetworkLinkKey]*NetworkLink),
		RoutesMap: make(map[string]*NetworkRoute),
	}
}

func (g *NetworkGraph) IncrNode(name string) *NetworkNode {
	n, ok := g.NodesMap[name]
	// if this one doesn't exist, lets add it
	if !ok {
		n = &NetworkNode{
			Name: name,
		}
		g.NodesMap[name] = n
	}
	n.RefCount++
	return n
}

func (g *NetworkGraph) GetNode(name string) *NetworkNode {
	n, _ := g.NodesMap[name]
	return n
}

func (g *NetworkGraph) GetNodeCount() int {
	return len(g.NodesMap)
}

func (g *NetworkGraph) DecrNode(name string) {
	n, ok := g.NodesMap[name]

	if !ok {
		logrus.Warningf("Attempted to remove node with ip %v which wasn't in the graph", name)
		return
	}

	n.RefCount--
	if n.RefCount == 0 {
		delete(g.NodesMap, name)
	}
}

func (g *NetworkGraph) IncrLink(src, dst string) *NetworkLink {
	key := NetworkLinkKey{src, dst}
	l, ok := g.LinksMap[key]
	if !ok {
		l = &NetworkLink{
			Src: g.IncrNode(src),
			Dst: g.IncrNode(dst),
		}
		g.LinksMap[key] = l
	}
	l.RefCount++
	return l
}

func (g *NetworkGraph) GetLink(src, dst string) *NetworkLink {
	key := NetworkLinkKey{src, dst}
	l, _ := g.LinksMap[key]
	return l
}

func (g *NetworkGraph) GetLinkCount() int {
	return len(g.LinksMap)
}

func (g *NetworkGraph) DecrLink(src, dst string) {
	key := NetworkLinkKey{src, dst}
	l, ok := g.LinksMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove link %v which wasn't in the graph", key)
		return
	}
	// decrement ourselves
	l.RefCount--
	if l.RefCount == 0 {
		// Decrement our children
		g.DecrNode(src)
		g.DecrNode(dst)
		delete(g.LinksMap, key)
	}
}

func (g *NetworkGraph) pathKey(hops []string) string {
	h := md5.New()
	for _, hop := range hops {
		io.WriteString(h, hop)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (g *NetworkGraph) IncrRoute(hops []string) *NetworkRoute {
	key := g.pathKey(hops)

	// check if we have a route for this already
	route, ok := g.RoutesMap[key]
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
		route = &NetworkRoute{
			Path: path,
		}
		g.RoutesMap[key] = route
	}

	// increment route's refcount
	route.RefCount++

	return route
}

func (g *NetworkGraph) GetRoute(hops []string) *NetworkRoute {
	r, _ := g.RoutesMap[g.pathKey(hops)]
	return r
}

func (g *NetworkGraph) GetRouteCount() int {
	return len(g.RoutesMap)
}

func (g *NetworkGraph) DecrRoute(hops []string) {
	key := g.pathKey(hops)
	r, ok := g.RoutesMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove route %v which wasn't in the graph", key)
		return
	}

	r.RefCount--
	if r.RefCount == 0 {
		// decrement all the links/nodes as well
		for i, node := range r.Path {
			g.DecrNode(node.Name)
			if i-1 > 0 {
				g.DecrLink(r.Path[i-1].Name, node.Name)
			}
		}

		delete(g.RoutesMap, key)
	}
}

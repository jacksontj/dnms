// TODO: better name? network topology?
package graph

import (
	"container/ring"
	"crypto/md5"
	"encoding/hex"
	"io"

	"github.com/Sirupsen/logrus"
)

// TODO: maintain some maps for easier lookup
type NetworkGraph struct {
	// nodeName -> Node
	NodesMap map[string]*NetworkNode `json:"nodes"`

	// nodeName,nodeName -> NetworkLink
	LinksMap map[string]*NetworkLink `json:"edges"`

	RoutesMap map[string]*NetworkRoute `json:"routes"`
}

func Create() *NetworkGraph {
	return &NetworkGraph{
		NodesMap:  make(map[string]*NetworkNode),
		LinksMap:  make(map[string]*NetworkLink),
		RoutesMap: make(map[string]*NetworkRoute),
	}
}

func (g *NetworkGraph) IncrNode(name string) (*NetworkNode, bool) {
	n, ok := g.NodesMap[name]
	// if this one doesn't exist, lets add it
	if !ok {
		n = NewNetworkNode(name)
		g.NodesMap[name] = n
	}
	n.refCount++
	return n, !ok
}

func (g *NetworkGraph) GetNode(name string) *NetworkNode {
	n, _ := g.NodesMap[name]
	return n
}

func (g *NetworkGraph) GetNodeCount() int {
	return len(g.NodesMap)
}

func (g *NetworkGraph) DecrNode(name string) bool {
	n, ok := g.NodesMap[name]

	if !ok {
		logrus.Warningf("Attempted to remove node with ip %v which wasn't in the graph", name)
		return false
	}

	n.refCount--
	if n.refCount == 0 {
		delete(g.NodesMap, name)
		return true
	}
	return false
}

func (g *NetworkGraph) IncrLink(src, dst string) (*NetworkLink, bool) {
	key := src + "," + dst
	l, ok := g.LinksMap[key]
	if !ok {
		srcNode, _ := g.IncrNode(src)
		dstNode, _ := g.IncrNode(dst)
		l = &NetworkLink{
			Src: srcNode,
			Dst: dstNode,
		}
		g.LinksMap[key] = l
	}
	l.refCount++
	return l, !ok
}

func (g *NetworkGraph) GetLink(src, dst string) *NetworkLink {
	key := src + "," + dst
	l, _ := g.LinksMap[key]
	return l
}

func (g *NetworkGraph) GetLinkCount() int {
	return len(g.LinksMap)
}

func (g *NetworkGraph) DecrLink(src, dst string) bool {
	key := src + "," + dst
	l, ok := g.LinksMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove link %v which wasn't in the graph", key)
		return false
	}
	// decrement ourselves
	l.refCount--
	if l.refCount == 0 {
		// Decrement our children
		g.DecrNode(src)
		g.DecrNode(dst)
		delete(g.LinksMap, key)
		return true
	}
	return false
}

func (g *NetworkGraph) pathKey(hops []string) string {
	h := md5.New()
	for _, hop := range hops {
		io.WriteString(h, hop)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (g *NetworkGraph) IncrRoute(hops []string) (*NetworkRoute, bool) {
	key := g.pathKey(hops)

	// check if we have a route for this already
	route, ok := g.RoutesMap[key]
	// if we don't have it, lets make it
	if !ok {
		logrus.Infof("New Route: key=%s %v", key, hops)
		path := make([]*NetworkNode, 0, len(hops))
		for i, hop := range hops {
			hopNode, _ := g.IncrNode(hop)
			path = append(path, hopNode)
			// If there was something prior-- lets add the link as well
			if i-1 >= 0 {
				g.IncrLink(hops[i-1], hop)
			}
		}
		route = &NetworkRoute{
			Path:       path,
			State:      Up,
			metricRing: ring.New(10), // TODO: config
		}
		g.RoutesMap[key] = route
	}

	// increment route's refcount
	route.refCount++

	return route, !ok
}

func (g *NetworkGraph) GetRoute(hops []string) *NetworkRoute {
	r, _ := g.RoutesMap[g.pathKey(hops)]
	return r
}

func (g *NetworkGraph) GetRouteCount() int {
	return len(g.RoutesMap)
}

func (g *NetworkGraph) DecrRoute(hops []string) bool {
	key := g.pathKey(hops)
	r, ok := g.RoutesMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove route %v which wasn't in the graph", key)
		return false
	}

	r.refCount--
	if r.refCount == 0 {
		// decrement all the links/nodes as well
		for i, node := range r.Path {
			g.DecrNode(node.Name)
			if i-1 >= 0 {
				g.DecrLink(r.Path[i-1].Name, node.Name)
			}
		}

		delete(g.RoutesMap, key)
		return true
	}
	return false
}

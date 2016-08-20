package aggregator

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
)

// This wraps graph.NetworkGraph to keep track of a given peer's refcounts on the
// graph, so that when a peer goes away we can cleanup after it
type PeerGraphMap struct {
	Name      string
	nodesMap  map[*graph.NetworkNode]int
	nodesLock *sync.RWMutex

	linksMap  map[*graph.NetworkLink]int
	linksLock *sync.RWMutex

	routesMap  map[*graph.NetworkRoute]int
	routesLock *sync.RWMutex

	// pointer to the graph for us to use
	Graph *graph.NetworkGraph

	subscriberExit chan bool
}

func NewPeerGraphMap(name string, g *graph.NetworkGraph) *PeerGraphMap {
	p := &PeerGraphMap{
		Name:       name,
		nodesMap:   make(map[*graph.NetworkNode]int),
		nodesLock:  &sync.RWMutex{},
		linksMap:   make(map[*graph.NetworkLink]int),
		linksLock:  &sync.RWMutex{},
		routesMap:  make(map[*graph.NetworkRoute]int),
		routesLock: &sync.RWMutex{},

		Graph: g,
	}

	// subscribe
	c := Subscribe(p)

	p.subscriberExit = c

	return p
}

func (p *PeerGraphMap) AddNode(n *graph.NetworkNode) {
	p.nodesLock.Lock()
	defer p.nodesLock.Unlock()
	p.addNode(n)
}

func (p *PeerGraphMap) addNode(n *graph.NetworkNode) {
	node, added := p.Graph.IncrNode(n.Name, n)
	if added {
		p.nodesMap[node] = 0
	}
	p.nodesMap[node]++
}

func (p *PeerGraphMap) RemoveNode(n *graph.NetworkNode) {
	p.nodesLock.Lock()
	defer p.nodesLock.Unlock()
	p.removeNode(n)
}

func (p *PeerGraphMap) removeNode(n *graph.NetworkNode) {
	node, removed := p.Graph.DecrNode(n.Name)
	p.nodesMap[node]--

	if removed && p.nodesMap[node] != 0 {
		logrus.Warningf("link removed from graph, even when we have %d refcounts to it!", p.nodesMap[node])
	}

	if removed {
		delete(p.nodesMap, node)
	}
}

func (p *PeerGraphMap) AddLink(l *graph.NetworkLink) {
	p.linksLock.Lock()
	defer p.linksLock.Unlock()
	p.addLink(l)
}

func (p *PeerGraphMap) addLink(l *graph.NetworkLink) {
	link, added := p.Graph.IncrLink(l.SrcName, l.DstName, l)
	if added {
		p.linksMap[link] = 0
	}
	p.linksMap[link]++
}

func (p *PeerGraphMap) RemoveLink(l *graph.NetworkLink) {
	p.linksLock.Lock()
	defer p.linksLock.Unlock()
	p.removeLink(l)
}

func (p *PeerGraphMap) removeLink(l *graph.NetworkLink) {
	link, removed := p.Graph.DecrLink(l.SrcName, l.DstName)
	p.linksMap[link]--

	if removed && p.linksMap[link] != 0 {
		logrus.Warningf("link removed from graph, even when we have %d refcounts to it!", p.linksMap[link])
	}

	if removed {
		delete(p.linksMap, link)
	}
}

func (p *PeerGraphMap) AddRoute(r *graph.NetworkRoute) {
	p.routesLock.Lock()
	defer p.routesLock.Unlock()
	p.addRoute(r)
}

func (p *PeerGraphMap) addRoute(r *graph.NetworkRoute) {
	route, added := p.Graph.IncrRoute(r.Hops(), r)
	if added {
		p.routesMap[route] = 0
	}
	p.routesMap[route]++
}

func (p *PeerGraphMap) RemoveRoute(r *graph.NetworkRoute) {
	p.routesLock.Lock()
	defer p.routesLock.Unlock()
	p.removeRoute(r)
}

func (p *PeerGraphMap) removeRoute(r *graph.NetworkRoute) {
	route, removed := p.Graph.DecrRoute(r.Hops())
	p.routesMap[route]--

	if removed && p.routesMap[route] != 0 {
		logrus.Warningf("route removed from graph, even when we have %d refcounts to it!", p.routesMap[route])
	}

	if removed {
		delete(p.routesMap, route)
	}

}

// remove all routes associated with this peer
func (p *PeerGraphMap) cleanup() {
	// remove all routes
	p.routesLock.RLock()
	for route, count := range p.routesMap {
		for x := 0; x < count; x++ {
			p.removeRoute(route)
		}
	}
	p.routesLock.RUnlock()

	// remove all links
	p.linksLock.RLock()
	for link, count := range p.linksMap {
		for x := 0; x < count; x++ {
			p.removeLink(link)
		}
	}
	p.linksLock.RUnlock()

	// remove all nodes
	p.nodesLock.RLock()
	for node, count := range p.nodesMap {
		for x := 0; x < count; x++ {
			p.removeNode(node)
		}
	}
	p.nodesLock.RUnlock()
	p.subscriberExit <- false
}

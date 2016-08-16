package aggregator

import (
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
)

// TODO: use routemap
type PeerGraphMap struct {
	// map of peer -> routes -> ourRefcount
	peerRouteMap map[string]map[*graph.NetworkRoute]int
	mapLock      *sync.RWMutex

	Graph *graph.NetworkGraph
}

func NewPeerGraphMap() *PeerGraphMap {
	return &PeerGraphMap{
		peerRouteMap: make(map[string]map[*graph.NetworkRoute]int),
		mapLock:      &sync.RWMutex{},
		Graph:        graph.Create(),
	}
}

func (p *PeerGraphMap) addRoute(peer string, r *graph.NetworkRoute) {
	p.mapLock.Lock()
	defer p.mapLock.Unlock()
	pmap, ok := p.peerRouteMap[peer]
	if !ok {
		pmap = make(map[*graph.NetworkRoute]int)
		p.peerRouteMap[peer] = pmap
	}
	route, added := p.Graph.IncrRoute(r.Hops())
	if added {
		p.peerRouteMap[peer][route] = 0
	}
	p.peerRouteMap[peer][route]++
}

func (p *PeerGraphMap) removeRoute(peer string, r *graph.NetworkRoute) {
	p.mapLock.Lock()
	defer p.mapLock.Unlock()
	route, removed := p.Graph.DecrRoute(r.Hops())
	p.peerRouteMap[peer][route]--

	if removed {
		delete(p.peerRouteMap[peer], route)
	}
}

// TODO: use
func (p *PeerGraphMap) addPeer(peer string) {
	p.mapLock.Lock()
	defer p.mapLock.Unlock()
	pmap, ok := p.peerRouteMap[peer]
	if !ok {
		pmap = make(map[*graph.NetworkRoute]int)
		p.peerRouteMap[peer] = pmap
	}
}

// remove all routes associated with a peer
func (p *PeerGraphMap) removePeer(peer string) {
	p.mapLock.Lock()
	defer p.mapLock.Unlock()
	pmap, ok := p.peerRouteMap[peer]
	if !ok {
		logrus.Warningf("Attempting to remove a peer which isn't in the map: %v", peer)
	}

	for route, count := range pmap {
		for x := 0; x < count; x++ {
			p.Graph.DecrRoute(route.Hops())
		}
	}
}

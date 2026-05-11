package aggregator

import (
	"context"
	"sync"

	"github.com/jacksontj/dnms/graph"
	"github.com/sirupsen/logrus"
)

// This is an aggregated graph of the cluster
// peerMap is a mapping of peerName -> peergraphmap (which keeps track of what
// items in the graph where due to the given peer)
type AggGraphMap struct {
	ctx context.Context

	// map of peer -> routes -> ourRefcount
	peerMap map[string]*PeerGraphMap
	mapLock *sync.RWMutex

	Graph *graph.NetworkGraph

	// SubscribeToken, if set, is sent as a bearer Authorization header on
	// every peer SSE subscription. Empty disables the header.
	SubscribeToken string
}

func NewAggGraphMap(ctx context.Context) *AggGraphMap {
	return &AggGraphMap{
		ctx:     ctx,
		peerMap: make(map[string]*PeerGraphMap),
		mapLock: &sync.RWMutex{},
		Graph:   graph.Create(ctx),
	}
}

func (p *AggGraphMap) AddPeer(peer string) {
	p.mapLock.Lock()
	defer p.mapLock.Unlock()
	_, ok := p.peerMap[peer]
	if !ok {
		p.peerMap[peer] = NewPeerGraphMap(p.ctx, peer, p.Graph, p.SubscribeToken)
	}
}

// remove all routes associated with a peer
func (p *AggGraphMap) RemovePeer(peer string) {
	p.mapLock.Lock()
	defer p.mapLock.Unlock()
	pmap, ok := p.peerMap[peer]
	if !ok {
		logrus.Warningf("Attempting to remove a peer which isn't in the map: %v", peer)
		return
	}

	pmap.cleanup()
	pmap.Stop()
	delete(p.peerMap, peer)
}

func (p *AggGraphMap) GetPeerMap(peer string) *PeerGraphMap {
	p.mapLock.RLock()
	defer p.mapLock.RUnlock()
	ret, _ := p.peerMap[peer]
	return ret
}

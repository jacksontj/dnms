package mapper

import (
	"strconv"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/traceroute"
)

// TODO move elsewhere?
type Peer struct {
	Name string
	Port int
	// TODO: addr etc.
}

func (p *Peer) String() string {
	return p.Name + ":" + strconv.Itoa(p.Port)
}

// Responsible for maintaining a `NetworkGraph` by mapping the network at
// a configured interval
type Mapper struct {
	localName string
	// locking around peers is important-- as there are background jobs mapping
	// and we don't want them adding nodes back after we remove them
	// TODO: more scoped lock? or goroutine?
	peerMap  map[string]*Peer
	peerLock *sync.RWMutex

	// graph of the network
	Graph *graph.NetworkGraph
	// map of how to send a packet on each route
	RouteMap *RouteMap
}

func NewMapper(n string) *Mapper {
	m := &Mapper{
		localName: n,
		peerMap:   make(map[string]*Peer),
		Graph:     graph.Create(),
		RouteMap:  NewRouteMap(),
		peerLock:  &sync.RWMutex{},
	}

	return m
}

func (m *Mapper) AddPeer(p Peer) {
	logrus.Infof("add peer: %v", p)
	m.peerLock.Lock()
	defer m.peerLock.Unlock()
	_, ok := m.peerMap[p.Name]
	if ok {
		logrus.Warning("Mapper asked to add peer that already exists: %v", p)
		return
	} else {
		m.peerMap[p.Name] = &p
	}
}

func (m *Mapper) RemovePeer(p Peer) {
	m.peerLock.Lock()
	defer m.peerLock.Unlock()
	_, ok := m.peerMap[p.Name]
	if ok {
		// TODO: better-- at least its all encapsualated here
		// Remove routes from routemap
		for _, route := range m.RouteMap.RemoveDst(p.String()) {
			if route == nil {
				continue
			}
			m.Graph.DecrRoute(route.Hops())
		}
		// delete the peer
		delete(m.peerMap, p.Name)
	} else {
		logrus.Warning("Mapper asked to remove peer that doesn't exists: %v", p)
	}
}

// TODO: better, since this will be concurrent
func (m *Mapper) IterPeers() chan *Peer {
	peerChan := make(chan *Peer)
	go func() {
		// get list of peer keys
		m.peerLock.RLock()
		pKeys := make([]string, 0, len(m.peerMap))
		for key := range m.peerMap {
			pKeys = append(pKeys, key)
		}
		m.peerLock.RUnlock()
		for _, key := range pKeys {
			m.peerLock.RLock()
			peer, ok := m.peerMap[key]
			m.peerLock.RUnlock()
			if ok {
				peerChan <- peer
			}
		}

		close(peerChan)
	}()
	return peerChan
}

// Start the mapping
func (m *Mapper) Start() {
	go m.mapPeers()
}

// TODO implement stopping
func (m *Mapper) Stop() {

}

// TODO: parallelize the individual peer mapping
// Target for background goroutine responsible for doing the actual mapping
// We specifically map all peers on a given port to effectively get breadth first
// instead of depth first mapping
func (m *Mapper) mapPeers() {
	for {
		// TODO: config
		srcPortStart := 33435
		srcPortEnd := 33500

		for srcPort := srcPortStart; srcPort < srcPortEnd; srcPort++ {
			peerChan := m.IterPeers()
			for peer := range peerChan {
				m.mapPeer(peer, srcPort)
				// TODO configurable rate
				time.Sleep(time.Second)
			}
		}
	}
}

// Map a single peer on a single source port
func (m *Mapper) mapPeer(p *Peer, srcPort int) {

	options := traceroute.TracerouteOptions{}
	options.SetSrcPort(srcPort) // TODO: config
	options.SetDstPort(p.Port)  // TODO: config
	options.SetMaxHops(20)

	ret, err := traceroute.Traceroute(
		p.Name, // TODO: take the IP direct
		&options,
	)
	if err != nil {
		logrus.Infof("Traceroute err: %v", err)
		return
	}

	logrus.Infof("Traceroute %d -> %s: complete", srcPort, p.Name)

	path := make([]string, 0, len(ret.Hops))

	for _, hop := range ret.Hops {
		path = append(path, hop.AddressString())
	}
	logrus.Infof("traceroute path: %v", path)

	currRoute := m.RouteMap.GetRouteOption(m.localName, srcPort, p.Name, p.Port)

	// If we don't have a current route, or the paths differ-- lets update
	if currRoute == nil || !currRoute.SamePath(path) {
		m.peerLock.RLock()
		// check that this peer still exists
		_, ok := m.peerMap[p.Name]
		if ok {
			// Add new one
			newRoute, _ := m.Graph.IncrRoute(path)
			m.RouteMap.UpdateRouteOption(m.localName, srcPort, p.Name, p.Port, newRoute)

			// Remove old one if it exists
			if currRoute != nil {
				m.Graph.DecrRoute(currRoute.Hops())
			}
		}
		m.peerLock.RUnlock()
	}
}

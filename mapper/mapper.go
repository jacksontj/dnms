package mapper

import (
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

// Responsible for maintaining a `NetworkGraph` by mapping the network at
// a configured interval
type Mapper struct {
	peerMap map[string]*Peer

	// graph of the network
	Graph *graph.NetworkGraph
	// map of how to send a packet on each route
	RouteMap *RouteMap
}

func NewMapper() *Mapper {
	return &Mapper{
		peerMap:  make(map[string]*Peer),
		Graph:    graph.Create(),
		RouteMap: NewRouteMap(),
	}
}

func (m *Mapper) AddPeer(p Peer) {
	_, ok := m.peerMap[p.Name]
	if ok {
		logrus.Warning("Mapper asked to add peer that already exists: %v", p)
		return
	} else {
		m.peerMap[p.Name] = &p
	}
}

func (m *Mapper) RemovePeer(p Peer) {
	_, ok := m.peerMap[p.Name]
	if ok {
		delete(m.peerMap, p.Name)
	} else {
		logrus.Warning("Mapper asked to remove peer that doesn't exists: %v", p)
	}
}

// TODO: better, since this will be concurrent
func (m *Mapper) IterPeers(peerChan chan *Peer) {
	go func() {
		for _, peer := range m.peerMap {
			peerChan <- peer
		}

		close(peerChan)
	}()
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
func (m *Mapper) mapPeers() {
	for {
		peerChan := make(chan *Peer)
		m.IterPeers(peerChan)
		for peer := range peerChan {
			m.mapPeer(peer)
			// TODO configurable rate
			time.Sleep(time.Second * 5)
		}
	}
}

// Map a single peer
func (m *Mapper) mapPeer(p *Peer) {
	// TODO: config
	srcPortStart := 33435
	srcPortEnd := 33500

	for srcPort := srcPortStart; srcPort < srcPortEnd; srcPort++ {

		options := traceroute.TracerouteOptions{}
		options.SetSrcPort(srcPort) // TODO: config
		options.SetDstPort(p.Port)  // TODO: config

		ret, err := traceroute.Traceroute(
			p.Name, // TODO: take the IP direct
			&options,
		)
		if err != nil {
			logrus.Infof("Traceroute err: %v", err)
			continue
		}

		logrus.Infof("Traceroute %d -> %s: complete", srcPort, p.Name)

		path := make([]string, 0, len(ret.Hops))

		for _, hop := range ret.Hops {
			path = append(path, hop.AddressString())
		}

		currRoute := m.RouteMap.GetRouteOption(srcPort, p.Name)

		// If we don't have a current route, or the paths differ-- lets update
		if currRoute == nil || !currRoute.SamePath(path) {
			// Add new one
			m.RouteMap.UpdateRouteOption(srcPort, p.Name, m.Graph.IncrRoute(path))

			// Remove old one if it exists
			if currRoute != nil {
				m.Graph.DecrRoute(currRoute.Hops())
			}
		}
	}

}

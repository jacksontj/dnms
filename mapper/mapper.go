package mapper

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/traceroute"
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

// TODO: randomize shuffle (since this is used for mapping and pinging
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
		// shuffle the keys, this way the cluster won't all do them in the
		// same order
		Shuffle(pKeys)
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
		srcPortEnd := 33445

		for srcPort := srcPortStart; srcPort < srcPortEnd; srcPort++ {
			peerChan := m.IterPeers()
			for peer := range peerChan {
				m.mapPeer(peer, srcPort)
				// TODO configurable rate
				time.Sleep(time.Millisecond * 100)
			}
		}
	}
}

// Map a single peer on a single source port
func (m *Mapper) mapPeer(p *Peer, srcPort int) {

	srcIP, err := traceroute.GetLocalIP()
	if err != nil {
		logrus.Errorf("unable to get a local address to send from: %v", err)
		return
	}
	tracerouteOpts := &traceroute.TracerouteOptions{
		SourceAddr: srcIP,
		SourcePort: srcPort,

		DestinationAddr: net.ParseIP(p.Name),
		DestinationPort: p.Port,

		// enumerated value of tcp/udp/icmp
		ProbeType: traceroute.UdpProbe,

		// TTL options
		StartingTTL: 1,
		MaxTTL:      30,

		// Probe options
		ProbeTimeout: time.Second,
		ProbeCount:   1,
	}

	result, err := traceroute.Traceroute(tracerouteOpts)
	if err != nil {
		logrus.Infof("Traceroute err: %v", err)
		return
	}

	logrus.Infof("Traceroute %d -> %s: complete", srcPort, p.Name)

	path := make([]string, 0, len(result.Hops))
	missingPath := make([]int, 0)

	for i, hop := range result.Hops {
		// if there was no address in the response, lets just keep track of it
		// we'll replace it with something unique to annotate this specific unknown node
		if hop.Responses[0].Address == nil {
			path = append(path, "*")
			missingPath = append(missingPath, i)
		} else {
			path = append(path, hop.Responses[0].Address.String())
		}
	}

	// strip out first and last-- this makes the graph more connected, since we
	// aren't really interested in mapping peers-- so much as the network in the
	// middle. We don't lose data, because the RouteMap	keeps track of which peers
	// send down which routes
	// TLDR; the goal is to not have a peer in a `path`
	path = path[:len(path)-1]

	// if there where any names missing (something in missingPath) then lets
	// make a unique name for this missing node.
	// Since we are just mapping, we don't know much about this node-- just
	// that is is between some other number of nodes. Because of this we'll
	// create the name of the node based on the surrounding nodes in the route
	// to avoid large numbers of duplicates (especially if the unknown node is
	// on either end of the route.
	// So if we have a route of: foo -> * -> * -> bar -> baz -> qux
	// the "*" nodes will end up with keys like:
	//		first "*": foo|*|*,bar
	// 		second "*": foo,*|*|,bar
	//
	// Note: the item surrounded by the "|" is the specific node we are looking at
	if len(missingPath) > 0 {
		namesToReplace := make(map[int]string)
		for _, i := range missingPath {
			prefixParts := make([]string, 0)
			suffixParts := make([]string, 0)
			// find the first path entry with a name before us
			for x := i - 1; x >= 0; x-- {
				// Prepend if it exists
				prefixParts = append([]string{path[x]}, prefixParts...)
				if path[x] != "*" {
					break
				}
			}
			// find the first path entry with a name after us
			for x := i + 1; x < len(path); x++ {
				suffixParts = append(suffixParts, path[x])
				if path[x] != "*" {
					break
				}
			}
			prefix := strings.Join(prefixParts, ",")
			suffix := strings.Join(suffixParts, ",")
			namesToReplace[i] = fmt.Sprintf("%s|%s|%s", prefix, "*", suffix)
		}
		// replace all the names
		for i, newHopName := range namesToReplace {
			path[i] = newHopName
		}

	}
	logrus.Debugf("traceroute path: %v", path)

	currRoute := m.RouteMap.GetRouteOption(m.localName, srcPort, p.Name, p.Port)

	// If we don't have a current route, or the paths differ-- lets update
	if currRoute == nil || !currRoute.SamePath(path) {
		m.peerLock.RLock()
		defer m.peerLock.RUnlock()
		// check that this peer still exists
		_, ok := m.peerMap[p.Name]
		if ok {
			// TODO: if the route is compatible (meaning there are fewer links
			// because something returned "*") then lets keep the old one
			// for some period of time

			if currRoute != nil {
				mergedPath, err := graph.MergeRoutePath(currRoute.Hops(), path)
				// if there was no error, we can merge them
				if err == nil {
					logrus.Infof("we have a mergedpath!\na=%v\nb=%v", currRoute.Hops(), path)
					// TODO: migrate/inherit the metrics
					// Add new one
					newRoute, _ := m.Graph.IncrRoute(mergedPath, nil)
					m.RouteMap.UpdateRouteOption(m.localName, srcPort, p.String(), newRoute)

					// Remove old one if it exists
					if currRoute != nil {
						logrus.Infof("replaced path old: %v", currRoute.Hops())
						logrus.Infof("replaced path new: %v", mergedPath)
						m.Graph.DecrRoute(currRoute.Hops())
					} else {
						logrus.Infof("new path: %v", mergedPath)
					}
					// TODO: do this better-- for now this works
					// assuming we have a match, not only do we want to update
					// this routemap entry-- we want to update everyone who is pointing
					// at this route -- since we just made it better
					numChangedRoutes := m.RouteMap.ReplaceRoute(currRoute, newRoute)
					// fix the refcounts
					for x := 0; x < numChangedRoutes; x++ {
						m.Graph.IncrRoute(mergedPath, nil)
						m.Graph.DecrRoute(currRoute.Hops())
					}
					return
				}
			}

			// Add new one
			newRoute, _ := m.Graph.IncrRoute(path, nil)
			m.RouteMap.UpdateRouteOption(m.localName, srcPort, p.String(), newRoute)

			// Remove old one if it exists
			if currRoute != nil {
				logrus.Infof("replaced path old: %v", currRoute.Hops())
				logrus.Infof("replaced path new: %v", path)
				m.Graph.DecrRoute(currRoute.Hops())
			} else {
				logrus.Infof("new path: %v", path)
			}
		}
	}
}

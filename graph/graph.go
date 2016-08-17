// TODO: better name? network topology?
package graph

import (
	"container/ring"
	"crypto/md5"
	"encoding/hex"
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
)

type NetworkGraph struct {
	// nodeName -> Node
	NodesMap  map[string]*NetworkNode `json:"nodes"`
	NodesLock *sync.RWMutex           `json:"-"`

	// nodeName,nodeName -> NetworkLink
	LinksMap  map[string]*NetworkLink `json:"edges"`
	LinksLock *sync.RWMutex           `json:"-"`

	RoutesMap  map[string]*NetworkRoute `json:"routes"`
	RoutesLock *sync.RWMutex            `json:"-"`

	// event stuff
	eventChannels     map[chan *Event]bool
	eventRegistration chan chan *Event
	internalEvents    chan *Event
}

func Create() *NetworkGraph {
	g := &NetworkGraph{
		NodesMap:   make(map[string]*NetworkNode),
		NodesLock:  &sync.RWMutex{},
		LinksMap:   make(map[string]*NetworkLink),
		LinksLock:  &sync.RWMutex{},
		RoutesMap:  make(map[string]*NetworkRoute),
		RoutesLock: &sync.RWMutex{},

		eventChannels:     make(map[chan *Event]bool),
		eventRegistration: make(chan chan *Event),
		internalEvents:    make(chan *Event),
	}

	go g.publisher()

	return g
}

// TODO: buffer messages?
// goroutine target to do all the publishing of events
func (g *NetworkGraph) publisher() {
	for {
		select {
		case newChannel := <-g.eventRegistration:
			g.eventChannels[newChannel] = true
		case newEvent := <-g.internalEvents:
			//logrus.Infof("got a new event! %v", newEvent)
			for subscriberChannel := range g.eventChannels {
				select {
				case subscriberChannel <- newEvent:
					//logrus.Infof("sent that event to a channel")
				default:
					//logrus.Infof("Unable to send event to that subscriber, killing")
					delete(g.eventChannels, subscriberChannel)
					close(subscriberChannel)
				}
			}
		}
	}
}

// TODO: locking here
// Dump everything in the NetworkGraph into a channel
func (g *NetworkGraph) EventDumpChannel() chan *Event {
	// TODO: buffer?
	c := make(chan *Event)
	go func(c chan *Event) {
		// dump nodes
		for _, n := range g.NodesMap {
			c <- &Event{
				E:    addEvent,
				Item: n,
			}
		}
		// dump links
		for _, l := range g.LinksMap {
			c <- &Event{
				E:    addEvent,
				Item: l,
			}
		}
		// dump routes
		for _, route := range g.RoutesMap {
			c <- &Event{
				E:    addEvent,
				Item: route,
			}
		}

		close(c)
	}(c)
	return c
}

// add subscriber to our events
func (g *NetworkGraph) Subscribe(c chan *Event) {
	g.eventRegistration <- c
}

func (g *NetworkGraph) IncrNode(name string) (*NetworkNode, bool) {
	g.NodesLock.Lock()
	defer g.NodesLock.Unlock()
	n, ok := g.NodesMap[name]
	// if this one doesn't exist, lets add it
	if !ok {
		n = NewNetworkNode(name)
		g.NodesMap[name] = n

		// TODO: there is an obvious race here-- the NewNetworkNode() call spawns
		// a gouroutine in the background-- we need to either hold the event (ordering problems)
		// or send an update event after the fact
		g.internalEvents <- &Event{
			E:    addEvent,
			Item: n,
		}
	}
	n.refCount++
	return n, !ok
}

func (g *NetworkGraph) GetNode(name string) *NetworkNode {
	g.NodesLock.RLock()
	defer g.NodesLock.RUnlock()
	n, _ := g.NodesMap[name]
	return n
}

func (g *NetworkGraph) GetNodeCount() int {
	g.NodesLock.RLock()
	defer g.NodesLock.RUnlock()
	return len(g.NodesMap)
}

func (g *NetworkGraph) DecrNode(name string) bool {
	g.NodesLock.Lock()
	defer g.NodesLock.Unlock()
	n, ok := g.NodesMap[name]

	if !ok {
		logrus.Warningf("Attempted to remove node with ip %v which wasn't in the graph", name)
		return false
	}

	n.refCount--
	if n.refCount == 0 {
		delete(g.NodesMap, name)
		g.internalEvents <- &Event{
			E:    removeEvent,
			Item: n,
		}
		return true
	}
	return false
}

func (g *NetworkGraph) IncrLink(src, dst string) (*NetworkLink, bool) {
	key := src + "," + dst
	g.LinksLock.Lock()
	defer g.LinksLock.Unlock()
	l, ok := g.LinksMap[key]
	if !ok {
		srcNode, _ := g.IncrNode(src)
		dstNode, _ := g.IncrNode(dst)
		l = &NetworkLink{
			Src: srcNode,
			Dst: dstNode,
		}
		g.LinksMap[key] = l
		g.internalEvents <- &Event{
			E:    addEvent,
			Item: l,
		}
	}
	l.refCount++
	return l, !ok
}

func (g *NetworkGraph) GetLink(src, dst string) *NetworkLink {
	g.LinksLock.RLock()
	defer g.LinksLock.RUnlock()
	key := src + "," + dst
	l, _ := g.LinksMap[key]
	return l
}

func (g *NetworkGraph) GetLinkCount() int {
	g.LinksLock.RLock()
	defer g.LinksLock.RUnlock()
	return len(g.LinksMap)
}

func (g *NetworkGraph) DecrLink(src, dst string) bool {
	key := src + "," + dst
	g.LinksLock.Lock()
	defer g.LinksLock.Unlock()
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
		g.internalEvents <- &Event{
			E:    removeEvent,
			Item: l,
		}
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

	g.RoutesLock.Lock()
	defer g.RoutesLock.Unlock()

	// check if we have a route for this already
	route, ok := g.RoutesMap[key]
	// if we don't have it, lets make it
	if !ok {
		logrus.Debugf("New Route: key=%s %v", key, hops)
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
			metricRing: ring.New(100), // TODO: config
			mLock:      &sync.RWMutex{},
			updateChan: g.internalEvents,
		}
		g.RoutesMap[key] = route

		g.internalEvents <- &Event{
			E:    addEvent,
			Item: route,
		}
	}

	// increment route's refcount
	route.refCount++

	return route, !ok
}

func (g *NetworkGraph) GetRoute(hops []string) *NetworkRoute {
	g.RoutesLock.RLock()
	defer g.RoutesLock.RUnlock()
	r, _ := g.RoutesMap[g.pathKey(hops)]
	return r
}

func (g *NetworkGraph) GetRouteCount() int {
	g.RoutesLock.RLock()
	defer g.RoutesLock.RUnlock()
	return len(g.RoutesMap)
}

func (g *NetworkGraph) DecrRoute(hops []string) (*NetworkRoute, bool) {
	g.RoutesLock.Lock()
	defer g.RoutesLock.Unlock()
	key := g.pathKey(hops)
	r, ok := g.RoutesMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove route %v which wasn't in the graph", key)
		return nil, false
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
		g.internalEvents <- &Event{
			E:    removeEvent,
			Item: r,
		}
		return r, true
	}
	return r, false
}

// TODO: better name? network topology?
package graph

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"
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

// Create constructs a NetworkGraph and starts the publisher goroutine.
// The publisher exits when ctx is canceled. Pass context.Background() if
// the graph should outlive any one request scope (e.g. process lifetime).
func Create(ctx context.Context) *NetworkGraph {
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

	go g.publisher(ctx)

	return g
}

// TODO: buffer messages?
// goroutine target to do all the publishing of events
func (g *NetworkGraph) publisher(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for c := range g.eventChannels {
				delete(g.eventChannels, c)
				close(c)
			}
			return
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

func (g *NetworkGraph) IncrNode(name string, newNode *NetworkNode) (*NetworkNode, bool) {
	g.NodesLock.Lock()
	defer g.NodesLock.Unlock()
	n, ok := g.NodesMap[name]
	// if this one doesn't exist, lets add it
	if !ok {
		if newNode == nil {
			n = NewNetworkNode(name, g.internalEvents)
		} else {
			n = newNode
			n.updateChan = g.internalEvents
		}
		g.NodesMap[name] = n

		// Now that there is a new thing we fire an addEvent.
		// Note: if the background DNS lookup was initiated an updateEvent
		// will fire as soon as the lookup completes
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

func (g *NetworkGraph) DecrNode(name string) (*NetworkNode, bool) {
	g.NodesLock.Lock()
	defer g.NodesLock.Unlock()
	n, ok := g.NodesMap[name]

	if !ok {
		logrus.Warningf("Attempted to remove node with ip %v which wasn't in the graph", name)
		return nil, false
	}

	n.refCount--
	if n.refCount == 0 {
		delete(g.NodesMap, name)
		g.internalEvents <- &Event{
			E:    removeEvent,
			Item: n,
		}
		return n, true
	}
	return n, false
}

// LinkAttribution is the per-link fault-attribution view produced by
// AttributeLinkLoss. For a given link, we compare the loss rate of routes that
// traverse it against the loss rate of routes that don't — a positive
// Suspicion means routes through this link are losing more often than the
// network-wide baseline, making it the prime suspect for a fault.
type LinkAttribution struct {
	Src              string  `json:"src"`
	Dst              string  `json:"dst"`
	RoutesThrough    int     `json:"routesThrough"`
	RoutesNotThrough int     `json:"routesNotThrough"`
	// Sample-weighted means so chatty routes count more than rarely-pinged ones.
	AvgLossThrough    float64 `json:"avgLossThrough"`
	AvgLossNotThrough float64 `json:"avgLossNotThrough"`
	// Suspicion = AvgLossThrough - AvgLossNotThrough.
	// Positive: this link is correlated with extra loss; rank descending.
	Suspicion float64 `json:"suspicion"`
}

// AttributeLinkLoss correlates per-route loss observations with link
// membership to identify which hops are likely causing loss. For each link L:
//
//	avgLossThrough    = sample-weighted mean lossRate of routes containing L
//	avgLossNotThrough = sample-weighted mean lossRate of routes NOT containing L
//	suspicion         = avgLossThrough - avgLossNotThrough
//
// A healthy link on a noisy path lands near zero (or negative); a genuinely
// bad link surfaces with high suspicion because every route through it
// shares its loss while routes avoiding it don't.
//
// Links with no traversing routes (or no samples on them) are returned with
// zeroed fields so callers can still see "no data" entries.
func (g *NetworkGraph) AttributeLinkLoss() []LinkAttribution {
	// Snapshot routes + their summaries under the routes lock, then drop it
	// before walking links — the links lock is taken inside IncrLink under
	// the routes lock during IncrRoute, so we must not invert that order.
	type routeStat struct {
		route   *NetworkRoute
		loss    float64
		samples int
	}
	g.RoutesLock.RLock()
	stats := make([]routeStat, 0, len(g.RoutesMap))
	for _, r := range g.RoutesMap {
		s := r.metrics.summary()
		if s.NumPoints == 0 {
			continue
		}
		stats = append(stats, routeStat{r, s.LossRate, s.NumPoints})
	}
	g.RoutesLock.RUnlock()

	g.LinksLock.RLock()
	out := make([]LinkAttribution, 0, len(g.LinksMap))
	for _, l := range g.LinksMap {
		var thruRoutes, notRoutes, thruSamples, notSamples int
		var thruLossW, notLossW float64
		for _, s := range stats {
			traverses := false
			for _, rl := range s.route.linkPath {
				if rl == l {
					traverses = true
					break
				}
			}
			if traverses {
				thruRoutes++
				thruSamples += s.samples
				thruLossW += s.loss * float64(s.samples)
			} else {
				notRoutes++
				notSamples += s.samples
				notLossW += s.loss * float64(s.samples)
			}
		}
		attr := LinkAttribution{
			Src:              l.SrcName,
			Dst:              l.DstName,
			RoutesThrough:    thruRoutes,
			RoutesNotThrough: notRoutes,
		}
		if thruSamples > 0 {
			attr.AvgLossThrough = thruLossW / float64(thruSamples)
		}
		if notSamples > 0 {
			attr.AvgLossNotThrough = notLossW / float64(notSamples)
		}
		attr.Suspicion = attr.AvgLossThrough - attr.AvgLossNotThrough
		out = append(out, attr)
	}
	g.LinksLock.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		// Primary: higher suspicion first. Tiebreak by # routes traversing
		// (more evidence beats less).
		if out[i].Suspicion != out[j].Suspicion {
			return out[i].Suspicion > out[j].Suspicion
		}
		return out[i].RoutesThrough > out[j].RoutesThrough
	})
	return out
}

func (g *NetworkGraph) IncrLink(src, dst string, newLink *NetworkLink) (*NetworkLink, bool) {
	key := src + ";" + dst
	g.LinksLock.Lock()
	defer g.LinksLock.Unlock()
	l, ok := g.LinksMap[key]
	if !ok {
		if newLink == nil {
			srcNode, _ := g.IncrNode(src, nil)
			dstNode, _ := g.IncrNode(dst, nil)
			l = &NetworkLink{
				SrcName: srcNode.Name,
				srcNode: srcNode,
				DstName: dstNode.Name,
				dstNode: dstNode,
			}
		} else {
			srcNode, _ := g.IncrNode(src, newLink.srcNode)
			dstNode, _ := g.IncrNode(dst, newLink.dstNode)
			// update child pointers
			newLink.srcNode = srcNode
			newLink.dstNode = dstNode
			l = newLink
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

func (g *NetworkGraph) GetLink(key string) *NetworkLink {
	g.LinksLock.RLock()
	defer g.LinksLock.RUnlock()
	l, _ := g.LinksMap[key]
	return l
}

func (g *NetworkGraph) GetLinkCount() int {
	g.LinksLock.RLock()
	defer g.LinksLock.RUnlock()
	return len(g.LinksMap)
}

func (g *NetworkGraph) DecrLink(src, dst string) (*NetworkLink, bool) {
	key := src + ";" + dst
	g.LinksLock.Lock()
	defer g.LinksLock.Unlock()
	l, ok := g.LinksMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove link %v which wasn't in the graph", key)
		return nil, false
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
		return l, true
	}
	return l, false
}

func (g *NetworkGraph) pathKey(hops []string) string {
	h := md5.New()
	for _, hop := range hops {
		io.WriteString(h, hop)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (g *NetworkGraph) IncrRoute(hops []string, newRoute *NetworkRoute) (*NetworkRoute, bool) {
	key := g.pathKey(hops)

	g.RoutesLock.Lock()
	defer g.RoutesLock.Unlock()

	// check if we have a route for this already
	route, ok := g.RoutesMap[key]
	// if we don't have it, lets make it
	if !ok {
		logrus.Debugf("New Route: key=%s %v", key, hops)
		if newRoute == nil {
			path := make([]*NetworkNode, 0, len(hops))
			linkPath := make([]*NetworkLink, 0, max(0, len(hops)-1))
			for i, hop := range hops {
				hopNode, _ := g.IncrNode(hop, nil)
				path = append(path, hopNode)
				// If there was something prior-- lets add the link as well
				if i-1 >= 0 {
					link, _ := g.IncrLink(hops[i-1], hop, nil)
					linkPath = append(linkPath, link)
				}
			}
			route = &NetworkRoute{
				Path:       hops,
				path:       path,
				linkPath:   linkPath,
				State:      Up,
				metrics:    newMetricsRing(),
				updateChan: g.internalEvents,
			}
		} else {
			newRoute.path = make([]*NetworkNode, len(newRoute.Path))
			newRoute.linkPath = make([]*NetworkLink, 0, max(0, len(newRoute.Path)-1))
			if newRoute.metrics == nil {
				newRoute.metrics = newMetricsRing()
			}
			for i, nodeName := range newRoute.Path {
				// Increment the node (this will convert the name to a pointer)
				node, _ := g.IncrNode(nodeName, nil)
				// set the pointer in our `path`
				newRoute.path[i] = node

				// If there was something prior-- lets add the link as well
				if i-1 >= 0 {
					link, _ := g.IncrLink(newRoute.path[i-1].Name, nodeName, nil)
					newRoute.linkPath = append(newRoute.linkPath, link)
				}
			}
			route = newRoute
			route.updateChan = g.internalEvents
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
	key := g.pathKey(hops)

	g.RoutesLock.Lock()
	defer g.RoutesLock.Unlock()
	r, ok := g.RoutesMap[key]
	if !ok {
		logrus.Warningf("Attempted to remove route %v which wasn't in the graph", key)
		return nil, false
	}

	r.refCount--
	if r.refCount == 0 {
		// decrement all the links/nodes as well
		for i, nodeName := range r.Path {
			g.DecrNode(nodeName)
			if i-1 >= 0 {
				g.DecrLink(r.Path[i-1], nodeName)
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

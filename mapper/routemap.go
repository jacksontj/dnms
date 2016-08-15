package mapper

// Map for port + node -> route
import (
	"strconv"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
)

// TODO: do our own route refcounting (up and down)
type RouteMap struct {
	// "srcPort:nodename" -> route
	NodeRouteMap map[string]*graph.NetworkRoute

	// dstNodename -> NodeRouteMap-Key
	nodeKeyMap map[string]map[string]interface{}

	lock *sync.RWMutex
}

func NewRouteMap() *RouteMap {
	return &RouteMap{
		NodeRouteMap: make(map[string]*graph.NetworkRoute),
		nodeKeyMap:   make(map[string]map[string]interface{}),
		lock:         &sync.RWMutex{},
	}
}

func (r *RouteMap) GetRoute(key string) *graph.NetworkRoute {
	r.lock.RLock()
	defer r.lock.RUnlock()
	route, _ := r.NodeRouteMap[key]
	return route
}

func (r *RouteMap) FindRoute(path []string) (string, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	for key, route := range r.NodeRouteMap {
		if route.SamePathReverse(path) {
			return key, true
		}
	}
	return "", false
}

// TODO: embed the key in the route struct, so we can return a channel of *NetworkRoute
func (r *RouteMap) IterRoutes(name string, keysChan chan string) {
	go func() {
		usedRoutes := make(map[*graph.NetworkRoute]interface{})

		r.lock.RLock()
		nodeMap, ok := r.nodeKeyMap[name]
		r.lock.RUnlock()
		// If there is something, iterate over them and stick the key down the channel
		if ok {
			nodeMapKeys := make([]string, 0)
			r.lock.RLock()
			for key := range nodeMap {
				nodeMapKeys = append(nodeMapKeys, key)
			}
			r.lock.RUnlock()

			for _, key := range nodeMapKeys {
				r.lock.RLock()
				route, _ := r.NodeRouteMap[key]
				r.lock.RUnlock()
				if _, ok := usedRoutes[route]; !ok {
					keysChan <- key
					usedRoutes[route] = struct{}{}
				}
			}
		}

		close(keysChan)
	}()
}

func (r *RouteMap) addNodeKey(name, key string) {
	nMap, ok := r.nodeKeyMap[name]
	if !ok {
		nMap = make(map[string]interface{})
		r.nodeKeyMap[name] = nMap
	}

	nMap[key] = struct{}{}
}

func (r *RouteMap) RemoveNodeKey(name, key string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	nMap, ok := r.nodeKeyMap[name]
	if !ok {
		return
	}

	delete(nMap, key)
}

func (r *RouteMap) GetRouteOption(srcPort int, dst string) *graph.NetworkRoute {
	key := strconv.Itoa(srcPort) + ":" + dst

	r.lock.RLock()
	defer r.lock.RUnlock()
	route, _ := r.NodeRouteMap[key]
	return route
}

//
func (r *RouteMap) UpdateRouteOption(srcPort int, dst string, newRoute *graph.NetworkRoute) {
	key := strconv.Itoa(srcPort) + ":" + dst

	r.lock.Lock()
	defer r.lock.Unlock()

	route, ok := r.NodeRouteMap[key]

	// if it doesn't exist, lets make it
	if !ok || route != newRoute {
		r.NodeRouteMap[key] = newRoute
	}
	r.addNodeKey(dst, key)
}

// TODO: make iterator? This isn't safe concurrency-wise right now
// TODO: do our own route refcounting
// Remove all route options associated with dst
func (r *RouteMap) RemoveDst(dst string) []*graph.NetworkRoute {
	r.lock.Lock()
	defer r.lock.Unlock()
	nodeKeys, ok := r.nodeKeyMap[dst]
	if !ok {
		logrus.Warningf("Removing route options for a dst that isn't in the map: %s", dst)
		return nil
	}
	ret := make([]*graph.NetworkRoute, 0, len(nodeKeys))
	for key := range nodeKeys {
		v, _ := r.NodeRouteMap[key]
		ret = append(ret, v)
		delete(r.NodeRouteMap, key)
	}
	return ret
}

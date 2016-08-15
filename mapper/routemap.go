package mapper

// Map for port + node -> route
import (
	"encoding/json"
	"strconv"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
)

// TODO: do our own route refcounting (up and down)
type RouteMap struct {
	// key == srcName:srcPort,dstName:dstPort
	// key -> route
	NodeRouteMap map[string]*graph.NetworkRoute

	// dstNodeKey -> NodeRouteMap-Key
	dstNodeMap map[string]map[string]interface{}

	// TODO srcNodeMap

	lock *sync.RWMutex
}

func NewRouteMap() *RouteMap {
	return &RouteMap{
		NodeRouteMap:   make(map[string]*graph.NetworkRoute),
		dstNodeMap: make(map[string]map[string]interface{}),
		lock:           &sync.RWMutex{},
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
func (r *RouteMap) IterRoutes(dstKey string, keysChan chan string) {
	go func() {
		usedRoutes := make(map[*graph.NetworkRoute]interface{})

		r.lock.RLock()
		nodeMap, ok := r.dstNodeMap[dstKey]
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

func (r *RouteMap) addNodeKey(dstKey, key string) {
	nMap, ok := r.dstNodeMap[dstKey]
	if !ok {
		nMap = make(map[string]interface{})
		r.dstNodeMap[dstKey] = nMap
	}

	nMap[key] = struct{}{}
}

func (r *RouteMap) removeNodeKey(dstKey, key string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	nMap, ok := r.dstNodeMap[dstKey]
	if !ok {
		return
	}

	delete(nMap, key)
}

func (r *RouteMap) GetRouteOption(srcName string, srcPort int, dstName string, dstPort int) *graph.NetworkRoute {
	key := srcName + ":" + strconv.Itoa(srcPort) + "," + dstName + ":" + strconv.Itoa(dstPort)

	r.lock.RLock()
	defer r.lock.RUnlock()
	route, _ := r.NodeRouteMap[key]
	return route
}

//
func (r *RouteMap) UpdateRouteOption(srcName string, srcPort int, dstName string, dstPort int, newRoute *graph.NetworkRoute) {
	key := srcName + ":" + strconv.Itoa(srcPort) + "," + dstName + ":" + strconv.Itoa(dstPort)

	r.lock.Lock()
	defer r.lock.Unlock()

	route, ok := r.NodeRouteMap[key]

	// if it doesn't exist, lets make it
	if !ok || route != newRoute {
		r.NodeRouteMap[key] = newRoute
	}
	r.addNodeKey(dstName+":"+strconv.Itoa(dstPort), key)
}

// TODO: set port
// TODO: do our own route refcounting
// Remove all route options associated with dst
func (r *RouteMap) RemoveDst(dst string) []*graph.NetworkRoute {
	r.lock.Lock()
	defer r.lock.Unlock()
	nodeKeys, ok := r.dstNodeMap[dst]
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

// Fancy marshal method
func (r *RouteMap) MarshalJSON() ([]byte, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	type Alias RouteMap
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	})
}

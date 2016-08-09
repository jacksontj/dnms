package mapper

// Map for port + node -> route
import (
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
)

// TODO: do our own route refcounting (up and down)
type RouteMap struct {
	// "srcPort:nodename" -> route
	NodeRouteMap map[string]*graph.NetworkRoute

	// nodename -> NodeRouteMap-Key
	nodeKeyMap map[string]map[string]interface{}
}

func NewRouteMap() *RouteMap {
	return &RouteMap{
		NodeRouteMap: make(map[string]*graph.NetworkRoute),
		nodeKeyMap:   make(map[string]map[string]interface{}),
	}
}

func (r *RouteMap) GetRoute(key string) *graph.NetworkRoute {
	route, _ := r.NodeRouteMap[key]
	return route
}

func (r *RouteMap) FindRoute(path []string) (string, bool) {
	for key, route := range r.NodeRouteMap {
		if route.SamePathReverse(path) {
			return key, true
		}
	}
	return "", false
}

func (r *RouteMap) IterRoutes(name string, keysChan chan string) {
	go func() {
		usedRoutes := make(map[*graph.NetworkRoute]interface{})

		nodeMap, ok := r.nodeKeyMap[name]
		// If there is something, iterate over them and stick the key down the channel
		if ok {
			for key := range nodeMap {
				route, _ := r.NodeRouteMap[key]
				if _, ok := usedRoutes[route]; !ok {
					keysChan <- key
					usedRoutes[route] = struct{}{}
				}

			}
		}

		close(keysChan)
	}()
}

func (r *RouteMap) AddNodeKey(name, key string) {
	nMap, ok := r.nodeKeyMap[name]
	if !ok {
		nMap = make(map[string]interface{})
		r.nodeKeyMap[name] = nMap
	}

	nMap[key] = struct{}{}
}

func (r *RouteMap) RemoveNodeKey(name, key string) {
	nMap, ok := r.nodeKeyMap[name]
	if !ok {
		return
	}

	delete(nMap, key)
}

func (r *RouteMap) GetRouteOption(srcPort int, dst string) *graph.NetworkRoute {
	key := strconv.Itoa(srcPort) + ":" + dst

	route, _ := r.NodeRouteMap[key]
	return route
}

//
func (r *RouteMap) UpdateRouteOption(srcPort int, dst string, newRoute *graph.NetworkRoute) {
	key := strconv.Itoa(srcPort) + ":" + dst

	route, ok := r.NodeRouteMap[key]

	// if it doesn't exist, lets make it
	if !ok || route != newRoute {
		r.NodeRouteMap[key] = newRoute
	}

	r.AddNodeKey(dst, key)
}

// TODO: make iterator? This isn't safe concurrency-wise right now
// TODO: do our own route refcounting
// Remove all route options associated with dst
func (r *RouteMap) RemoveDst(dst string) []*graph.NetworkRoute {
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

package main

// Map for port + node -> route
import (
	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
)

type RouteMap struct {
	// "srcPort:nodename" -> route
	nodeRouteMap map[string]*graph.NetworkRoute

	// nodename -> nodeRouteMap-Key
	nodeKeyMap map[string]map[string]interface{}
}

func NewRouteMap() *RouteMap {
	return &RouteMap{
		nodeRouteMap: make(map[string]*graph.NetworkRoute),
		nodeKeyMap:   make(map[string]map[string]interface{}),
	}
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
	key := string(srcPort) + ":" + dst

	route, _ := r.nodeRouteMap[key]
	return route
}

//
func (r *RouteMap) UpdateRouteOption(srcPort int, dst string, newRoute *graph.NetworkRoute) {
	key := string(srcPort) + ":" + dst

	route, ok := r.nodeRouteMap[key]

	// if it doesn't exist, lets make it
	if !ok || route != newRoute {
		r.nodeRouteMap[key] = newRoute
	}

	r.AddNodeKey(dst, key)
}

// Remove all route options associated with dst
func (r *RouteMap) RemoveDst(dst string) {
	nodeKeys, ok := r.nodeKeyMap[dst]
	if !ok {
		logrus.Warningf("Removing route options for a dst that isn't in the map: %s", dst)
		return
	}
	for key := range nodeKeys {
		delete(r.nodeRouteMap, key)
	}
}

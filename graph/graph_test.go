package graph

import (
	"strings"
	"testing"
)

func validateGraph(t *testing.T, g *NetworkGraph, expectedNodes map[string]int, expectedLinks map[string]int, expectedRoutes []RouteTestSpec) {

	// Validate Nodes
	// TODO: test for too many nodes
	for ipString, count := range expectedNodes {
		node := g.GetNode(ipString)
		if node == nil {
			t.Errorf("Node %s missing!", ipString)
		} else {
			if node.refCount != count {
				t.Errorf("Node %s has the wrong refCount expected=%d actual=%d", ipString, count, node.refCount)
			}
		}
	}

	// validate Links
	for linkKey, count := range expectedLinks {
		link := g.GetLink(linkKey)
		if link == nil {
			t.Errorf("Link %v missing!", linkKey)
		} else {
			if link.refCount != count {
				t.Errorf("Link %v has the wrong refCount expected=%d actual=%d", linkKey, count, link.refCount)
			}
		}
	}

	// validate routes
	for _, rSpec := range expectedRoutes {
		route := g.GetRoute(rSpec.Path)
		if route == nil {
			t.Errorf("Route %v missing!", rSpec)
		} else {
			if route.refCount != rSpec.Count {
				t.Errorf("Route %v has the wrong refCount expected=%d actual=%d", rSpec, rSpec.Count, route.refCount)
			}
		}
	}

}

func TestNodes(t *testing.T) {
	g := Create()

	expectedNodes := map[string]int{
		"192.168.1.1": 2,
		"192.168.1.2": 1,
	}

	for ipString, count := range expectedNodes {
		for i := 0; i < count; i++ {
			g.IncrNode(ipString, nil)
		}
	}

	validateGraph(t, g, expectedNodes, nil, nil)
}

func TestLinks(t *testing.T) {
	g := Create()

	expectedLinks := map[string]int{
		"192.168.1.1;192.168.1.2": 2,
		"192.168.1.2;192.168.1.3": 1,
	}

	expectedNodes := map[string]int{
		"192.168.1.1": 1,
		"192.168.1.2": 2,
		"192.168.1.3": 1,
	}

	for linkKey, count := range expectedLinks {
		keyParts := strings.Split(linkKey, ";")
		for i := 0; i < count; i++ {
			g.IncrLink(keyParts[0], keyParts[1], nil)
		}
	}

	validateGraph(t, g, expectedNodes, expectedLinks, nil)
}

// Routes are hard to define in a simple map-- this is cheating ;)
type RouteTestSpec struct {
	Count int
	Path  []string
}

func TestRoutes(t *testing.T) {
	g := Create()
	expectedRoutes := []RouteTestSpec{
		RouteTestSpec{
			Count: 1,
			Path: []string{
				"192.168.1.1",
				"192.168.1.2",
				"192.168.1.3",
				"192.168.1.4",
			},
		},
	}

	expectedLinks := map[string]int{
		"192.168.1.1;192.168.1.2": 1,
		"192.168.1.2;192.168.1.3": 1,
		"192.168.1.3;192.168.1.4": 1,
	}

	expectedNodes := map[string]int{
		"192.168.1.1": 2,
		"192.168.1.2": 3,
		"192.168.1.3": 3,
		"192.168.1.4": 2,
	}

	for _, rSpec := range expectedRoutes {
		for i := 0; i < rSpec.Count; i++ {
			g.IncrRoute(rSpec.Path, nil)
		}
	}

	validateGraph(t, g, expectedNodes, expectedLinks, expectedRoutes)

	// Verify that we ended up with just 1
	if g.GetRouteCount() != 1 {
		t.Errorf("Wrong number of routes! expected=1 actual=%v", g.GetRouteCount())
	}

	for _, rSpec := range expectedRoutes {
		for i := 0; i < rSpec.Count; i++ {
			g.DecrRoute(rSpec.Path)
		}
	}
	// Verify that dec afterwards got us down to 0
	if g.GetRouteCount() != 0 {
		t.Errorf("Wrong number of routes! expected=0 actual=%v", g.GetRouteCount())
	}

}

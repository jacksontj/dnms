package graph

import (
	"net"
	"testing"
)

func validateGraph(t *testing.T, g *NetworkGraph, expectedNodes map[string]int, expectedLinks map[NetworkLinkKey]int, expectedRoutes map[RouteKey]RouteTestSpec) {

	// Validate Nodes
	// TODO: test for too many nodes
	for ipString, count := range expectedNodes {
		node := g.GetNode(net.ParseIP(ipString))
		if node == nil {
			t.Error("Node %s missing!", ipString)
		} else {
			if node.RefCount != count {
				t.Error("Node %s has the wrong refcount expected=%d actual=%d", ipString, count, node.RefCount)
			}
		}
	}

	// validate Links
	for linkKey, count := range expectedLinks {
		srcip := net.ParseIP(linkKey.Src)
		dstip := net.ParseIP(linkKey.Dst)
		link := g.GetLink(srcip, dstip)
		if link == nil {
			t.Error("Link %v missing!", linkKey)
		} else {
			if link.RefCount != count {
				t.Error("Link %v has the wrong refcount expected=%d actual=%d", linkKey, count, link.RefCount)
			}
		}
	}

	// validate routes
	for routeKey, rSpec := range expectedRoutes {
		src, _ := net.ResolveUDPAddr("udp", routeKey.Src)
		dst, _ := net.ResolveUDPAddr("udp", routeKey.Dst)

		route := g.GetRoute(*src, *dst)
		if route == nil {
			t.Error("Route %v missing!", routeKey)
		} else {
			if route.RefCount != rSpec.Count {
				t.Error("Route %v has the wrong refcount expected=%d actual=%d", routeKey, rSpec.Count, route.RefCount)
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
		ip := net.ParseIP(ipString)
		for i := 0; i < count; i++ {
			g.IncrNode(ip)
		}
	}

	validateGraph(t, g, expectedNodes, nil, nil)
}

func TestLinks(t *testing.T) {
	g := Create()

	expectedLinks := map[NetworkLinkKey]int{
		NetworkLinkKey{"192.168.1.1", "192.168.1.2"}: 2,
		NetworkLinkKey{"192.168.1.2", "192.168.1.3"}: 1,
	}

	expectedNodes := map[string]int{
		"192.168.1.1": 1,
		"192.168.1.2": 2,
		"192.168.1.3": 1,
	}

	for linkKey, count := range expectedLinks {
		srcip := net.ParseIP(linkKey.Src)
		dstip := net.ParseIP(linkKey.Dst)
		for i := 0; i < count; i++ {
			g.IncrLink(srcip, dstip)
		}
	}

	validateGraph(t, g, expectedNodes, expectedLinks, nil)
}

// Routes are hard to define in a simple map-- this is cheating ;)
type RouteTestSpec struct {
	Count int
	Path  []net.IP
}

func TestRoutes(t *testing.T) {
	g := Create()
	expectedRoutes := map[RouteKey]RouteTestSpec{
		RouteKey{"192.168.1.1:1", "192.168.1.4:1"}: RouteTestSpec{
			Count: 1,
			Path: []net.IP{
				net.ParseIP("192.168.1.1"),
				net.ParseIP("192.168.1.2"),
				net.ParseIP("192.168.1.3"),
				net.ParseIP("192.168.1.4"),
			},
		},
	}

	expectedLinks := map[NetworkLinkKey]int{
		NetworkLinkKey{"192.168.1.1", "192.168.1.2"}: 1,
		NetworkLinkKey{"192.168.1.2", "192.168.1.3"}: 1,
		NetworkLinkKey{"192.168.1.3", "192.168.1.4"}: 1,
	}

	expectedNodes := map[string]int{
		"192.168.1.1": 1,
		"192.168.1.2": 2,
		"192.168.1.3": 2,
		"192.168.1.4": 1,
	}

	for routeKey, rSpec := range expectedRoutes {
		src, _ := net.ResolveUDPAddr("udp", routeKey.Src)
		dst, _ := net.ResolveUDPAddr("udp", routeKey.Dst)

		for i := 0; i < rSpec.Count; i++ {
			g.AddRoute(*src, *dst, rSpec.Path)
		}
	}

	validateGraph(t, g, expectedNodes, expectedLinks, expectedRoutes)
}

/*
   def test_routes(self):
       routes = {
           (('a', 1), ('z', 1)): [
               'a',
               'b',
               'c',
               'z',
           ],
       }
       expected_routes = {
           (('a', 1), ('z', 1)): 1,
       }

       expected_links = {
           ('a', 'b'): 1,
           ('b', 'c'): 1,
           ('c', 'z'): 1,
       }

       expected_nodes = {
           'a': 1,
           'z': 1,
           # these are 2 since multiple links reference the node
           'b': 2,
           'c': 2,
       }

       for route_key, route in routes.iteritems():
           self.graph.add_route(route_key[0], route_key[1], route)

       self._verify_graph(
           routes=expected_routes,
           links=expected_links,
           nodes=expected_nodes,
       )

       # lets replace the route with a different one
       routes = {
           (('a', 1), ('z', 1)): [
               'a',
               'z',
           ],
       }
       expected_routes = {
           (('a', 1), ('z', 1)): 1,
       }

       expected_links = {
           ('a', 'z'): 1,
       }

       expected_nodes = {
           'a': 1,
           'z': 1,
       }
       for route_key, route in routes.iteritems():
           self.graph.add_route(route_key[0], route_key[1], route)

       self._verify_graph(
           routes=expected_routes,
           links=expected_links,
           nodes=expected_nodes,
       )
*/

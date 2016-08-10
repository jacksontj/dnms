// TODO: separate package (to avoid namespace collisions)
package main

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/mapper"
)

type HTTPApi struct {
	m *mapper.Mapper
}

func (h *HTTPApi) Start() {
	// TODO: use the better mux?
	// Graph endpoints
	http.HandleFunc("/v1/graph", h.showGraph)
	http.HandleFunc("/v1/graph/nodes", h.showNodes)
	http.HandleFunc("/v1/graph/edges", h.showEdges)
	http.HandleFunc("/v1/graph/routes", h.showRoutes)

	// routemap endpoints
	http.HandleFunc("/v1/routemap", h.showRouteMap)

	// event endpoint
	http.HandleFunc("/v1/events", h.eventStream)

	go http.ListenAndServe(":12345", nil)
}

// TODO: better, terrible things are here
func (h *HTTPApi) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

// TODO: put marshal method on Graph
func (h *HTTPApi) showGraph(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.m.Graph)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showNodes(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.m.Graph.NodesMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.NodesMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showEdges(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.m.Graph.LinksMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.LinksMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showRoutes(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.m.Graph.RoutesMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.RoutesMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showRouteMap(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.m.RouteMap.NodeRouteMap)
	if err != nil {
		logrus.Errorf("Unable to marshal RouteMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

// TODO: have an event stream per API endpoint?
// TODO: start with a dump of everything-- then all updates since then (to avoid races)
// TODO: implement stream of events (removal/addition of graph elements, state changes,
// routemap changes, etc.)
func (h *HTTPApi) eventStream(w http.ResponseWriter, r *http.Request) {

}

func httpAPI(m *mapper.Mapper) {
	// TODO: API endpoint
	// Create helpful HTTP endpoint for debugging
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

	})
	go http.ListenAndServe(":12345", nil)
}

// TODO: separate package (to avoid namespace collisions)
package main

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/eventsource"
)

type HTTPApi struct {
	m *mapper.Mapper

	eventBroker *eventsource.Server
}

func NewHTTPApi(m *mapper.Mapper) *HTTPApi {
	api := &HTTPApi{
		m:           m,
		eventBroker: eventsource.NewServer(),
	}

	// TODO: config
	api.eventBroker.AllowCORS = true

	return api
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
	http.HandleFunc("/v1/events/graph", h.eventStreamGraph)

	// Create event listener to pull events from mapper and push into eventBroker
	go func() {
		for {
			// TODO: configurable buffer size?
			c := make(chan *graph.Event, 100)
			// subscriber
			h.m.Graph.Subscribe(c)

			for {
				event, closed := <-c
				// TODO: something to catch up? we'll have dropped events at least :/
				if !closed {
					logrus.Infof("Graph subscriber channel was closed, we might drop some messages")
					break
				}
				h.eventBroker.Publish([]string{"mapper"}, event)
			}
		}

	}()

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
func (h *HTTPApi) eventStreamGraph(w http.ResponseWriter, r *http.Request) {
	graphC := h.m.Graph.EventDumpChannel()
	preloadChannel := make(chan eventsource.Event, 0)

	// goroutine to Convert graph.Event to eventsource.Event
	go func() {
		for {
			e, ok := <-graphC
			if !ok {
				break
			}
			preloadChannel <- e
		}
		close(preloadChannel)
	}()
	handler := h.eventBroker.Handler("mapper", preloadChannel)
	handler(w, r)
}

func httpAPI(m *mapper.Mapper) {
	// TODO: API endpoint
	// Create helpful HTTP endpoint for debugging
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

	})
	go http.ListenAndServe(":12345", nil)
}

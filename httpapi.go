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
	// TODO: think more about the namespacing of this API. Most thing belong to "mapper"
	// but probably want to separate by "topology" "routing" or something like that

	// TODO: use the better mux?
	// Graph endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/graph", h.showGraph)
	mux.HandleFunc("/v1/graph/nodes", h.showNodes)
	mux.HandleFunc("/v1/graph/edges", h.showEdges)
	mux.HandleFunc("/v1/graph/routes", h.showRoutes)

	// all of our peers
	mux.HandleFunc("/v1/mapper/peers", h.showPeers)

	// routemap endpoints
	mux.HandleFunc("/v1/mapper/routemap", h.showRouteMap)

	// event endpoint
	mux.HandleFunc("/v1/events/graph", h.eventStreamGraph)
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

	go http.ListenAndServe(":12345", mux)
}

// TODO: better, terrible things are here
func (h *HTTPApi) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

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

func (h *HTTPApi) showPeers(w http.ResponseWriter, r *http.Request) {
	peers := make([]*mapper.Peer, 0)
	for peer := range h.m.IterPeers() {
		peers = append(peers, peer)
	}
	ret, err := json.Marshal(peers)
	if err != nil {
		logrus.Errorf("Unable to marshal Peers: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showRouteMap(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.m.RouteMap)
	if err != nil {
		logrus.Errorf("Unable to marshal RouteMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

// TODO: have an event stream per API endpoint?
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
	// Create helpful HTTP endpoint for debugging
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

	})
	go http.ListenAndServe(":12345", nil)
}

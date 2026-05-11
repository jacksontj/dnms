// TODO: separate package (to avoid namespace collisions)
package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/eventsource"
	"github.com/sirupsen/logrus"
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

func (h *HTTPApi) Start(ctx context.Context, mux *http.ServeMux) {
	// TODO: think more about the namespacing of this API. Most thing belong to "mapper"
	// but probably want to separate by "topology" "routing" or something like that

	// Graph endpoints
	mux.HandleFunc("/v1/graph", h.showGraph)
	mux.HandleFunc("/v1/graph/nodes", h.showNodes)
	mux.HandleFunc("/v1/graph/edges", h.showEdges)
	mux.HandleFunc("/v1/graph/routes", h.showRoutes)

	// Mapper endpoints
	// all of our peers
	mux.HandleFunc("/v1/mapper/peers", h.showPeers)
	// routemap endpoints
	mux.HandleFunc("/v1/mapper/routemap", h.showRouteMap)

	// events endpoint
	mux.HandleFunc("/v1/events/graph", h.eventStreamGraph)
	// Create event listener to pull events from mapper and push into eventBroker
	go h.republishGraphEvents(ctx)
}

func (h *HTTPApi) republishGraphEvents(ctx context.Context) {
	for ctx.Err() == nil {
		// TODO: configurable buffer size?
		c := make(chan *graph.Event, 100)
		h.m.Graph.Subscribe(c)
		drained := false
		for !drained {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-c:
				if !ok {
					// publisher dropped us as a slow subscriber; resubscribe
					logrus.Infof("Graph subscriber channel was closed, resubscribing")
					drained = true
					continue
				}
				h.eventBroker.Publish([]string{"mapper"}, event)
			}
		}
	}
}

// TODO: better, terrible things are here
func (h *HTTPApi) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func (h *HTTPApi) showGraph(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.NodesLock.RLock()
	h.m.Graph.LinksLock.RLock()
	h.m.Graph.RoutesLock.RLock()
	ret, err := json.Marshal(h.m.Graph)
	h.m.Graph.RoutesLock.RUnlock()
	h.m.Graph.LinksLock.RUnlock()
	h.m.Graph.NodesLock.RUnlock()
	if err != nil {
		logrus.Errorf("Unable to marshal Graph: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showNodes(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.NodesLock.RLock()
	defer h.m.Graph.NodesLock.RUnlock()
	ret, err := json.Marshal(h.m.Graph.NodesMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.NodesMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showEdges(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.LinksLock.RLock()
	defer h.m.Graph.LinksLock.RUnlock()
	ret, err := json.Marshal(h.m.Graph.LinksMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.LinksMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showRoutes(w http.ResponseWriter, r *http.Request) {
	h.m.Graph.RoutesLock.RLock()
	defer h.m.Graph.RoutesLock.RUnlock()
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

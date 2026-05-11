package aggregator

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
	p *AggGraphMap

	eventBroker *eventsource.Server
}

func NewHTTPApi(p *AggGraphMap) *HTTPApi {
	api := &HTTPApi{
		p:           p,
		eventBroker: eventsource.NewServer(),
	}

	// CORS is handled by the HTTPSecurity middleware on the main mux; see
	// the comment in httpapi.go (mapper side) for details.

	return api
}

func (h *HTTPApi) Start(ctx context.Context, mux *http.ServeMux) {
	// TODO: think more about the namespacing of this API. Most thing belong to "mapper"
	// but probably want to separate by "topology" "routing" or something like that

	// Graph endpoints
	mux.HandleFunc("/v1/aggregator/graph", h.showGraph)
	mux.HandleFunc("/v1/aggregator/graph/nodes", h.showNodes)
	mux.HandleFunc("/v1/aggregator/graph/edges", h.showEdges)
	mux.HandleFunc("/v1/aggregator/graph/edges/health", h.showEdgeHealth)
	mux.HandleFunc("/v1/aggregator/graph/routes", h.showRoutes)

	// TODO: aggregate mapper data

	// event endpoint
	mux.HandleFunc("/v1/aggregator/events/graph", h.eventStreamGraph)
	go h.republishGraphEvents(ctx)
}

func (h *HTTPApi) republishGraphEvents(ctx context.Context) {
	for ctx.Err() == nil {
		// TODO: configurable buffer size?
		c := make(chan *graph.Event, 100)
		h.p.Graph.Subscribe(c)
		drained := false
		for !drained {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-c:
				if !ok {
					logrus.Infof("Graph subscriber channel was closed, resubscribing")
					drained = true
					continue
				}
				h.eventBroker.Publish([]string{"mapper"}, event)
			}
		}
	}
}

// setCommonHeaders sets per-response headers that apply to every handler.
// CORS is handled by the HTTPSecurity middleware in main.
func (h *HTTPApi) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}

func (h *HTTPApi) showGraph(w http.ResponseWriter, r *http.Request) {
	ret, err := json.Marshal(h.p.Graph)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showNodes(w http.ResponseWriter, r *http.Request) {
	h.p.Graph.NodesLock.RLock()
	defer h.p.Graph.NodesLock.RUnlock()
	ret, err := json.Marshal(h.p.Graph.NodesMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.NodesMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showEdges(w http.ResponseWriter, r *http.Request) {
	h.p.Graph.LinksLock.RLock()
	defer h.p.Graph.LinksLock.RUnlock()
	ret, err := json.Marshal(h.p.Graph.LinksMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.LinksMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showEdgeHealth(w http.ResponseWriter, r *http.Request) {
	ranked := h.p.Graph.AttributeLinkLoss()
	ret, err := json.Marshal(ranked)
	if err != nil {
		logrus.Errorf("Unable to marshal edge health: %v", err)
		return
	}
	h.setCommonHeaders(w)
	w.Write(ret)
}

func (h *HTTPApi) showRoutes(w http.ResponseWriter, r *http.Request) {
	h.p.Graph.RoutesLock.RLock()
	defer h.p.Graph.RoutesLock.RUnlock()
	ret, err := json.Marshal(h.p.Graph.RoutesMap)
	if err != nil {
		logrus.Errorf("Unable to marshal Graph.RoutesMap: %v", err)
	} else {
		h.setCommonHeaders(w)
		w.Write(ret)
	}
}

func (h *HTTPApi) showPeers(w http.ResponseWriter, r *http.Request) {
	// TODO
	/*
		peers := make([]*mapper.Peer, 0)
		for peer := range h.p.IterPeers() {
			peers = append(peers, peer)
		}
		ret, err := json.Marshal(peers)
		if err != nil {
			logrus.Errorf("Unable to marshal Peers: %v", err)
		} else {
			h.setCommonHeaders(w)
			w.Write(ret)
		}
	*/
}

// TODO: have an event stream per API endpoint?
func (h *HTTPApi) eventStreamGraph(w http.ResponseWriter, r *http.Request) {
	graphC := h.p.Graph.EventDumpChannel()
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

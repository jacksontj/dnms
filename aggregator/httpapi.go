// TODO: separate package (to avoid namespace collisions)
package aggregator

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/jacksontj/dnms/graph"
	"github.com/jacksontj/dnms/mapper"
	"github.com/jacksontj/eventsource"
)

type HTTPApi struct {
	p *PeerGraphMap

	eventBroker *eventsource.Server
}

func NewHTTPApi(p *PeerGraphMap) *HTTPApi {
	api := &HTTPApi{
		p:           p,
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

	// event endpoint
	mux.HandleFunc("/v1/events/graph", h.eventStreamGraph)
	// Create event listener to pull events from mapper and push into eventBroker
	go func() {
		for {
			// TODO: configurable buffer size?
			c := make(chan *graph.Event, 100)
			// subscriber
			h.p.Graph.Subscribe(c)

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

	go http.ListenAndServe(":8888", mux)
}

// TODO: better, terrible things are here
func (h *HTTPApi) setCommonHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

// TODO: put marshal method on Graph
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
// TODO: start with a dump of everything-- then all updates since then (to avoid races)
// TODO: implement stream of events (removal/addition of graph elements, state changes,
// routemap changes, etc.)
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
	// TODO: API endpoint
	// Create helpful HTTP endpoint for debugging
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

	})
	go http.ListenAndServe(":12345", nil)
}

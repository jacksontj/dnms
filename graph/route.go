package graph

import (
	"container/ring"
	"encoding/json"

	"github.com/montanaflynn/stats"
)

// TODO: measure jitter (diff between 2 packet sends)
type RoutePingResponse struct {
	Pass    bool  // Did it ack?
	Latency int64 // Latency (if it ackd)
}

// TODO: RoundTripRoute? Right now the Route is a single direction since we only
// have one side of the traceroute. If the peers gossip about the reverse routes
// then we could potentially have both directions
// TODO: TTL for routes? If we just start up we don't want to have to re-ping the
// world before we are useful
// TODO: stats about route health
type NetworkRoute struct {
	Path []*NetworkNode `json:"path"`

	// Network statistics
	State graphState `json:"state"` // TODO: better handle in the serialization
	// TODO: race condition around this-- either lock it up, or goroutine it
	metricRing *ring.Ring

	// how many are refrencing it
	refCount int
}

func (r *NetworkRoute) HandleACK(pass bool, latency int64) {
	r.metricRing.Value = RoutePingResponse{
		Pass:    pass,
		Latency: latency,
	}
	r.metricRing = r.metricRing.Next()

	// TODO: change to percentage thresholds
	// update state
	if pass == true { // Going up
		switch r.State {
		case Suspect:
			r.State = Up
		case Down:
			r.State = Suspect
		}
	} else { // going down
		switch r.State {
		case Up:
			r.State = Suspect
		case Suspect:
			r.State = Down
		}
	}

}

func (r *NetworkRoute) SamePath(path []string) bool {
	// check len
	if len(path) != len(r.Path) {
		return false
	}

	for i, hop := range path {
		if hop != r.Path[i].Name {
			return false
		}
	}
	return true
}

// is this the same path, just in reverse?
func (r *NetworkRoute) SamePathReverse(path []string) bool {
	// check len
	if len(path) != len(r.Path) {
		return false
	}

	for i, hop := range path {
		if hop != r.Path[len(r.Path)-1-i].Name {
			return false
		}
	}
	return true
}

func (r *NetworkRoute) Hops() []string {
	hops := make([]string, 0, len(r.Path))
	for _, node := range r.Path {
		hops = append(hops, node.Name)
	}
	return hops
}

// Fancy marshal method
func (r *NetworkRoute) MarshalJSON() ([]byte, error) {
	// Convert MetricRing to a list of points
	fail := 0
	// TODO: re-add raw points
	metricPoints := make([]RoutePingResponse, 0, r.metricRing.Len())
	latencies := make([]float64, 0, r.metricRing.Len())
	r.metricRing.Do(func(x interface{}) {
		if x != nil {
			point := x.(RoutePingResponse)
			metricPoints = append(metricPoints, point)
			latencies = append(latencies, float64(point.Latency))
			if !point.Pass {
				fail++
			}
		}
	})

	// Do all metrics calculations here
	metrics := make(map[string]interface{})
	metrics["numPoints"] = len(metricPoints)
	if len(metricPoints) > 0 {
		var totalLatency float64 = 0
		for _, l := range latencies {
			totalLatency += l
		}
		metrics["average"] = float64(totalLatency) / float64(len(metricPoints))
		metrics["lossRate"] = float64(fail) / float64(len(metricPoints))
	} else {
		metrics["average"] = float64(0)
		metrics["lossRate"] = float64(0)
	}
	if dev, err := stats.StandardDeviation(latencies); err == nil {
		metrics["standardDeviation"] = dev
	}

	type Alias NetworkRoute
	return json.Marshal(&struct {
		//MetricPoints []RoutePingResponse
		Metrics map[string]interface{} `json:"metrics"`
		*Alias
	}{
		//MetricPoints: metricPoints,
		Metrics: metrics,
		Alias:   (*Alias)(r),
	})
}

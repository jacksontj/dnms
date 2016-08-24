package graph

import (
	"container/ring"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"

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
	Path []string       `json:"path"`
	path []*NetworkNode `json:"path"`

	// Network statistics
	State graphState `json:"state"` // TODO: better handle in the serialization

	metricRing *ring.Ring
	mLock      *sync.RWMutex

	// how many are refrencing it
	refCount int

	// Channel to send update event on
	updateChan chan *Event
}

// TODO: we make the assumption here that the other route goes away-- which since
// the only caller today is the merge stuff in mapper-- is safe. If that ever
// changes we'll need to be more careful here-- as we are just pointing at
// another ring-- which has its own pings going on
func (r *NetworkRoute) Inherit(o *NetworkRoute) {
	r.mLock.Lock()
	defer r.mLock.Unlock()
	r.metricRing = o.metricRing
}

func (r *NetworkRoute) Key() string {
	h := md5.New()
	for _, node := range r.path {
		io.WriteString(h, node.Name)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (r *NetworkRoute) HandleACK(pass bool, latency int64) {
	r.mLock.Lock()
	defer r.mLock.Unlock()
	r.metricRing.Value = RoutePingResponse{
		Pass:    pass,
		Latency: latency,
	}
	r.metricRing = r.metricRing.Next()

	// TODO: change to percentage thresholds
	// update state
	origState := r.State
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

	// TODO: also send updates when metrics change sufficiently?
	if origState != r.State {
		r.updateChan <- &Event{
			E:    updateEvent,
			Item: r,
		}
	}
}

func (r *NetworkRoute) SamePath(path []string) bool {
	// check len
	if len(path) != len(r.Path) {
		return false
	}

	for i, hop := range path {
		if hop != r.Path[i] {
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
		if hop != r.Path[len(r.Path)-1-i] {
			return false
		}
	}
	return true
}

func (r *NetworkRoute) Hops() []string {
	tmp := make([]string, len(r.Path))
	copy(tmp, r.Path)
	return tmp
}

// Fancy marshal method
func (r *NetworkRoute) MarshalJSON() ([]byte, error) {
	r.mLock.RLock()
	defer r.mLock.RUnlock()
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

// Fancy unmashal method
func (r *NetworkRoute) UnmarshalJSON(data []byte) error {
	type Alias NetworkRoute
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.metricRing = ring.New(100) // TODO: config
	r.mLock = &sync.RWMutex{}
	return nil
}

package graph

import (
	"container/ring"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/montanaflynn/stats"
)

// Better name
type pathMergePuzzlePiece struct {
	// a value we have
	Val string
	// a requirement we have from some other key
	Req map[string]interface{}
}

func newPathMergePuzzlePiece(v string) *pathMergePuzzlePiece {
	return &pathMergePuzzlePiece{
		Val: v,
		Req: make(map[string]interface{}),
	}
}

func (p *pathMergePuzzlePiece) Valid() bool {
	for v, _ := range p.Req {
		// requisites of "*" mean nothing
		if v == "*" {
			continue
		}
		if v != p.Val {
			return false
		}
	}
	return true
}

// Create a "puzzle" for the N constrained list of paths
func CreatePuzzle(paths ...[]string) map[int]*pathMergePuzzlePiece {
	puzzle := make(map[int]*pathMergePuzzlePiece)
	for _, path := range paths {
		for i, v := range path {
			if strings.Contains(v, "*") {
				// set a value for the piece if one doesn't exist
				parentPiece, ok := puzzle[i]
				if !ok {
					parentPiece = newPathMergePuzzlePiece(v)
					puzzle[i] = parentPiece
				} else {
					if parentPiece.Val == "" {
						parentPiece.Val = v
					}
				}

				hopParts := strings.SplitN(v, "|", 3)
				var prefixParts, suffixParts []string
				if strings.Contains(hopParts[0], ",") {
					prefixParts = strings.Split(hopParts[0], ",")
				}
				if strings.Contains(hopParts[2], ",") {
					suffixParts = strings.Split(hopParts[2], ",")
				}

				// add prefix req
				for x, prefixPart := range prefixParts {
					key := i - (len(prefixParts) - x)
					piece, ok := puzzle[key]
					if !ok {
						// TODO: default value?
						piece = newPathMergePuzzlePiece("")
						puzzle[key] = piece
					}
					piece.Req[prefixPart] = struct{}{}
				}

				// add suffix req
				for x, suffixPart := range suffixParts {
					key := i + 1 + x
					piece, ok := puzzle[key]
					if !ok {
						// TODO: default value?
						piece = newPathMergePuzzlePiece("")
						puzzle[key] = piece
					}
					piece.Req[suffixPart] = struct{}{}
				}

			} else {
				piece, ok := puzzle[i]
				if !ok {
					puzzle[i] = newPathMergePuzzlePiece(v)
				} else {
					piece.Val = v
				}
			}
		}
	}
	return puzzle
}

// Given 2 paths, see if we can resolve them into a single list
func MergeRoutePath(o []string, n []string) ([]string, error) {
	if len(o) != len(n) {
		return nil, fmt.Errorf("path lens don't match")
	}
	puzzle := CreatePuzzle(o, n)

	// resolve
	ret := make([]string, len(o))
	for i, piece := range puzzle {
		if !piece.Valid() {
			return nil, fmt.Errorf("doesn't fit")
		}
		ret[i] = piece.Val
	}
	return ret, nil

}

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

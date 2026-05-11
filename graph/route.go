package graph

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"
)

// TODO: measure jitter (diff between 2 packet sends)
//
// RoutePingResponse is retained as a type alias for backward compat with
// callers that constructed the old struct directly.
type RoutePingResponse = PingSample

// TODO: RoundTripRoute? Right now the Route is a single direction since we only
// have one side of the traceroute. If the peers gossip about the reverse routes
// then we could potentially have both directions
// TODO: TTL for routes? If we just start up we don't want to have to re-ping the
// world before we are useful
type NetworkRoute struct {
	Path []string       `json:"path"`
	path []*NetworkNode `json:"-"`

	// linkPath caches the *NetworkLink chain for this route so HandleACK can
	// fan out per-ack metrics without re-resolving links.
	linkPath []*NetworkLink

	// Network statistics
	State graphState `json:"state"` // TODO: better handle in the serialization

	metrics *metricsRing
	stateMu sync.Mutex

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
	r.metrics = o.metrics
}

func (r *NetworkRoute) Key() string {
	h := md5.New()
	for _, node := range r.path {
		io.WriteString(h, node.Name)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (r *NetworkRoute) HandleACK(pass bool, latency int64) {
	r.metrics.record(PingSample{Pass: pass, Latency: latency})

	r.stateMu.Lock()
	origState := r.State
	if pass { // Going up
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
	newState := r.State
	r.stateMu.Unlock()

	// TODO: also send updates when metrics change sufficiently?
	if origState != newState {
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
	type Alias NetworkRoute
	return json.Marshal(&struct {
		Metrics MetricSummary `json:"metrics"`
		*Alias
	}{
		Metrics: r.metrics.summary(),
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
	r.metrics = newMetricsRing()
	return nil
}

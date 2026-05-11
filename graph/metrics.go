package graph

import (
	"container/ring"
	"sync"

	"github.com/montanaflynn/stats"
)

// MetricRingSize is the number of recent samples retained per route. Set
// before constructing the graph (e.g. from main()); subsequent reads happen
// without synchronization so later changes are not safe.
var MetricRingSize = 100

// PingSample is a single ping outcome recorded in the originating route's
// metric ring. Link-level attribution is derived from these route samples
// at query time (see graph.AttributeLinkLoss) rather than fanned out into
// per-link rings.
type PingSample struct {
	Pass    bool  `json:"pass"`
	Latency int64 `json:"latencyNs"`
}

// metricsRing is a fixed-size ring of recent ping samples, safe for
// concurrent use.
type metricsRing struct {
	ring *ring.Ring
	lock sync.RWMutex
}

func newMetricsRing() *metricsRing {
	return &metricsRing{ring: ring.New(MetricRingSize)}
}

func (m *metricsRing) record(sample PingSample) {
	m.lock.Lock()
	m.ring.Value = sample
	m.ring = m.ring.Next()
	m.lock.Unlock()
}

// snapshot copies all currently-populated samples out so the caller can read
// them without holding the ring's lock.
func (m *metricsRing) snapshot() []PingSample {
	m.lock.RLock()
	defer m.lock.RUnlock()
	out := make([]PingSample, 0, m.ring.Len())
	m.ring.Do(func(x interface{}) {
		if x != nil {
			out = append(out, x.(PingSample))
		}
	})
	return out
}

// MetricSummary holds the rolled-up health view of a route or link. Exposed in
// JSON responses so consumers don't have to recompute from raw samples.
type MetricSummary struct {
	NumPoints         int     `json:"numPoints"`
	AverageLatencyNs  float64 `json:"average"`
	LossRate          float64 `json:"lossRate"`
	StandardDeviation float64 `json:"standardDeviation"`
}

func summarize(samples []PingSample) MetricSummary {
	out := MetricSummary{NumPoints: len(samples)}
	if len(samples) == 0 {
		return out
	}
	latencies := make([]float64, 0, len(samples))
	var fail int
	var totalLatency float64
	for _, s := range samples {
		latencies = append(latencies, float64(s.Latency))
		totalLatency += float64(s.Latency)
		if !s.Pass {
			fail++
		}
	}
	out.AverageLatencyNs = totalLatency / float64(len(samples))
	out.LossRate = float64(fail) / float64(len(samples))
	if dev, err := stats.StandardDeviation(latencies); err == nil {
		out.StandardDeviation = dev
	}
	return out
}

func (m *metricsRing) summary() MetricSummary {
	return summarize(m.snapshot())
}

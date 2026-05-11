// TODO: better name? network topology?
package graph

import (
	"encoding/json"
	"sync"
)

type NetworkLink struct {
	SrcName string `json:"src"`
	srcNode *NetworkNode
	DstName string `json:"dst"`
	dstNode *NetworkNode

	// metrics is populated by NetworkRoute.HandleACK for each route traversing
	// this link, so we can attribute loss/latency per hop. Initialized lazily
	// by initMetrics so unmarshaled links (e.g. in the aggregator) get one too.
	metrics     *metricsRing
	metricsOnce sync.Once

	refCount int
}

func (l *NetworkLink) initMetrics() {
	l.metricsOnce.Do(func() {
		l.metrics = newMetricsRing()
	})
}

// recordSample is called by NetworkRoute.HandleACK for every link in the
// route's resolved path.
func (l *NetworkLink) recordSample(s PingSample) {
	l.initMetrics()
	l.metrics.record(s)
}

// Metrics returns the rolled-up health view for this link.
func (l *NetworkLink) Metrics() MetricSummary {
	l.initMetrics()
	return l.metrics.summary()
}

// Fancy marshal method — surface the rolled-up metrics alongside src/dst.
func (l *NetworkLink) MarshalJSON() ([]byte, error) {
	type Alias NetworkLink
	return json.Marshal(&struct {
		Metrics MetricSummary `json:"metrics"`
		*Alias
	}{
		Metrics: l.Metrics(),
		Alias:   (*Alias)(l),
	})
}

// LinkHealth is the on-the-wire shape returned by graph health endpoints —
// flat enough to ingest in dashboards without parsing nested objects.
type LinkHealth struct {
	Src     string  `json:"src"`
	Dst     string  `json:"dst"`
	Samples int     `json:"samples"`
	Loss    float64 `json:"lossRate"`
	AvgNs   float64 `json:"averageLatencyNs"`
}

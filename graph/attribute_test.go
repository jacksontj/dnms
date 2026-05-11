package graph

import (
	"context"
	"testing"
)

// recordSamples drives n synthetic ACKs into a route's metric ring with the
// given loss rate (numFail / n samples fail).
func recordSamples(r *NetworkRoute, n, numFail int) {
	for i := 0; i < n; i++ {
		r.HandleACK(i >= numFail, int64(1_000_000))
	}
}

// findAttr returns the attribution row for the (src,dst) edge, or nil.
func findAttr(rows []LinkAttribution, src, dst string) *LinkAttribution {
	for i := range rows {
		if rows[i].Src == src && rows[i].Dst == dst {
			return &rows[i]
		}
	}
	return nil
}

// TestAttributeLinkLossPinpointsSharedHop sets up three routes — two lossy
// routes from different endpoints that share the link X->Y, and one clean
// route through completely different hops — and verifies that:
//
//  1. X->Y (uniquely shared by the lossy routes) gets the highest suspicion;
//  2. Adjacent links that only appear on one lossy route get a lower score
//     because the un-traversed set averages in the other lossy route too;
//  3. Links only on the clean route show negative suspicion.
func TestAttributeLinkLossPinpointsSharedHop(t *testing.T) {
	g := Create(context.Background())

	// Two lossy routes sharing X->Y but with disjoint endpoints (so Y->B etc.
	// aren't equally implicated). One unrelated clean route.
	rBad1, _ := g.IncrRoute([]string{"A", "X", "Y", "B"}, nil)
	rBad2, _ := g.IncrRoute([]string{"C", "X", "Y", "D"}, nil)
	rClean, _ := g.IncrRoute([]string{"A", "P", "Q", "B"}, nil)

	recordSamples(rBad1, 20, 18) // 90% loss
	recordSamples(rBad2, 20, 18) // 90% loss
	recordSamples(rClean, 20, 0) // 0% loss

	attrs := g.AttributeLinkLoss()

	xy := findAttr(attrs, "X", "Y")
	if xy == nil {
		t.Fatalf("no attribution row for X->Y")
	}
	if xy.Suspicion < 0.85 {
		t.Errorf("X->Y: expected suspicion ≈ 0.9, got %.3f (thru=%.3f not=%.3f)",
			xy.Suspicion, xy.AvgLossThrough, xy.AvgLossNotThrough)
	}

	// The clean links should be exonerated (negative suspicion).
	for _, edge := range [][2]string{{"A", "P"}, {"P", "Q"}, {"Q", "B"}} {
		got := findAttr(attrs, edge[0], edge[1])
		if got == nil {
			t.Fatalf("no attribution row for %s->%s", edge[0], edge[1])
		}
		if got.Suspicion >= 0 {
			t.Errorf("%s->%s: expected negative suspicion (clean link), got %.3f",
				edge[0], edge[1], got.Suspicion)
		}
	}

	// X->Y should rank above adjacent single-route links (A->X, Y->B, C->X,
	// Y->D), since those have one lossy + one clean in their non-traversed
	// set instead of pure clean evidence.
	adjacent := []*LinkAttribution{
		findAttr(attrs, "A", "X"),
		findAttr(attrs, "Y", "B"),
		findAttr(attrs, "C", "X"),
		findAttr(attrs, "Y", "D"),
	}
	for _, a := range adjacent {
		if a == nil {
			t.Fatalf("adjacent link missing from attribution output")
		}
		if a.Suspicion >= xy.Suspicion {
			t.Errorf("%s->%s suspicion (%.3f) should rank below X->Y (%.3f)",
				a.Src, a.Dst, a.Suspicion, xy.Suspicion)
		}
	}

	// Top-ranked output should be the shared bad link.
	if attrs[0].Src != "X" || attrs[0].Dst != "Y" {
		t.Errorf("expected top-ranked link to be X->Y, got %s->%s (suspicion=%.3f)",
			attrs[0].Src, attrs[0].Dst, attrs[0].Suspicion)
	}
}

// TestAttributeLinkLossNoSamplesIsZero verifies that links with no observed
// routes (or routes with no samples) end up with zeroed fields rather than
// NaN or runtime panics.
func TestAttributeLinkLossNoSamplesIsZero(t *testing.T) {
	g := Create(context.Background())
	g.IncrRoute([]string{"A", "B", "C"}, nil) // route exists but no ACKs

	attrs := g.AttributeLinkLoss()
	if len(attrs) == 0 {
		t.Fatalf("expected link rows, got none")
	}
	for _, a := range attrs {
		if a.AvgLossThrough != 0 || a.AvgLossNotThrough != 0 || a.Suspicion != 0 {
			t.Errorf("%s->%s: expected zero attribution with no samples, got %+v", a.Src, a.Dst, a)
		}
	}
}

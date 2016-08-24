package graph

import (
	"testing"
)

func pathMatch(a, b []string) bool {
	if a == nil || b == nil {
		return false
	}
	// check len
	if len(a) != len(b) {
		return false
	}

	for i, hop := range a {
		if hop != b[i] {
			return false
		}
	}
	return true
}

// test that new ones have more things than the old one had
func TestSimpleRoutesForward(t *testing.T) {
	o := []string{"1", "2", "2|*|4", "4"}
	goodNew := []string{"1", "2", "3", "4"}

	newPath, err := MergeRoutePath(o, goodNew)
	if err != nil {
		t.Errorf("error merging clean: %v", err)
	}

	if !pathMatch(newPath, goodNew) {
		t.Error("paths don't match")
	}
}

// test that new routes are missing things the old one had
func TestSimpleRoutesBackward(t *testing.T) {
	o := []string{"1", "2", "3", "4"}
	badNew := []string{"1", "2", "2|*|4", "4"}

	newPath, err := MergeRoutePath(o, badNew)
	if err != nil {
		t.Errorf("error merging clean: %v", err)
	}

	if !pathMatch(newPath, o) {
		t.Errorf("paths don't match \nexpected=%v \ngot=%v", o, newPath)
	}
}

// test that new routes are missing things the old one had
func TestSimpleRoutesBidirectional(t *testing.T) {
	o := []string{"1", "1|*|3", "3", "4"}
	mixedNew := []string{"1", "2", "2|*|4", "4"}
	expected := []string{"1", "2", "3", "4"}

	newPath, err := MergeRoutePath(o, mixedNew)
	if err != nil {
		t.Errorf("error merging clean: %v", err)
	}

	if !pathMatch(newPath, expected) {
		t.Errorf("paths don't match \nexpected=%v \ngot=%v", expected, newPath)
	}
}

// test that new routes are missing things the old one had
func TestComplexRoutesBidirectional(t *testing.T) {
	o := []string{"1", "1|*|*,4", "1,*|*|4", "4", "5"}
	mixedNew := []string{"1", "2", "3", "3|*|5", "5"}
	expected := []string{"1", "2", "3", "4", "5"}

	newPath, err := MergeRoutePath(o, mixedNew)
	if err != nil {
		t.Errorf("error merging clean: %v", err)
	}

	if !pathMatch(newPath, expected) {
		t.Errorf("paths don't match \nexpected=%v \ngot=%v", expected, newPath)
	}
}

// Test that when both are missing a hop-- we get the shorter of the 2
func TestShortenRoutes(t *testing.T) {
	a := []string{"1", "1|*|*,*,5", "1,*|*|*,5", "1,*,*|*|5", "5"}
	b := []string{"1", "2", "2|*|*,5", "2,*|*|5", "5"}
	c := []string{"1", "1|*|*,4", "1,*|*|4", "4", "5"}
	d := []string{"1", "1|*|3", "3", "3|*|5", "5"}

	// tests, a, b, expected
	tests := []([][]string){
		[][]string{a, b, b},
		[][]string{a, c, c},
		[][]string{a, d, d},
	}

	for i, testSpec := range tests {
		newPath, err := MergeRoutePath(testSpec[0], testSpec[1])
		if err != nil {
			t.Errorf("error merging clean: %v", err)
		}

		if !pathMatch(newPath, testSpec[2]) {
			t.Errorf("%d paths don't match \nexpected=%v \ngot=%v", i, testSpec[2], newPath)
		}
	}

}

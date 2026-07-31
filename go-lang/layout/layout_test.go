package layout

import (
	"math"
	"testing"
)

func square(x, y, size float64) Ring {
	return Ring{{x, y}, {x + size, y}, {x + size, y + size}, {x, y + size}}
}

func terr(kind string, rings ...Ring) *Territory {
	poly := Ring2(rings)
	t := &Territory{Kind: kind, Polygons: []Ring2{poly}}
	return t
}

func TestPathFor(t *testing.T) {
	cases := map[string]string{
		"aaa.gdf":            "aaa.layout.json",
		"../aaa.gdf":         "../aaa.layout.json",
		"/boards/class.gdf":  "/boards/class.layout.json",
	}
	for in, want := range cases {
		if got := PathFor(in); got != want {
			t.Errorf("PathFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestArea(t *testing.T) {
	tr := terr("land", square(0, 0, 10))
	if got := tr.Area(); math.Abs(got-100) > 1e-9 {
		t.Errorf("area = %v, want 100", got)
	}
}

func TestAreaSubtractsHoles(t *testing.T) {
	tr := terr("land", square(0, 0, 10), square(3, 3, 4))
	if got := tr.Area(); math.Abs(got-84) > 1e-9 {
		t.Errorf("area = %v, want 84 (100 minus a 4x4 hole)", got)
	}
}

func TestContains(t *testing.T) {
	tr := terr("land", square(0, 0, 10))
	if !tr.Contains(5, 5) {
		t.Error("centre should be inside")
	}
	if tr.Contains(20, 5) {
		t.Error("point outside the square should not be inside")
	}
}

func TestContainsRespectsHoles(t *testing.T) {
	tr := terr("land", square(0, 0, 10), square(3, 3, 4))
	if tr.Contains(5, 5) {
		t.Error("a point in the hole must not count as inside")
	}
	if !tr.Contains(1, 1) {
		t.Error("a point in the ring but outside the hole must be inside")
	}
}

// A territory made of several disjoint pieces -- islands, or a region split
// across the map seam -- must report containment for any of them.
func TestContainsMultiplePolygons(t *testing.T) {
	tr := &Territory{Kind: "land", Polygons: []Ring2{
		{square(0, 0, 5)},
		{square(100, 100, 5)},
	}}
	if !tr.Contains(2, 2) || !tr.Contains(102, 102) {
		t.Error("both pieces should report containment")
	}
	if tr.Contains(50, 50) {
		t.Error("the gap between pieces is not inside")
	}
}

func TestSharedBorderDetectsAbuttingRegions(t *testing.T) {
	a := terr("land", square(0, 0, 10))
	b := terr("land", square(10, 0, 10)) // shares the whole x=10 edge

	// Slightly over 10: measuring within a tolerance also picks up the stretches
	// of the perpendicular edges that pass close to the shared corners.
	got := SharedBorder(a, b, 0.5, 0.75)
	if got < 10 || got > 14 {
		t.Errorf("shared border = %.2f, want a little over 10", got)
	}
}

func TestSharedBorderIgnoresDistantRegions(t *testing.T) {
	a := terr("land", square(0, 0, 10))
	b := terr("land", square(50, 50, 10))

	if got := SharedBorder(a, b, 0.5, 0.75); got != 0 {
		t.Errorf("shared border = %.2f, want 0 for far-apart regions", got)
	}
}

// Two regions meeting only at a corner must not read as adjacent, otherwise
// every four-way junction on the map invents two spurious adjacencies.
func TestSharedBorderCornerTouchIsNegligible(t *testing.T) {
	a := terr("land", square(0, 0, 10))
	b := terr("land", square(10, 10, 10))

	got := SharedBorder(a, b, 0.5, 0.75)
	if got >= Lmin {
		t.Errorf("corner contact measured %.2f, which would count as adjacency", got)
	}
}

func TestLinked(t *testing.T) {
	l := &Layout{Links: []Link{{A: "Eastern US", B: "Western US", Why: "seam"}}}

	if !l.Linked("Eastern US", "Western US") {
		t.Error("declared link not found")
	}
	if !l.Linked("Western US", "Eastern US") {
		t.Error("links must be symmetric")
	}
	if l.Linked("Eastern US", "Brazil") {
		t.Error("unrelated pair reported as linked")
	}
}

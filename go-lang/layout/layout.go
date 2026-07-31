// Package layout loads and validates the map geometry that pairs with a .gdf
// board definition.
//
// Geometry lives in a sibling file: aaa.gdf pairs with aaa.layout.json. The two
// are validated against each other at load time, so a board can never render
// with coordinates belonging to a different board -- the failure mode that made
// the previous hand-authored coordinate table useless.
package layout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Point is a position in layout space.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	// R is the inscribed-circle radius at a label anchor. The renderer hides a
	// label when the territory is too small to hold it at the current zoom.
	R float64 `json:"r,omitempty"`
}

// Ring is a closed ring of points; the first is the outer boundary and any
// others are holes. The final point is not repeated.
type Ring [][2]float64

// Territory is one region's geometry. It carries no colour, owner or
// production: those come from the game state, so a layout survives rule changes.
type Territory struct {
	Kind     string  `json:"kind"` // "land" or "sea"
	Polygons []Ring2 `json:"polygons"`
	Label    Point   `json:"label"`
	Marker   Point   `json:"marker"`
	Capital  *Point  `json:"capital,omitempty"`
	BBox     []float64 `json:"bbox,omitempty"`
}

// Ring2 is one polygon: outer ring first, then holes.
type Ring2 []Ring

// Link is a declared adjacency that the geometry does not express as a shared
// border, recorded deliberately so the front end can draw a connector and the
// validator can distinguish "known" from "newly broken".
type Link struct {
	A   string  `json:"a"`
	B   string  `json:"b"`
	Why string  `json:"why"`
	Gap float64 `json:"gap"`
}

// Layout is a whole board's geometry.
type Layout struct {
	SchemaVersion int                   `json:"schemaVersion"`
	SourceGDF     string                `json:"sourceGdf"`
	GeneratedBy   string                `json:"generatedBy"`
	ViewBox       []float64             `json:"viewBox"`
	Views         map[string][]float64  `json:"views"`
	Territories   map[string]*Territory `json:"territories"`
	Links         []Link                `json:"links"`
}

// PathFor returns the layout file that pairs with a .gdf path.
func PathFor(gdfPath string) string {
	trimmed := strings.TrimSuffix(gdfPath, filepath.Ext(gdfPath))
	return trimmed + ".layout.json"
}

// Load reads a layout file. Raw bytes are returned alongside the parsed value
// so a server can echo the file verbatim without re-marshalling it.
func Load(path string) (*Layout, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("reading layout %s: %w", path, err)
	}

	var l Layout
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, nil, fmt.Errorf("parsing layout %s: %w", path, err)
	}
	if len(l.ViewBox) != 4 {
		return nil, nil, fmt.Errorf("layout %s: viewBox must have 4 numbers, got %d",
			path, len(l.ViewBox))
	}
	if len(l.Territories) == 0 {
		return nil, nil, fmt.Errorf("layout %s: no territories", path)
	}
	return &l, raw, nil
}

// LoadFor loads the layout paired with a .gdf path.
func LoadFor(gdfPath string) (*Layout, []byte, error) {
	return Load(PathFor(gdfPath))
}

// Width and Height of the layout's coordinate space.
func (l *Layout) Width() float64  { return l.ViewBox[2] }
func (l *Layout) Height() float64 { return l.ViewBox[3] }

// Linked reports whether a and b are recorded as a non-touching connection.
func (l *Layout) Linked(a, b string) bool {
	for _, link := range l.Links {
		if (link.A == a && link.B == b) || (link.A == b && link.B == a) {
			return true
		}
	}
	return false
}

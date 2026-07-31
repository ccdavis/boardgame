package main

import (
	"boardgame/models"
	"boardgame/parser"
	"testing"
)

// loadBoard parses the real board definition.
func loadBoard(t testing.TB) *models.Game {
	t.Helper()

	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		t.Fatalf("creating parser: %v", err)
	}
	g, err := p.Parse()
	if err != nil {
		t.Fatalf("parsing ../aaa.gdf: %v", err)
	}
	return g
}

// TestBoardGraphIsWellFormed is the standing guard on the shipped board. Every
// map-layout check downstream assumes the adjacency graph is symmetric,
// connected and simple; if that stops being true, geometry work built on top of
// it is chasing a moving target.
func TestBoardGraphIsWellFormed(t *testing.T) {
	g := loadBoard(t)

	problems := g.ValidateGraph()
	if len(problems) == 0 {
		return
	}

	t.Errorf("aaa.gdf adjacency graph has %d problem(s):", len(problems))
	for _, p := range problems {
		t.Errorf("  %s", p)
	}
}

// TestBoardIslandsAreReachableBySea checks that every land territory with no
// land neighbour borders at least one sea zone. An island that borders none is
// unreachable by any unit.
//
// Note the rule is "at least one", not "exactly one": Australia is a
// continent-island bordering three sea zones. The map layout validator's
// containment check has to allow for that.
func TestBoardIslandsAreReachableBySea(t *testing.T) {
	g := loadBoard(t)

	for name, terr := range g.Board {
		if terr.Terrain != models.Land {
			continue
		}

		landNeighbours, seaNeighbours := 0, 0
		for _, nb := range terr.ConnectedTo {
			if nb.Terrain == models.Water {
				seaNeighbours++
			} else {
				landNeighbours++
			}
		}

		if landNeighbours == 0 && seaNeighbours == 0 {
			t.Errorf("island %q borders neither land nor sea; it is unreachable", name)
		}
	}
}

// TestMadagascarIsReachedOnlyViaItsSeaZone pins the specific data repair made in
// M0a. Four ocean zones used to list the island of Madagascar directly, which
// both bypassed Madagascar Sea and made the graph non-planar.
func TestMadagascarIsReachedOnlyViaItsSeaZone(t *testing.T) {
	g := loadBoard(t)

	madagascar := g.Board["Madagascar"]
	if madagascar == nil {
		t.Fatal("Madagascar not found on the board")
	}

	if len(madagascar.ConnectedTo) != 1 {
		var names []string
		for _, nb := range madagascar.ConnectedTo {
			names = append(names, nb.Name)
		}
		t.Fatalf("Madagascar should border only Madagascar Sea, got %v", names)
	}
	if got := madagascar.ConnectedTo[0].Name; got != "Madagascar Sea" {
		t.Errorf("Madagascar's only neighbour should be Madagascar Sea, got %q", got)
	}
}

// TestBoardSeaZonesAreConnectedToEachOther guards against a sea zone that can
// only be entered from land, which would strand any fleet that sailed into it.
func TestBoardSeaZonesAreConnectedToEachOther(t *testing.T) {
	g := loadBoard(t)

	for name, terr := range g.Board {
		if terr.Terrain != models.Water {
			continue
		}
		hasSeaNeighbour := false
		for _, nb := range terr.ConnectedTo {
			if nb.Terrain == models.Water {
				hasSeaNeighbour = true
				break
			}
		}
		if !hasSeaNeighbour {
			t.Errorf("sea zone %q has no adjacent sea zone; a fleet entering it could never leave", name)
		}
	}
}

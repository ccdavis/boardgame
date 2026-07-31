package models

import "testing"

// buildGraph makes a board from an adjacency spec. Edges are added exactly as
// written (one direction only), so tests can construct asymmetric graphs that
// ConnectTerritories would refuse to produce.
func buildGraph(spec map[string][]string) *Game {
	g := NewGame()
	for name := range spec {
		g.Board[name] = &Territory{Name: name}
	}
	for name, neighbours := range spec {
		for _, nb := range neighbours {
			if t, ok := g.Board[nb]; ok {
				g.Board[name].ConnectedTo = append(g.Board[name].ConnectedTo, t)
			}
		}
	}
	return g
}

func kinds(problems []GraphProblem) map[GraphProblemKind]int {
	counts := make(map[GraphProblemKind]int)
	for _, p := range problems {
		counts[p.Kind]++
	}
	return counts
}

func TestValidateGraph_CleanGraphHasNoProblems(t *testing.T) {
	g := buildGraph(map[string][]string{
		"A": {"B"},
		"B": {"A", "C"},
		"C": {"B"},
	})

	if problems := g.ValidateGraph(); len(problems) != 0 {
		t.Errorf("expected no problems, got %d:", len(problems))
		for _, p := range problems {
			t.Errorf("  %s", p)
		}
	}
}

func TestValidateGraph_DetectsAsymmetry(t *testing.T) {
	// A claims B as a neighbour; B does not reciprocate.
	g := buildGraph(map[string][]string{
		"A": {"B"},
		"B": {"C"},
		"C": {"B"},
	})

	problems := g.ValidateGraph()
	if got := kinds(problems)[GraphAsymmetric]; got != 1 {
		t.Fatalf("expected 1 asymmetric edge, got %d (%v)", got, problems)
	}
	for _, p := range problems {
		if p.Kind == GraphAsymmetric && (p.A != "A" || p.B != "B") {
			t.Errorf("expected the A--B edge to be reported, got %s", p)
		}
	}
}

func TestValidateGraph_DetectsSelfLoop(t *testing.T) {
	g := buildGraph(map[string][]string{"A": {"A", "B"}, "B": {"A"}})

	if got := kinds(g.ValidateGraph())[GraphSelfLoop]; got != 1 {
		t.Errorf("expected 1 self loop, got %d", got)
	}
}

func TestValidateGraph_DetectsDuplicateEdge(t *testing.T) {
	g := buildGraph(map[string][]string{"A": {"B", "B"}, "B": {"A"}})

	if got := kinds(g.ValidateGraph())[GraphDuplicate]; got != 1 {
		t.Errorf("expected 1 duplicate edge, got %d", got)
	}
}

func TestValidateGraph_DetectsIsolatedTerritory(t *testing.T) {
	g := buildGraph(map[string][]string{"A": {"B"}, "B": {"A"}, "Lonely": {}})

	counts := kinds(g.ValidateGraph())
	if counts[GraphIsolated] != 1 {
		t.Errorf("expected 1 isolated territory, got %d", counts[GraphIsolated])
	}
}

func TestValidateGraph_DetectsDisconnectedComponent(t *testing.T) {
	// Two well-formed components that never touch.
	g := buildGraph(map[string][]string{
		"A": {"B"}, "B": {"A"},
		"Y": {"Z"}, "Z": {"Y"},
	})

	problems := g.ValidateGraph()
	if got := kinds(problems)[GraphDisconnected]; got != 2 {
		t.Errorf("expected 2 unreachable territories, got %d (%v)", got, problems)
	}
}

func TestValidateGraph_DetectsDanglingOwner(t *testing.T) {
	g := buildGraph(map[string][]string{"A": {"B"}, "B": {"A"}})
	g.Board["A"].Owner = &Player{Name: "Atlantis"} // never registered in g.Players

	if got := kinds(g.ValidateGraph())[GraphDanglingOwner]; got != 1 {
		t.Errorf("expected 1 dangling owner, got %d", got)
	}
}

// ConnectTerritories must record adjacency in both directions. Before this was
// fixed, an edge declared from only one side in the .gdf stayed one-way in the
// engine -- a fleet in Madagascar Sea could never invade Madagascar.
func TestConnectTerritories_IsBidirectional(t *testing.T) {
	g := NewGame()
	g.Board["Madagascar"] = &Territory{Name: "Madagascar"}
	g.Board["Madagascar Sea"] = &Territory{Name: "Madagascar Sea"}

	if err := g.ConnectTerritories("Madagascar", "Madagascar Sea"); err != nil {
		t.Fatalf("ConnectTerritories: %v", err)
	}

	if !adjacentTo(g.Board["Madagascar"], g.Board["Madagascar Sea"]) {
		t.Error("forward edge missing")
	}
	if !adjacentTo(g.Board["Madagascar Sea"], g.Board["Madagascar"]) {
		t.Error("reverse edge missing: adjacency must be symmetric")
	}
	if problems := g.ValidateGraph(); len(problems) != 0 {
		t.Errorf("expected a clean graph, got %v", problems)
	}
}

func TestConnectTerritories_IsIdempotent(t *testing.T) {
	g := NewGame()
	g.Board["A"] = &Territory{Name: "A"}
	g.Board["B"] = &Territory{Name: "B"}

	for i := 0; i < 3; i++ {
		if err := g.ConnectTerritories("A", "B"); err != nil {
			t.Fatalf("ConnectTerritories: %v", err)
		}
		// Declaring the same edge from the other side must not duplicate it either.
		if err := g.ConnectTerritories("B", "A"); err != nil {
			t.Fatalf("ConnectTerritories: %v", err)
		}
	}

	if n := len(g.Board["A"].ConnectedTo); n != 1 {
		t.Errorf("A should have 1 neighbour, got %d", n)
	}
	if n := len(g.Board["B"].ConnectedTo); n != 1 {
		t.Errorf("B should have 1 neighbour, got %d", n)
	}
}

func TestConnectTerritories_UnknownTerritory(t *testing.T) {
	g := NewGame()
	g.Board["A"] = &Territory{Name: "A"}

	if err := g.ConnectTerritories("A", "Nowhere"); err == nil {
		t.Error("expected an error connecting to an undeclared territory")
	}
	if err := g.ConnectTerritories("Nowhere", "A"); err == nil {
		t.Error("expected an error connecting from an undeclared territory")
	}
}

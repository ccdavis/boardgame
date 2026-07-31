package main

import (
	"boardgame/models"
	"boardgame/parser"
	"testing"
)

// TestGameLoader tests loading the aaa.gdf file
func TestGameLoader(t *testing.T) {
	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	game, err := p.Parse()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	// Verify basic game structure
	if len(game.Players) == 0 {
		t.Error("Expected at least one player")
	}

	if len(game.Board) == 0 {
		t.Error("Expected at least one territory")
	}

	if len(game.GlobalPieceTemplates) == 0 {
		t.Error("Expected at least one unit type")
	}

	if len(game.Pieces) == 0 {
		t.Error("Expected at least one piece to be placed")
	}

	// Invariants rather than exact counts.
	//
	// These used to assert 6 players / 127 territories / 298 pieces. Those
	// numbers describe one particular board, so every edit to aaa.gdf broke the
	// test without anything actually being wrong -- and a test that has to be
	// updated whenever the data changes stops being read. What matters is that
	// the parsed board is internally consistent.
	if len(game.Players) < 2 {
		t.Errorf("expected at least two powers, got %d", len(game.Players))
	}

	if len(game.Board) < 50 {
		t.Errorf("expected a full board, got only %d territories", len(game.Board))
	}

	if len(game.GlobalPieceTemplates) < 5 {
		t.Errorf("expected the unit roster, got %d types", len(game.GlobalPieceTemplates))
	}

	// Every placed piece must be a declared unit type.
	for id, piece := range game.Pieces {
		if _, ok := game.GlobalPieceTemplates[piece.Name]; !ok {
			t.Errorf("piece %d is a %q, which is not a declared unit type", id, piece.Name)
		}
	}

	// Every piece must sit in exactly one territory. A piece in none of them is
	// leaked and will never be seen again; a piece in two is double-counted.
	holdings := make(map[int]string)
	for name, territory := range game.Board {
		for _, id := range territory.Pieces {
			if previous, seen := holdings[id]; seen {
				t.Errorf("piece %d is in both %q and %q", id, previous, name)
			}
			holdings[id] = name
		}
	}
	for id := range game.Pieces {
		if _, placed := holdings[id]; !placed {
			// Cargo legitimately lives inside a transport rather than on the board.
			carried := false
			for _, other := range game.Pieces {
				for _, heldID := range other.Holding {
					if heldID == id {
						carried = true
					}
				}
			}
			if !carried {
				t.Errorf("piece %d (%s) is in no territory and is carried by nothing",
					id, game.Pieces[id].Name)
			}
		}
	}

	// The adjacency graph must be symmetric, connected and simple.
	if problems := game.ValidateGraph(); len(problems) > 0 {
		t.Errorf("board graph has %d problem(s):", len(problems))
		for _, p := range problems {
			t.Errorf("  %s", p)
		}
	}

	// Verify territories are connected
	germany := game.Board["Germany"]
	if germany == nil {
		t.Fatal("Germany territory not found")
	}

	if len(germany.ConnectedTo) == 0 {
		t.Error("Germany should be connected to other territories")
	}

	// Verify pieces are placed
	if len(germany.Pieces) == 0 {
		t.Error("Germany should have pieces placed on it")
	}
}

// TestChangeOwnership tests the territory ownership transfer logic
func TestChangeOwnership(t *testing.T) {
	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	game, err := p.Parse()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	// Get UK and Japan players
	uk := game.Players["UK"]
	japan := game.Players["Japan"]

	if uk == nil || japan == nil {
		t.Fatal("Could not find UK or Japan players")
	}

	// Get a UK territory
	if len(uk.Territories) == 0 {
		t.Fatal("UK has no territories")
	}

	ukTerritory := uk.Territories[0]
	territoryName := ukTerritory.Name

	// Count initial territories
	ukInitialCount := len(uk.Territories)
	japanInitialCount := len(japan.Territories)

	// Verify initial ownership
	if ukTerritory.Owner != uk {
		t.Error("Territory should initially be owned by UK")
	}

	// Change ownership to Japan
	models.ChangeOwnership(ukTerritory, japan)

	// Verify ownership changed
	if ukTerritory.Owner != japan {
		t.Error("Territory ownership should have changed to Japan")
	}

	// Verify territory counts updated
	if len(uk.Territories) != ukInitialCount-1 {
		t.Errorf("UK should have one less territory. Expected %d, got %d",
			ukInitialCount-1, len(uk.Territories))
	}

	if len(japan.Territories) != japanInitialCount+1 {
		t.Errorf("Japan should have one more territory. Expected %d, got %d",
			japanInitialCount+1, len(japan.Territories))
	}

	// Verify the territory is in Japan's list
	found := false
	for _, t := range japan.Territories {
		if t.Name == territoryName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Territory %s not found in Japan's territory list", territoryName)
	}

	// Verify the territory is NOT in UK's list
	for _, terr := range uk.Territories {
		if terr.Name == territoryName {
			t.Errorf("Territory %s should not be in UK's territory list anymore", territoryName)
		}
	}
}

// TestTerritoriesConnected verifies that territory connections are properly established
func TestTerritoriesConnected(t *testing.T) {
	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	game, err := p.Parse()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	// Test a few known connections from aaa.gdf
	germany := game.Board["Germany"]
	if germany == nil {
		t.Fatal("Germany not found")
	}

	// Germany should be connected to Western Europe, Southern Europe, Eastern Europe, etc.
	connectedNames := make(map[string]bool)
	for _, territory := range germany.ConnectedTo {
		connectedNames[territory.Name] = true
	}

	expectedConnections := []string{"Western Europe", "Southern Europe", "Eastern Europe", "North Sea", "Baltic Sea"}
	for _, expected := range expectedConnections {
		if !connectedNames[expected] {
			t.Errorf("Germany should be connected to %s", expected)
		}
	}
}

// TestPiecePlacement verifies pieces are correctly placed on territories
func TestPiecePlacement(t *testing.T) {
	p, err := parser.NewParser("../aaa.gdf")
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	game, err := p.Parse()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	// Check that Germany has pieces
	germany := game.Board["Germany"]
	if len(germany.Pieces) == 0 {
		t.Error("Germany should have pieces placed on it")
	}

	// According to aaa.gdf, Germany should have: 5 infantry,4 armor,1 fighter,1 bomber,1 AAA,1 factory
	pieces := game.GetPiecesInTerritory("Germany")
	if len(pieces) != 13 {
		t.Errorf("Germany should have 13 pieces, got %d", len(pieces))
	}

	// Count piece types
	pieceCounts := make(map[string]int)
	for _, piece := range pieces {
		pieceCounts[piece.Name]++
	}

	expected := map[string]int{
		"infantry": 5,
		"armor":    4,
		"fighter":  1,
		"bomber":   1,
		"AAA":      1,
		"factory":  1,
	}

	for pieceType, expectedCount := range expected {
		if pieceCounts[pieceType] != expectedCount {
			t.Errorf("Germany should have %d %s, got %d",
				expectedCount, pieceType, pieceCounts[pieceType])
		}
	}
}

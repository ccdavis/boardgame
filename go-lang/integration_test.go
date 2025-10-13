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

	// Verify specific counts from aaa.gdf
	if len(game.Players) != 6 {
		t.Errorf("Expected 6 players, got %d", len(game.Players))
	}

	if len(game.Board) != 127 {
		t.Errorf("Expected 127 territories, got %d", len(game.Board))
	}

	if len(game.GlobalPieceTemplates) != 10 {
		t.Errorf("Expected 10 unit types, got %d", len(game.GlobalPieceTemplates))
	}

	if len(game.Pieces) != 298 {
		t.Errorf("Expected 298 pieces, got %d", len(game.Pieces))
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

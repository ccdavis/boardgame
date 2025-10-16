package game

import (
	"boardgame/models"
	"boardgame/parser"
	"path/filepath"
	"testing"
)

// TestInvalidMovementsOnRealMap tests that infantry cannot make invalid moves like
// Paris -> London (English Channel in the way), Philippines -> India (sea zones in the way), etc.
func TestInvalidMovementsOnRealMap(t *testing.T) {
	// Load the real game map
	gdfPath := filepath.Join("..", "..", "aaa.gdf")
	p, err := parser.NewParser(gdfPath)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	g, err := p.Parse()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	// Set up player sides
	g.Players["Germany"].Side = "Axis"
	g.Players["UK"].Side = "Allies"

	// Create a test infantry piece owned by Germany
	infantryTemplate := g.GlobalPieceTemplates["infantry"]
	testPiece := &models.Piece{
		Name:     "infantry",
		Terrain:  infantryTemplate.Terrain,
		Movement: infantryTemplate.Movement,
		Attack:   infantryTemplate.Attack,
		Defend:   infantryTemplate.Defend,
		Cost:     infantryTemplate.Cost,
	}

	testCases := []struct {
		name        string
		from        string
		to          string
		shouldFail  bool
		description string
	}{
		{
			name:        "Paris to London - should fail (English Channel blocks)",
			from:        "Western Europe", // Paris
			to:          "Britain",        // London
			shouldFail:  true,
			description: "Infantry cannot cross the English Channel without transport",
		},
		{
			name:        "Philippines to India - should fail (sea zones block)",
			from:        "Philipines",
			to:          "India",
			shouldFail:  true,
			description: "Infantry cannot cross multiple sea zones",
		},
		{
			name:        "Berlin to Moscow directly - should fail (must go through Poland/Ukraine)",
			from:        "Germany",
			to:          "Russia",
			shouldFail:  true,
			description: "Infantry movement=1, cannot reach Moscow from Berlin in one turn",
		},
		{
			name:        "Germany to Western Europe - should succeed (adjacent land)",
			from:        "Germany",
			to:          "Western Europe",
			shouldFail:  false,
			description: "Adjacent land territories should be reachable",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate if the piece can reach the destination
			distance, path, err := CalculateMovementDistanceForPiece(g, testPiece, tc.from, tc.to)

			if tc.shouldFail {
				// We expect this to fail - either error or distance too far
				if err == nil && distance <= int(testPiece.Movement) {
					t.Errorf("%s: Expected move to fail, but it succeeded. Distance=%d, Path=%v",
						tc.description, distance, path)
				}
			} else {
				// We expect this to succeed
				if err != nil {
					t.Errorf("%s: Expected move to succeed, but got error: %v",
						tc.description, err)
				} else if distance > int(testPiece.Movement) {
					t.Errorf("%s: Distance %d exceeds movement %d. Path=%v",
						tc.description, distance, testPiece.Movement, path)
				}
			}
		})
	}
}

// TestAirUnitsCanCrossWater tests that air units can move over water
func TestAirUnitsCanCrossWater(t *testing.T) {
	gdfPath := filepath.Join("..", "..", "aaa.gdf")
	p, err := parser.NewParser(gdfPath)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}

	g, err := p.Parse()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	// Create a fighter (air unit)
	fighterTemplate := g.GlobalPieceTemplates["fighter"]
	fighter := &models.Piece{
		Name:     "fighter",
		Terrain:  fighterTemplate.Terrain,
		Movement: fighterTemplate.Movement, // Usually 4
		Attack:   fighterTemplate.Attack,
		Defend:   fighterTemplate.Defend,
		Cost:     fighterTemplate.Cost,
	}

	// Fighter should be able to fly from Britain to Western Europe (over English Channel)
	distance, path, err := CalculateMovementDistanceForPiece(g, fighter, "Britain", "Western Europe")
	if err != nil {
		t.Errorf("Fighter should be able to fly from Britain to Western Europe, but got error: %v", err)
	}
	if distance > int(fighter.Movement) {
		t.Errorf("Fighter distance %d exceeds movement %d. Path=%v", distance, fighter.Movement, path)
	}
}

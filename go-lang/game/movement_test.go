package game

import (
	"boardgame/models"
	"testing"
)

// createMovementTestGame creates a game for movement testing
func createMovementTestGame() *models.Game {
	game := models.NewGame()

	// Add players
	game.PlayerOrder = []string{"USSR", "Germany"}
	game.GetOrCreatePlayer("USSR")
	game.GetOrCreatePlayer("Germany")

	// Create a chain of connected territories
	// Moscow -> Karelia -> Finland -> Norway -> North Sea -> London
	game.AddTerritory("Moscow", models.Land, "USSR", 8)
	game.AddTerritory("Karelia", models.Land, "USSR", 2)
	game.AddTerritory("Finland", models.Land, "Germany", 2)
	game.AddTerritory("Norway", models.Land, "Germany", 3)
	game.AddTerritory("North Sea", models.Water, "Neutral", 0)
	game.AddTerritory("London", models.Land, "UK", 8)

	// Connect them in a chain
	game.ConnectTerritories("Moscow", "Karelia")
	game.ConnectTerritories("Karelia", "Finland")
	game.ConnectTerritories("Finland", "Norway")
	game.ConnectTerritories("Norway", "North Sea")
	game.ConnectTerritories("North Sea", "London")

	// Add bidirectional connections
	game.ConnectTerritories("Karelia", "Moscow")
	game.ConnectTerritories("Finland", "Karelia")
	game.ConnectTerritories("Norway", "Finland")
	game.ConnectTerritories("North Sea", "Norway")
	game.ConnectTerritories("London", "North Sea")

	// Add unit templates
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.AddPieceTemplate("armor", models.Land, 2, 3, 3, 5)
	game.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 10)
	game.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	game.AddPieceTemplate("bomber", models.Air, 6, 4, 1, 15)

	return game
}

// TestValidateMovementBasic tests basic movement validation
func TestValidateMovementBasic(t *testing.T) {
	game := createMovementTestGame()

	// Place infantry in Moscow
	game.PlacePieces("Moscow", "infantry", 1)
	infantryID := 1

	// Valid move to connected territory
	err := ValidateMovement(game, infantryID, "Moscow", "Karelia", CombatMove)
	if err != nil {
		t.Errorf("Valid move should succeed: %v", err)
	}
}

// TestValidateMovementPieceNotFound tests error when piece doesn't exist
func TestValidateMovementPieceNotFound(t *testing.T) {
	game := createMovementTestGame()

	err := ValidateMovement(game, 999, "Moscow", "Karelia", CombatMove)
	if err == nil {
		t.Error("Expected error for non-existent piece")
	}
}

// TestValidateMovementTerritoryNotFound tests error when territory doesn't exist
func TestValidateMovementTerritoryNotFound(t *testing.T) {
	game := createMovementTestGame()
	game.PlacePieces("Moscow", "infantry", 1)

	err := ValidateMovement(game, 1, "Moscow", "Atlantis", CombatMove)
	if err == nil {
		t.Error("Expected error for non-existent territory")
	}

	err = ValidateMovement(game, 1, "Atlantis", "Moscow", CombatMove)
	if err == nil {
		t.Error("Expected error for non-existent territory")
	}
}

// TestValidateMovementPieceNotInTerritory tests error when piece isn't in source territory
func TestValidateMovementPieceNotInTerritory(t *testing.T) {
	game := createMovementTestGame()
	game.PlacePieces("Moscow", "infantry", 1)

	// Try to move from Karelia (piece is in Moscow)
	err := ValidateMovement(game, 1, "Karelia", "Finland", CombatMove)
	if err == nil {
		t.Error("Expected error when piece not in source territory")
	}
}

// TestValidateMovementTerrainRestrictions tests terrain compatibility
func TestValidateMovementTerrainRestrictions(t *testing.T) {
	game := createMovementTestGame()

	// Place land unit in Moscow
	game.PlacePieces("Moscow", "infantry", 1)
	infantryID := 1

	// Place sea unit in North Sea
	game.PlacePieces("North Sea", "transport", 1)
	transportID := 2

	// Land unit cannot move to water
	game.ConnectTerritories("Karelia", "North Sea")
	err := ValidateMovement(game, infantryID, "Moscow", "North Sea", CombatMove)
	if err == nil {
		t.Error("Expected error: land unit cannot move to water")
	}

	// Sea unit cannot move to land
	err = ValidateMovement(game, transportID, "North Sea", "London", CombatMove)
	if err == nil {
		t.Error("Expected error: sea unit cannot move to land")
	}
}

// TestValidateMovementAirUnits tests that air units can move anywhere
func TestValidateMovementAirUnits(t *testing.T) {
	game := createMovementTestGame()

	// Place fighter in Moscow
	game.PlacePieces("Moscow", "fighter", 1)
	fighterID := 1

	// Connect Moscow to both land and water for direct movement
	game.ConnectTerritories("Moscow", "North Sea")

	// Fighter can move to land
	err := ValidateMovement(game, fighterID, "Moscow", "Karelia", CombatMove)
	if err != nil {
		t.Errorf("Fighter should be able to move to land: %v", err)
	}

	// Fighter can move to water
	err = ValidateMovement(game, fighterID, "Moscow", "North Sea", CombatMove)
	if err != nil {
		t.Errorf("Fighter should be able to move to water: %v", err)
	}
}

// TestValidateMovementNotConnected tests error when territories aren't connected
func TestValidateMovementNotConnected(t *testing.T) {
	game := createMovementTestGame()
	game.PlacePieces("Moscow", "infantry", 1)

	// Moscow and Finland are not directly connected
	err := ValidateMovement(game, 1, "Moscow", "Finland", CombatMove)
	if err == nil {
		t.Error("Expected error: territories not directly connected")
	}
}

// TestValidateMovementZeroMovement tests units that cannot move
func TestValidateMovementZeroMovement(t *testing.T) {
	game := createMovementTestGame()

	// Add factory (movement=0)
	game.AddPieceTemplate("factory", models.Land, 0, 0, 0, 15)
	game.PlacePieces("Moscow", "factory", 1)
	factoryID := 1

	err := ValidateMovement(game, factoryID, "Moscow", "Karelia", CombatMove)
	if err == nil {
		t.Error("Expected error: factory cannot move")
	}
}

// TestCalculateMovementDistance tests path distance calculation
func TestCalculateMovementDistance(t *testing.T) {
	game := createMovementTestGame()

	tests := []struct {
		from     string
		to       string
		expected int
	}{
		{"Moscow", "Karelia", 1},
		{"Moscow", "Finland", 2},
		{"Moscow", "Norway", 3},
		{"Moscow", "North Sea", 4},
		{"Moscow", "London", 5},
		{"Karelia", "Finland", 1},
		{"Finland", "London", 3},
	}

	for _, tt := range tests {
		distance, path, err := CalculateMovementDistance(game, tt.from, tt.to)
		if err != nil {
			t.Errorf("Failed to calculate distance from %s to %s: %v", tt.from, tt.to, err)
			continue
		}

		if distance != tt.expected {
			t.Errorf("Distance from %s to %s: expected %d, got %d", tt.from, tt.to, tt.expected, distance)
		}

		if len(path) != distance+1 {
			t.Errorf("Path length mismatch: distance=%d, path=%v", distance, path)
		}

		if path[0] != tt.from {
			t.Errorf("Path should start at %s, got %s", tt.from, path[0])
		}

		if path[len(path)-1] != tt.to {
			t.Errorf("Path should end at %s, got %s", tt.to, path[len(path)-1])
		}
	}
}

// TestCalculateMovementDistanceNoPath tests error when no path exists
func TestCalculateMovementDistanceNoPath(t *testing.T) {
	game := createMovementTestGame()

	// Add disconnected territory
	game.AddTerritory("Australia", models.Land, "UK", 2)

	_, _, err := CalculateMovementDistance(game, "Moscow", "Australia")
	if err == nil {
		t.Error("Expected error: no path between disconnected territories")
	}
}

// TestCanReachTerritory tests movement range checking
func TestCanReachTerritory(t *testing.T) {
	game := createMovementTestGame()

	// Place units with different movement ranges
	game.PlacePieces("Moscow", "infantry", 1)  // movement=1
	game.PlacePieces("Moscow", "armor", 1)     // movement=2
	game.PlacePieces("Moscow", "fighter", 1)   // movement=4
	game.PlacePieces("Moscow", "bomber", 1)    // movement=6

	infantryID := 1
	armorID := 2
	fighterID := 3
	bomberID := 4

	tests := []struct {
		pieceID  int
		from     string
		to       string
		canReach bool
	}{
		{infantryID, "Moscow", "Karelia", true},   // distance=1, movement=1
		{infantryID, "Moscow", "Finland", false},  // distance=2, movement=1
		{armorID, "Moscow", "Karelia", true},      // distance=1, movement=2
		{armorID, "Moscow", "Finland", true},      // distance=2, movement=2
		{armorID, "Moscow", "Norway", false},      // distance=3, movement=2
		{fighterID, "Moscow", "Norway", true},     // distance=3, movement=4
		{fighterID, "Moscow", "North Sea", true},  // distance=4, movement=4
		{fighterID, "Moscow", "London", false},    // distance=5, movement=4
		{bomberID, "Moscow", "London", true},      // distance=5, movement=6
	}

	for _, tt := range tests {
		canReach, err := CanReachTerritory(game, tt.pieceID, tt.from, tt.to)
		if err != nil {
			t.Errorf("Error checking if piece %d can reach %s: %v", tt.pieceID, tt.to, err)
			continue
		}

		if canReach != tt.canReach {
			piece := game.Pieces[tt.pieceID]
			t.Errorf("Piece %s (movement=%d) reaching %s from %s: expected %v, got %v",
				piece.Name, piece.Movement, tt.to, tt.from, tt.canReach, canReach)
		}
	}
}

// TestGetReachableTerritories tests finding all reachable territories
func TestGetReachableTerritories(t *testing.T) {
	game := createMovementTestGame()

	// Place infantry (movement=1) in Moscow
	game.PlacePieces("Moscow", "infantry", 1)
	infantryID := 1

	reachable, err := GetReachableTerritories(game, infantryID, "Moscow")
	if err != nil {
		t.Fatalf("Failed to get reachable territories: %v", err)
	}

	// Infantry with movement=1 should reach only Karelia (distance=1)
	if len(reachable) != 1 {
		t.Errorf("Infantry should reach 1 territory, got %d", len(reachable))
	}

	if len(reachable) > 0 && reachable[0].Name != "Karelia" {
		t.Errorf("Infantry should reach Karelia, got %s", reachable[0].Name)
	}

	// Place armor (movement=2) in Moscow
	game.PlacePieces("Moscow", "armor", 1)
	armorID := 2

	reachable, err = GetReachableTerritories(game, armorID, "Moscow")
	if err != nil {
		t.Fatalf("Failed to get reachable territories: %v", err)
	}

	// Armor with movement=2 should reach Karelia (1) and Finland (2)
	if len(reachable) != 2 {
		t.Errorf("Armor should reach 2 territories, got %d", len(reachable))
	}

	// Verify the territories are correct
	territoryNames := make(map[string]bool)
	for _, terr := range reachable {
		territoryNames[terr.Name] = true
	}

	if !territoryNames["Karelia"] || !territoryNames["Finland"] {
		t.Error("Armor should reach Karelia and Finland")
	}
}

// TestMovementTracker tests the movement tracking system
func TestMovementTracker(t *testing.T) {
	tracker := NewMovementTracker()

	// Add some moves
	err := tracker.AddMove(1, "Moscow", "Karelia", CombatMove)
	if err != nil {
		t.Errorf("Failed to add move: %v", err)
	}

	err = tracker.AddMove(2, "Berlin", "Poland", CombatMove)
	if err != nil {
		t.Errorf("Failed to add move: %v", err)
	}

	err = tracker.AddMove(3, "London", "Scotland", NoncombatMove)
	if err != nil {
		t.Errorf("Failed to add move: %v", err)
	}

	// Check total moves
	if len(tracker.Moves) != 3 {
		t.Errorf("Expected 3 moves, got %d", len(tracker.Moves))
	}

	// Check moves by type
	combatMoves := tracker.GetMovesByType(CombatMove)
	if len(combatMoves) != 2 {
		t.Errorf("Expected 2 combat moves, got %d", len(combatMoves))
	}

	noncombatMoves := tracker.GetMovesByType(NoncombatMove)
	if len(noncombatMoves) != 1 {
		t.Errorf("Expected 1 noncombat move, got %d", len(noncombatMoves))
	}
}

// TestMovementTrackerDuplicateMove tests preventing duplicate moves
func TestMovementTrackerDuplicateMove(t *testing.T) {
	tracker := NewMovementTracker()

	err := tracker.AddMove(1, "Moscow", "Karelia", CombatMove)
	if err != nil {
		t.Errorf("Failed to add first move: %v", err)
	}

	// Try to move the same piece again
	err = tracker.AddMove(1, "Karelia", "Finland", CombatMove)
	if err == nil {
		t.Error("Expected error when moving same piece twice")
	}
}

// TestMovementTrackerRemoveMove tests canceling moves
func TestMovementTrackerRemoveMove(t *testing.T) {
	tracker := NewMovementTracker()

	tracker.AddMove(1, "Moscow", "Karelia", CombatMove)
	tracker.AddMove(2, "Berlin", "Poland", CombatMove)

	if len(tracker.Moves) != 2 {
		t.Errorf("Expected 2 moves, got %d", len(tracker.Moves))
	}

	// Remove piece 1's move
	err := tracker.RemoveMove(1)
	if err != nil {
		t.Errorf("Failed to remove move: %v", err)
	}

	if len(tracker.Moves) != 1 {
		t.Errorf("Expected 1 move after removal, got %d", len(tracker.Moves))
	}

	// Piece 1 should be able to move again
	err = tracker.AddMove(1, "Moscow", "Karelia", CombatMove)
	if err != nil {
		t.Errorf("Should be able to move piece 1 after removal: %v", err)
	}
}

// TestMovementTrackerClear tests clearing all moves
func TestMovementTrackerClear(t *testing.T) {
	tracker := NewMovementTracker()

	tracker.AddMove(1, "Moscow", "Karelia", CombatMove)
	tracker.AddMove(2, "Berlin", "Poland", CombatMove)
	tracker.AddMove(3, "London", "Scotland", NoncombatMove)

	tracker.Clear()

	if len(tracker.Moves) != 0 {
		t.Errorf("Expected 0 moves after clear, got %d", len(tracker.Moves))
	}

	if len(tracker.PiecesMovedFrom) != 0 {
		t.Errorf("Expected 0 tracked pieces after clear, got %d", len(tracker.PiecesMovedFrom))
	}
}

// TestExecuteMoves tests applying moves to game state
func TestExecuteMoves(t *testing.T) {
	game := createMovementTestGame()
	tracker := NewMovementTracker()

	// Place pieces
	game.PlacePieces("Moscow", "infantry", 2)
	infantryID1 := 1
	infantryID2 := 2

	// Track moves
	tracker.AddMove(infantryID1, "Moscow", "Karelia", CombatMove)
	tracker.AddMove(infantryID2, "Moscow", "Karelia", CombatMove)

	// Verify pieces are in Moscow
	moscowPieces := game.GetPiecesInTerritory("Moscow")
	if len(moscowPieces) != 2 {
		t.Errorf("Expected 2 pieces in Moscow, got %d", len(moscowPieces))
	}

	// Execute moves
	err := tracker.ExecuteMoves(game)
	if err != nil {
		t.Fatalf("Failed to execute moves: %v", err)
	}

	// Verify pieces moved to Karelia
	moscowPieces = game.GetPiecesInTerritory("Moscow")
	kareliaPieces := game.GetPiecesInTerritory("Karelia")

	if len(moscowPieces) != 0 {
		t.Errorf("Expected 0 pieces in Moscow after move, got %d", len(moscowPieces))
	}

	if len(kareliaPieces) != 2 {
		t.Errorf("Expected 2 pieces in Karelia after move, got %d", len(kareliaPieces))
	}
}

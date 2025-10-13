package models

import (
	"testing"
)

// TestPhaseEnum tests the Phase enum and its String() method
func TestPhaseEnum(t *testing.T) {
	tests := []struct {
		phase    Phase
		expected string
	}{
		{PurchasePhase, "Purchase Units"},
		{CombatMovePhase, "Combat Move"},
		{ConductCombatPhase, "Conduct Combat"},
		{NoncombatMovePhase, "Noncombat Move"},
		{MobilizePhase, "Mobilize New Units"},
		{CollectIncomePhase, "Collect Income"},
	}

	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.expected {
			t.Errorf("Phase %d: expected '%s', got '%s'", tt.phase, tt.expected, got)
		}
	}
}

// TestNewGameTurnState verifies NewGame initializes turn state correctly
func TestNewGameTurnState(t *testing.T) {
	game := NewGame()

	if game.CurrentPower != "" {
		t.Errorf("Expected CurrentPower to be empty, got '%s'", game.CurrentPower)
	}

	if game.CurrentPhase != PurchasePhase {
		t.Errorf("Expected CurrentPhase to be PurchasePhase, got %s", game.CurrentPhase)
	}

	if game.PurchasedUnits == nil {
		t.Error("Expected PurchasedUnits map to be initialized")
	}

	if len(game.PurchasedUnits) != 0 {
		t.Errorf("Expected PurchasedUnits to be empty, got %d items", len(game.PurchasedUnits))
	}

	if game.Turn != 1 {
		t.Errorf("Expected Turn to be 1, got %d", game.Turn)
	}
}

// TestPlayerIPCs verifies Player IPC tracking
func TestPlayerIPCs(t *testing.T) {
	game := NewGame()
	player := game.GetOrCreatePlayer("Germany")

	if player.IPCs != 0 {
		t.Errorf("Expected initial IPCs to be 0, got %d", player.IPCs)
	}

	// Test adding IPCs
	player.IPCs = 50
	if player.IPCs != 50 {
		t.Errorf("Expected IPCs to be 50, got %d", player.IPCs)
	}

	// Test spending IPCs
	player.IPCs -= 10
	if player.IPCs != 40 {
		t.Errorf("Expected IPCs to be 40 after spending 10, got %d", player.IPCs)
	}
}

// TestPendingUnits tests the pending unit purchase system
func TestPendingUnits(t *testing.T) {
	game := NewGame()
	playerName := "Germany"

	// Initially should have no purchased units
	if len(game.PurchasedUnits[playerName]) != 0 {
		t.Error("Expected no purchased units initially")
	}

	// Add some pending units
	game.PurchasedUnits[playerName] = []*PendingUnit{
		{Type: "infantry", Cost: 3},
		{Type: "armor", Cost: 5},
		{Type: "fighter", Cost: 10},
	}

	if len(game.PurchasedUnits[playerName]) != 3 {
		t.Errorf("Expected 3 purchased units, got %d", len(game.PurchasedUnits[playerName]))
	}

	// Verify the units
	if game.PurchasedUnits[playerName][0].Type != "infantry" {
		t.Errorf("Expected first unit to be infantry, got %s", game.PurchasedUnits[playerName][0].Type)
	}

	if game.PurchasedUnits[playerName][0].Cost != 3 {
		t.Errorf("Expected infantry cost to be 3, got %d", game.PurchasedUnits[playerName][0].Cost)
	}
}

// TestVictoryCityTracking tests territory victory city marking
func TestVictoryCityTracking(t *testing.T) {
	game := NewGame()

	// Add a territory
	err := game.AddTerritory("Berlin", Land, "Germany", 10)
	if err != nil {
		t.Fatalf("Failed to add territory: %v", err)
	}

	berlin := game.Board["Berlin"]

	// Initially should not be a victory city
	if berlin.IsVictoryCity {
		t.Error("Berlin should not be marked as victory city initially")
	}

	// Mark as victory city
	berlin.IsVictoryCity = true
	if !berlin.IsVictoryCity {
		t.Error("Berlin should be marked as victory city")
	}

	// Count victory cities
	victoryCities := 0
	for _, territory := range game.Board {
		if territory.IsVictoryCity {
			victoryCities++
		}
	}

	if victoryCities != 1 {
		t.Errorf("Expected 1 victory city, got %d", victoryCities)
	}
}

// TestIndustrialComplexDamage tests IC damage tracking
func TestIndustrialComplexDamage(t *testing.T) {
	game := NewGame()

	// Add a territory with production
	err := game.AddTerritory("Germany", Land, "Germany", 10)
	if err != nil {
		t.Fatalf("Failed to add territory: %v", err)
	}

	germany := game.Board["Germany"]

	// Initially should have no damage
	if germany.ICDamage != 0 {
		t.Errorf("Expected initial IC damage to be 0, got %d", germany.ICDamage)
	}

	// Add damage from strategic bombing
	germany.ICDamage = 5

	// Production capacity should be reduced by damage
	effectiveProduction := germany.Production - germany.ICDamage
	if effectiveProduction != 5 {
		t.Errorf("Expected effective production to be 5 (10-5), got %d", effectiveProduction)
	}

	// Repair some damage
	germany.ICDamage -= 2
	if germany.ICDamage != 3 {
		t.Errorf("Expected IC damage to be 3 after repair, got %d", germany.ICDamage)
	}

	// Damage cannot exceed 2x production value (rulebook)
	maxDamage := germany.Production * 2
	if maxDamage != 20 {
		t.Errorf("Expected max damage to be 20, got %d", maxDamage)
	}
}

// TestTurnProgression tests advancing through phases and turns
func TestTurnProgression(t *testing.T) {
	game := NewGame()

	// Add players in turn order
	game.PlayerOrder = []string{"USSR", "Germany", "UK", "Japan", "USA"}
	for _, name := range game.PlayerOrder {
		game.GetOrCreatePlayer(name)
	}

	// Start with first player
	game.CurrentPower = game.PlayerOrder[0]
	if game.CurrentPower != "USSR" {
		t.Errorf("Expected first player to be USSR, got %s", game.CurrentPower)
	}

	// Should start in Purchase phase
	if game.CurrentPhase != PurchasePhase {
		t.Errorf("Expected to start in Purchase phase, got %s", game.CurrentPhase)
	}

	// Simulate advancing through phases
	phases := []Phase{
		PurchasePhase,
		CombatMovePhase,
		ConductCombatPhase,
		NoncombatMovePhase,
		MobilizePhase,
		CollectIncomePhase,
	}

	for i, expectedPhase := range phases {
		game.CurrentPhase = expectedPhase
		if game.CurrentPhase != expectedPhase {
			t.Errorf("Phase %d: expected %s, got %s", i, expectedPhase, game.CurrentPhase)
		}
	}

	// After last phase, advance to next player
	currentPlayerIndex := 0
	for i, name := range game.PlayerOrder {
		if name == game.CurrentPower {
			currentPlayerIndex = i
			break
		}
	}

	nextPlayerIndex := (currentPlayerIndex + 1) % len(game.PlayerOrder)
	game.CurrentPower = game.PlayerOrder[nextPlayerIndex]
	game.CurrentPhase = PurchasePhase

	if game.CurrentPower != "Germany" {
		t.Errorf("Expected next player to be Germany, got %s", game.CurrentPower)
	}

	if game.CurrentPhase != PurchasePhase {
		t.Errorf("Expected to reset to Purchase phase, got %s", game.CurrentPhase)
	}

	// After all players have gone, increment turn
	game.CurrentPower = game.PlayerOrder[len(game.PlayerOrder)-1] // USA
	game.CurrentPhase = CollectIncomePhase

	// Simulate end of USA's turn (last player)
	game.Turn++
	game.CurrentPower = game.PlayerOrder[0] // Back to USSR
	game.CurrentPhase = PurchasePhase

	if game.Turn != 2 {
		t.Errorf("Expected Turn to be 2, got %d", game.Turn)
	}

	if game.CurrentPower != "USSR" {
		t.Errorf("Expected to wrap back to USSR, got %s", game.CurrentPower)
	}
}

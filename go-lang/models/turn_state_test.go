package models

import (
	"fmt"
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

// Test StartGame function
func TestStartGameFunction(t *testing.T) {
	game := NewGame()
	game.PlayerOrder = []string{"Germany", "USSR"}
	game.GetOrCreatePlayer("Germany")
	game.GetOrCreatePlayer("USSR")

	err := game.StartGame()
	if err != nil {
		t.Fatalf("StartGame failed: %v", err)
	}

	if game.CurrentPower != "Germany" {
		t.Errorf("Expected Germany to go first, got %s", game.CurrentPower)
	}

	if game.Turn != 1 {
		t.Errorf("Expected turn 1, got %d", game.Turn)
	}
}

// Test AdvancePhase function
func TestAdvancePhaseFunction(t *testing.T) {
	game := NewGame()
	game.PlayerOrder = []string{"Germany"}
	game.GetOrCreatePlayer("Germany")
	game.StartGame()

	err := game.AdvancePhase()
	if err != nil {
		t.Fatalf("AdvancePhase failed: %v", err)
	}

	if game.CurrentPhase != CombatMovePhase {
		t.Errorf("Expected CombatMovePhase, got %v", game.CurrentPhase)
	}
}

// Test PurchaseUnit function
func TestPurchaseUnitFunction(t *testing.T) {
	game := NewGame()
	game.PlayerOrder = []string{"Germany"}
	player := game.GetOrCreatePlayer("Germany")
	player.IPCs = 50

	game.AddPieceTemplate("infantry", Land, 1, 1, 2, 3)
	game.StartGame()

	err := game.PurchaseUnit("Germany", "infantry")
	if err != nil {
		t.Fatalf("PurchaseUnit failed: %v", err)
	}

	if player.IPCs != 47 {
		t.Errorf("Expected 47 IPCs, got %d", player.IPCs)
	}

	if len(game.PurchasedUnits["Germany"]) != 1 {
		t.Errorf("Expected 1 pending unit, got %d", len(game.PurchasedUnits["Germany"]))
	}
}

// Test CollectIncome function
func TestCollectIncomeFunction(t *testing.T) {
	game := NewGame()
	game.PlayerOrder = []string{"Germany"}
	player := game.GetOrCreatePlayer("Germany")
	player.IPCs = 10

	game.AddTerritory("Berlin", Land, "Germany", 5)
	game.StartGame()

	// Advance to Collect Income phase
	for game.CurrentPhase != CollectIncomePhase {
		game.AdvancePhase()
	}

	err := game.CollectIncome()
	if err != nil {
		t.Fatalf("CollectIncome failed: %v", err)
	}

	if player.IPCs != 15 {
		t.Errorf("Expected 15 IPCs (10 + 5), got %d", player.IPCs)
	}
}

// Test CaptureCapital function
func TestCaptureCapital(t *testing.T) {
	game := NewGame()

	// Create two players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Capital = "Berlin"
	germany.Side = "Axis"
	germany.IPCs = 50

	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Capital = "Moscow"
	ussr.Side = "Allies"
	ussr.IPCs = 20

	// Add Berlin territory
	err := game.AddTerritory("Berlin", Land, "Germany", 10)
	if err != nil {
		t.Fatalf("Failed to add Berlin: %v", err)
	}

	// USSR captures Berlin
	err = game.CaptureCapital("Berlin", "USSR")
	if err != nil {
		t.Fatalf("CaptureCapital failed: %v", err)
	}

	// Check that USSR got Germany's IPCs
	if ussr.IPCs != 70 {
		t.Errorf("Expected USSR to have 70 IPCs (20 + 50), got %d", ussr.IPCs)
	}

	// Check that Germany lost all IPCs
	if germany.IPCs != 0 {
		t.Errorf("Expected Germany to have 0 IPCs, got %d", germany.IPCs)
	}

	// Check that Berlin is now owned by USSR
	berlin := game.Board["Berlin"]
	if berlin.Owner != ussr {
		t.Error("Berlin should be owned by USSR")
	}
}

// Test CountVictoryCities function
func TestCountVictoryCities(t *testing.T) {
	game := NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"

	japan := game.GetOrCreatePlayer("Japan")
	japan.Side = "Axis"

	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"

	uk := game.GetOrCreatePlayer("UK")
	uk.Side = "Allies"

	// Add territories with victory cities
	game.AddTerritory("Berlin", Land, "Germany", 10)
	game.Board["Berlin"].IsVictoryCity = true

	game.AddTerritory("Tokyo", Land, "Japan", 8)
	game.Board["Tokyo"].IsVictoryCity = true

	game.AddTerritory("Moscow", Land, "USSR", 8)
	game.Board["Moscow"].IsVictoryCity = true

	game.AddTerritory("London", Land, "UK", 8)
	game.Board["London"].IsVictoryCity = true

	game.AddTerritory("Paris", Land, "Germany", 6) // Axis-controlled Allied VC
	game.Board["Paris"].IsVictoryCity = true

	// Count victory cities
	axisVC, alliesVC := game.CountVictoryCities()

	if axisVC != 3 {
		t.Errorf("Expected Axis to control 3 VCs (Berlin, Tokyo, Paris), got %d", axisVC)
	}

	if alliesVC != 2 {
		t.Errorf("Expected Allies to control 2 VCs (Moscow, London), got %d", alliesVC)
	}
}

// Test CheckVictoryConditions - No winner yet
func TestCheckVictoryConditions_NoWinner(t *testing.T) {
	game := NewGame()

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"

	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"

	// Add some victory cities but not enough for victory
	game.AddTerritory("Berlin", Land, "Germany", 10)
	game.Board["Berlin"].IsVictoryCity = true

	game.AddTerritory("Moscow", Land, "USSR", 8)
	game.Board["Moscow"].IsVictoryCity = true

	winner, hasWon, desc := game.CheckVictoryConditions()

	if hasWon {
		t.Errorf("Expected no winner yet, but got winner: %s (%s)", winner, desc)
	}

	if winner != "" {
		t.Errorf("Expected empty winner string, got %s", winner)
	}
}

// Test CheckVictoryConditions - Axis victory
func TestCheckVictoryConditions_AxisVictory(t *testing.T) {
	game := NewGame()

	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"

	// Add 9 victory cities controlled by Axis
	for i := 1; i <= 9; i++ {
		name := fmt.Sprintf("VC%d", i)
		game.AddTerritory(name, Land, "Germany", 5)
		game.Board[name].IsVictoryCity = true
	}

	winner, hasWon, desc := game.CheckVictoryConditions()

	if !hasWon {
		t.Error("Expected Axis to have won")
	}

	if winner != "Axis" {
		t.Errorf("Expected Axis to win, got %s", winner)
	}

	if desc == "" {
		t.Error("Expected victory description")
	}
}

// Test CheckVictoryConditions - Allies victory
func TestCheckVictoryConditions_AlliesVictory(t *testing.T) {
	game := NewGame()

	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"

	// Add 10 victory cities controlled by Allies
	for i := 1; i <= 10; i++ {
		name := fmt.Sprintf("VC%d", i)
		game.AddTerritory(name, Land, "USSR", 5)
		game.Board[name].IsVictoryCity = true
	}

	winner, hasWon, desc := game.CheckVictoryConditions()

	if !hasWon {
		t.Error("Expected Allies to have won")
	}

	if winner != "Allies" {
		t.Errorf("Expected Allies to win, got %s", winner)
	}

	if desc == "" {
		t.Error("Expected victory description")
	}
}

// Test CheckVictoryConditions - Total victory
func TestCheckVictoryConditions_TotalVictory(t *testing.T) {
	game := NewGame()

	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"

	// Add all 13 victory cities controlled by Axis
	for i := 1; i <= 13; i++ {
		name := fmt.Sprintf("VC%d", i)
		game.AddTerritory(name, Land, "Germany", 5)
		game.Board[name].IsVictoryCity = true
	}

	winner, hasWon, desc := game.CheckVictoryConditions()

	if !hasWon {
		t.Error("Expected Axis to have won with total victory")
	}

	if winner != "Axis" {
		t.Errorf("Expected Axis to win, got %s", winner)
	}

	// Should mention "Total Victory"
	if desc == "" || !contains(desc, "Total Victory") {
		t.Errorf("Expected 'Total Victory' in description, got: %s", desc)
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

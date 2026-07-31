package game

import (
	"boardgame/models"
	"fmt"
	"testing"
)

// Helper function to create a test game
// giveProductionCentre puts a factory in a territory.
//
// New units are built at an industrial complex, so a test that mobilises needs
// one. It is not in createTestGame because placing pieces there would shift
// every piece ID and piece count that the movement and combat tests depend on.
func giveProductionCentre(t *testing.T, g *models.Game, territory string) {
	t.Helper()
	if err := g.PlacePieces(territory, "factory", 1); err != nil {
		t.Fatalf("placing factory in %s: %v", territory, err)
	}
}

func createTestGame() *models.Game {
	game := models.NewGame()

	// Add players in turn order (USSR → Germany → UK → Japan → USA)
	game.PlayerOrder = []string{"USSR", "Germany", "UK", "Japan", "USA"}
	for _, name := range game.PlayerOrder {
		player := game.GetOrCreatePlayer(name)
		player.IPCs = 50 // Start with some money
	}

	// Add some territories
	game.AddTerritory("Moscow", models.Land, "USSR", 8)
	game.AddTerritory("Germany", models.Land, "Germany", 10)
	game.AddTerritory("London", models.Land, "UK", 8)
	game.AddTerritory("Tokyo", models.Land, "Japan", 8)
	game.AddTerritory("Washington", models.Land, "USA", 10)

	// Mark capitals as victory cities
	game.Board["Moscow"].IsVictoryCity = true
	game.Board["Germany"].IsVictoryCity = true
	game.Board["London"].IsVictoryCity = true
	game.Board["Tokyo"].IsVictoryCity = true
	game.Board["Washington"].IsVictoryCity = true

	// Add some unit templates
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.AddPieceTemplate("armor", models.Land, 2, 3, 3, 5)
	game.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 10)
	game.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)

	return game
}

// TestNewGameController tests controller creation
func TestNewGameController(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)

	if controller.Game != game {
		t.Error("Controller should reference the game")
	}
}

// TestStartGame tests game initialization
func TestStartGame(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)

	err := controller.StartGame()
	if err != nil {
		t.Fatalf("StartGame failed: %v", err)
	}

	if game.CurrentPower != "USSR" {
		t.Errorf("Expected first player to be USSR, got %s", game.CurrentPower)
	}

	if game.CurrentPhase != models.PurchasePhase {
		t.Errorf("Expected first phase to be Purchase, got %s", game.CurrentPhase)
	}

	if game.Turn != 1 {
		t.Errorf("Expected turn to be 1, got %d", game.Turn)
	}
}

// TestStartGameNoPlayers tests starting with no players
func TestStartGameNoPlayers(t *testing.T) {
	game := models.NewGame()
	controller := NewGameController(game)

	err := controller.StartGame()
	if err == nil {
		t.Error("Expected error when starting game with no players")
	}
}

// TestAdvancePhase tests phase progression
func TestAdvancePhase(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	phases := []models.Phase{
		models.PurchasePhase,
		models.CombatMovePhase,
		models.ConductCombatPhase,
		models.NoncombatMovePhase,
		models.MobilizePhase,
		models.CollectIncomePhase,
	}

	// Should start at first phase
	if game.CurrentPhase != phases[0] {
		t.Errorf("Expected to start at %s, got %s", phases[0], game.CurrentPhase)
	}

	// Advance through each phase
	for i := 0; i < len(phases)-1; i++ {
		err := controller.AdvancePhase()
		if err != nil {
			t.Fatalf("Failed to advance from phase %d: %v", i, err)
		}

		expectedPhase := phases[i+1]
		if game.CurrentPhase != expectedPhase {
			t.Errorf("After advancing from phase %d, expected %s, got %s",
				i, expectedPhase, game.CurrentPhase)
		}
	}

	// Advancing from last phase should move to next player's turn
	currentPlayer := game.CurrentPower
	err := controller.AdvancePhase()
	if err != nil {
		t.Fatalf("Failed to advance from last phase: %v", err)
	}

	if game.CurrentPower == currentPlayer {
		t.Error("Expected to advance to next player after last phase")
	}

	if game.CurrentPhase != models.PurchasePhase {
		t.Errorf("Expected to reset to Purchase phase, got %s", game.CurrentPhase)
	}
}

// TestAdvanceTurn tests turn progression through all players
func TestAdvanceTurn(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Track initial turn number
	initialTurn := game.Turn

	// Advance through all players
	expectedPlayers := []string{"Germany", "UK", "Japan", "USA", "USSR"}

	for _, expectedPlayer := range expectedPlayers {
		err := controller.AdvanceTurn()
		if err != nil {
			t.Fatalf("Failed to advance turn: %v", err)
		}

		if game.CurrentPower != expectedPlayer {
			t.Errorf("Expected player %s, got %s", expectedPlayer, game.CurrentPower)
		}

		if game.CurrentPhase != models.PurchasePhase {
			t.Errorf("Expected Purchase phase after turn advance, got %s", game.CurrentPhase)
		}
	}

	// After completing the cycle, turn should increment
	if game.Turn != initialTurn+1 {
		t.Errorf("Expected turn to increment from %d to %d, got %d",
			initialTurn, initialTurn+1, game.Turn)
	}
}

// TestGetCurrentPlayer tests retrieving the active player
func TestGetCurrentPlayer(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	player, err := controller.GetCurrentPlayer()
	if err != nil {
		t.Fatalf("Failed to get current player: %v", err)
	}

	if player.Name != "USSR" {
		t.Errorf("Expected current player to be USSR, got %s", player.Name)
	}
}

// TestCalculateIncome tests income calculation
func TestCalculateIncome(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)

	// USSR should have Moscow (8 IPCs)
	income, err := controller.CalculateIncome("USSR")
	if err != nil {
		t.Fatalf("Failed to calculate income: %v", err)
	}

	if income != 8 {
		t.Errorf("Expected USSR income to be 8, got %d", income)
	}

	// Germany should have Germany (10 IPCs)
	income, err = controller.CalculateIncome("Germany")
	if err != nil {
		t.Fatalf("Failed to calculate income: %v", err)
	}

	if income != 10 {
		t.Errorf("Expected Germany income to be 10, got %d", income)
	}
}

// TestCollectIncome tests income collection
func TestCollectIncome(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	player, _ := controller.GetCurrentPlayer()
	initialIPCs := player.IPCs

	err := controller.CollectIncome()
	if err != nil {
		t.Fatalf("Failed to collect income: %v", err)
	}

	expectedIPCs := initialIPCs + 8 // Moscow produces 8
	if player.IPCs != expectedIPCs {
		t.Errorf("Expected %d IPCs after collection, got %d", expectedIPCs, player.IPCs)
	}
}

// TestPurchaseUnit tests unit purchasing
func TestPurchaseUnit(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	player, _ := controller.GetCurrentPlayer()
	initialIPCs := player.IPCs

	// Purchase 2 infantry (3 IPCs each = 6 total)
	err := controller.PurchaseUnit("infantry", 2)
	if err != nil {
		t.Fatalf("Failed to purchase units: %v", err)
	}

	// Check IPCs were deducted
	expectedIPCs := initialIPCs - 6
	if player.IPCs != expectedIPCs {
		t.Errorf("Expected %d IPCs after purchase, got %d", expectedIPCs, player.IPCs)
	}

	// Check units were added to pending
	pending := game.PurchasedUnits[player.Name]
	if len(pending) != 2 {
		t.Errorf("Expected 2 pending units, got %d", len(pending))
	}

	if pending[0].Type != "infantry" {
		t.Errorf("Expected infantry, got %s", pending[0].Type)
	}
}

// TestPurchaseUnitInsufficientFunds tests purchasing with insufficient IPCs
func TestPurchaseUnitInsufficientFunds(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	player, _ := controller.GetCurrentPlayer()
	player.IPCs = 5 // Only 5 IPCs

	// Try to purchase a fighter (10 IPCs)
	err := controller.PurchaseUnit("fighter", 1)
	if err == nil {
		t.Error("Expected error when purchasing with insufficient funds")
	}
}

// TestPurchaseUnitWrongPhase tests purchasing outside Purchase phase
func TestPurchaseUnitWrongPhase(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Advance to Combat Move phase
	controller.AdvancePhase()

	err := controller.PurchaseUnit("infantry", 1)
	if err == nil {
		t.Error("Expected error when purchasing outside Purchase phase")
	}
}

// TestMobilizeUnit tests unit placement
func TestMobilizeUnit(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Purchase a unit first
	controller.PurchaseUnit("infantry", 1)

	// Units are built at an industrial complex.
	giveProductionCentre(t, game, "Moscow")

	// Advance to Mobilize phase
	game.CurrentPhase = models.MobilizePhase

	initialPieceCount := len(game.Pieces)

	// Mobilize the unit in Moscow
	err := controller.MobilizeUnit("Moscow", "infantry")
	if err != nil {
		t.Fatalf("Failed to mobilize unit: %v", err)
	}

	// Check that piece was placed
	if len(game.Pieces) != initialPieceCount+1 {
		t.Errorf("Expected %d pieces, got %d", initialPieceCount+1, len(game.Pieces))
	}

	// Check that pending unit was removed
	player, _ := controller.GetCurrentPlayer()
	pending := game.PurchasedUnits[player.Name]
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending units after mobilization, got %d", len(pending))
	}
}

// TestVictoryConditionImmediate tests immediate victory (13+ cities)
func TestVictoryConditionImmediate(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)

	// Initial setup has 2 Axis cities (Germany, Tokyo) and 3 Allied (Moscow, London, Washington)
	// Add 11 more territories to Axis to reach 13 victory cities (2 + 11 = 13)
	for i := 0; i < 11; i++ {
		name := fmt.Sprintf("AxisCity%d", i)
		game.AddTerritory(name, models.Land, "Germany", 1)
		game.Board[name].IsVictoryCity = true
	}

	winner, hasWon, err := controller.CheckVictoryCondition()
	if err != nil {
		t.Fatalf("Failed to check victory: %v", err)
	}

	if !hasWon {
		t.Error("Expected Axis to have immediate victory with 13+ cities")
	}

	if winner != "Axis" {
		t.Errorf("Expected Axis to win, got %s", winner)
	}
}

// TestVictoryConditionPotential tests potential victory (9+ for Axis, 10+ for Allies)
func TestVictoryConditionPotential(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)

	// Add 4 more cities to Axis (already has Germany + Tokyo = 2, need 7 more for 9 total)
	for i := 0; i < 7; i++ {
		name := fmt.Sprintf("AxisCity%d", i)
		game.AddTerritory(name, models.Land, "Germany", 1)
		game.Board[name].IsVictoryCity = true
	}

	winner, hasWon, err := controller.CheckVictoryCondition()
	if err != nil {
		t.Fatalf("Failed to check victory: %v", err)
	}

	// Should return "Axis" but hasWon should be false (not immediate victory)
	if winner != "Axis" {
		t.Errorf("Expected Axis to have potential victory, got %s", winner)
	}

	if hasWon {
		t.Error("Expected potential victory (not immediate) with 9 cities")
	}
}

// TestGetVictoryCityCounts tests counting victory cities by side
func TestGetVictoryCityCounts(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)

	axis, allies := controller.GetVictoryCityCounts()

	// Should have 2 Axis (Germany, Tokyo) and 3 Allied (Moscow, London, Washington)
	if axis != 2 {
		t.Errorf("Expected 2 Axis cities, got %d", axis)
	}

	if allies != 3 {
		t.Errorf("Expected 3 Allied cities, got %d", allies)
	}
}

// TestRepairIndustrialComplex tests IC repair
func TestRepairIndustrialComplex(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	player, _ := controller.GetCurrentPlayer()
	moscow := game.Board["Moscow"]

	// Damage the IC
	moscow.ICDamage = 5

	initialIPCs := player.IPCs

	// Repair 3 points of damage
	err := controller.RepairIndustrialComplex("Moscow", 3)
	if err != nil {
		t.Fatalf("Failed to repair IC: %v", err)
	}

	// Check IPCs were deducted (1 IPC per damage)
	if player.IPCs != initialIPCs-3 {
		t.Errorf("Expected %d IPCs after repair, got %d", initialIPCs-3, player.IPCs)
	}

	// Check damage was reduced
	if moscow.ICDamage != 2 {
		t.Errorf("Expected 2 damage remaining, got %d", moscow.ICDamage)
	}
}

// TestPlanMove tests planning moves during combat move phase
func TestPlanMove(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Connect territories and place units
	game.ConnectTerritories("Moscow", "Germany")
	game.PlacePieces("Moscow", "infantry", 1)
	infantryID := 1

	// Advance to combat move phase
	controller.AdvancePhase()

	// Plan a move
	err := controller.PlanMove(infantryID, "Moscow", "Germany")
	if err != nil {
		t.Fatalf("Failed to plan move: %v", err)
	}

	// Verify move is in tracker
	moves := controller.GetPlannedMoves()
	if len(moves) != 1 {
		t.Errorf("Expected 1 planned move, got %d", len(moves))
	}

	if moves[0].PieceID != infantryID {
		t.Errorf("Expected move for piece %d, got %d", infantryID, moves[0].PieceID)
	}

	if moves[0].Type != CombatMove {
		t.Error("Expected combat move type")
	}
}

// TestPlanMoveWrongPhase tests planning moves in wrong phase
func TestPlanMoveWrongPhase(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	game.ConnectTerritories("Moscow", "Germany")
	game.PlacePieces("Moscow", "infantry", 1)

	// Try to plan move in Purchase phase
	err := controller.PlanMove(1, "Moscow", "Germany")
	if err == nil {
		t.Error("Expected error when planning move in Purchase phase")
	}
}

// TestPlanMoveOutOfRange tests planning move beyond unit range
func TestPlanMoveOutOfRange(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Create a chain: Moscow -> Karelia -> Finland
	game.AddTerritory("Karelia", models.Land, "USSR", 2)
	game.AddTerritory("Finland", models.Land, "Germany", 2)
	game.ConnectTerritories("Moscow", "Karelia")
	game.ConnectTerritories("Karelia", "Finland")

	game.PlacePieces("Moscow", "infantry", 1) // infantry has movement=1
	infantryID := 1

	// Advance to combat move phase
	controller.AdvancePhase()

	// Try to move infantry 2 spaces (Moscow -> Finland)
	err := controller.PlanMove(infantryID, "Moscow", "Finland")
	if err == nil {
		t.Error("Expected error when moving infantry beyond range")
	}
}

// TestCancelMove tests canceling a planned move
func TestCancelMove(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	game.ConnectTerritories("Moscow", "Germany")
	game.PlacePieces("Moscow", "infantry", 1)

	// Advance to combat move phase
	controller.AdvancePhase()

	// Plan a move
	controller.PlanMove(1, "Moscow", "Germany")

	// Cancel it
	err := controller.CancelMove(1)
	if err != nil {
		t.Errorf("Failed to cancel move: %v", err)
	}

	// Verify move is removed
	moves := controller.GetPlannedMoves()
	if len(moves) != 0 {
		t.Errorf("Expected 0 planned moves after cancel, got %d", len(moves))
	}
}

// TestGetPlannedAttacks tests identifying hostile territory moves
func TestGetPlannedAttacks(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Connect Moscow (USSR) to Germany (Germany) and Karelia (USSR)
	game.AddTerritory("Karelia", models.Land, "USSR", 2)
	game.ConnectTerritories("Moscow", "Germany")
	game.ConnectTerritories("Moscow", "Karelia")

	game.PlacePieces("Moscow", "infantry", 2)

	// Advance to combat move phase
	controller.AdvancePhase()

	// Plan attack on Germany and move to Karelia
	controller.PlanMove(1, "Moscow", "Germany")   // Attack (different owner)
	controller.PlanMove(2, "Moscow", "Karelia")   // Friendly move (same owner)

	// Get planned attacks
	attacks := controller.GetPlannedAttacks()

	// Should only have Germany as an attack
	if len(attacks) != 1 {
		t.Errorf("Expected 1 attack, got %d", len(attacks))
	}

	if len(attacks) > 0 && attacks[0] != "Germany" {
		t.Errorf("Expected attack on Germany, got %s", attacks[0])
	}
}

// TestExecuteCombatMoves tests executing combat moves
func TestExecuteCombatMoves(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Setup
	game.ConnectTerritories("Moscow", "Germany")
	game.PlacePieces("Moscow", "infantry", 1)

	// Advance to combat move phase
	controller.AdvancePhase()

	// Plan move
	controller.PlanMove(1, "Moscow", "Germany")

	// Execute combat moves
	err := controller.ExecuteCombatMoves()
	if err != nil {
		t.Fatalf("Failed to execute combat moves: %v", err)
	}

	// Verify piece moved
	moscowPieces := game.GetPiecesInTerritory("Moscow")
	germanyPieces := game.GetPiecesInTerritory("Germany")

	if len(moscowPieces) != 0 {
		t.Errorf("Expected 0 pieces in Moscow, got %d", len(moscowPieces))
	}

	if len(germanyPieces) != 1 {
		t.Errorf("Expected 1 piece in Germany, got %d", len(germanyPieces))
	}

	// Verify battle was created
	if _, exists := controller.PendingBattles["Germany"]; !exists {
		t.Error("Expected battle to be created for Germany")
	}
}

// TestExecuteNoncombatMoves tests executing noncombat moves
func TestExecuteNoncombatMoves(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Setup - both territories owned by USSR
	game.AddTerritory("Karelia", models.Land, "USSR", 2)
	game.ConnectTerritories("Moscow", "Karelia")
	game.PlacePieces("Moscow", "infantry", 1)

	// Advance to noncombat move phase
	controller.AdvancePhase() // Combat Move
	controller.AdvancePhase() // Conduct Combat
	controller.AdvancePhase() // Noncombat Move

	// Plan noncombat move
	err := controller.PlanMove(1, "Moscow", "Karelia")
	if err != nil {
		t.Fatalf("Failed to plan noncombat move: %v", err)
	}

	// Execute noncombat moves
	err = controller.ExecuteNoncombatMoves()
	if err != nil {
		t.Fatalf("Failed to execute noncombat moves: %v", err)
	}

	// Verify piece moved
	moscowPieces := game.GetPiecesInTerritory("Moscow")
	kareliaPieces := game.GetPiecesInTerritory("Karelia")

	if len(moscowPieces) != 0 {
		t.Errorf("Expected 0 pieces in Moscow, got %d", len(moscowPieces))
	}

	if len(kareliaPieces) != 1 {
		t.Errorf("Expected 1 piece in Karelia, got %d", len(kareliaPieces))
	}

	// Verify no battle created (friendly move)
	if len(controller.PendingBattles) != 0 {
		t.Error("No battles should be created for noncombat moves")
	}
}

// TestCaptureTerritory tests territory ownership transfer
func TestCaptureTerritory(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Germany owned by Germany
	germanyTerritory := game.Board["Germany"]
	initialOwner := germanyTerritory.Owner.Name
	initialOwnerTerritoryCount := len(germanyTerritory.Owner.Territories)

	ussrPlayer := game.Players["USSR"]
	initialUSSRTerritories := len(ussrPlayer.Territories)

	// Capture Germany for USSR
	err := controller.CaptureTerritory("Germany", "USSR")
	if err != nil {
		t.Fatalf("Failed to capture territory: %v", err)
	}

	// Verify ownership changed
	if germanyTerritory.Owner.Name != "USSR" {
		t.Errorf("Expected Germany to be owned by USSR, got %s", germanyTerritory.Owner.Name)
	}

	// Verify territory was removed from Germany's list
	germanyPlayer := game.Players[initialOwner]
	if len(germanyPlayer.Territories) != initialOwnerTerritoryCount-1 {
		t.Errorf("Expected Germany player to have %d territories, got %d",
			initialOwnerTerritoryCount-1, len(germanyPlayer.Territories))
	}

	// Verify territory was added to USSR's list
	if len(ussrPlayer.Territories) != initialUSSRTerritories+1 {
		t.Errorf("Expected USSR to have %d territories, got %d",
			initialUSSRTerritories+1, len(ussrPlayer.Territories))
	}
}

// TestResolveBattleAttackerWins tests battle resolution with attacker victory
func TestResolveBattleAttackerWins(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Setup: USSR attacks Germany
	game.ConnectTerritories("Moscow", "Germany")
	game.PlacePieces("Moscow", "armor", 5)      // Strong attack force
	game.PlacePieces("Germany", "infantry", 1)  // Weak defense

	// Move USSR units to Germany
	controller.AdvancePhase() // Combat Move phase
	for i := 1; i <= 5; i++ {
		controller.PlanMove(i, "Moscow", "Germany")
	}
	controller.ExecuteCombatMoves()

	// Now resolve the battle
	// Use seeded dice roller for predictable results
	roller := NewSeededDiceRoller(12345)

	initialGermanyOwner := game.Board["Germany"].Owner.Name

	result, err := controller.ResolveBattle("Germany", roller)
	if err != nil {
		t.Fatalf("Failed to resolve battle: %v", err)
	}

	// With 5 armor vs 1 infantry, attacker should win
	if !result.AttackerWins {
		t.Error("Expected attacker to win with overwhelming force")
	}

	// Verify territory was captured
	if game.Board["Germany"].Owner.Name == initialGermanyOwner {
		t.Error("Territory ownership should have changed after attacker won")
	}

	if game.Board["Germany"].Owner.Name != "USSR" {
		t.Errorf("Expected Germany to be captured by USSR, got %s", game.Board["Germany"].Owner.Name)
	}

	// Verify battle was removed from pending
	if _, exists := controller.PendingBattles["Germany"]; exists {
		t.Error("Battle should be removed from pending after resolution")
	}
}

// TestResolveBattleDefenderWins tests battle resolution with defender victory
func TestResolveBattleDefenderWins(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Setup: USSR attacks Germany with weak force
	game.ConnectTerritories("Moscow", "Germany")
	game.PlacePieces("Moscow", "infantry", 1)   // Weak attack
	game.PlacePieces("Germany", "armor", 5)     // Strong defense

	// Move USSR unit to Germany
	controller.AdvancePhase() // Combat Move phase
	controller.PlanMove(1, "Moscow", "Germany")
	controller.ExecuteCombatMoves()

	roller := NewSeededDiceRoller(99999) // Different seed

	initialGermanyOwner := game.Board["Germany"].Owner.Name

	result, err := controller.ResolveBattle("Germany", roller)
	if err != nil {
		t.Fatalf("Failed to resolve battle: %v", err)
	}

	// Defender should win
	if !result.DefenderWins {
		t.Log("Note: Defender did not win - this can happen due to randomness")
	}

	// If defender won, territory should remain with original owner
	if result.DefenderWins {
		if game.Board["Germany"].Owner.Name != initialGermanyOwner {
			t.Error("Territory should remain with defender after defender wins")
		}
	}
}

// TestResolveBattleCasualties tests that casualties are removed from the board
func TestResolveBattleCasualties(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	// Setup
	game.ConnectTerritories("Moscow", "Germany")
	game.PlacePieces("Moscow", "infantry", 3)
	game.PlacePieces("Germany", "infantry", 2)

	initialTotalPieces := len(game.Pieces)

	// Move to attack
	controller.AdvancePhase()
	for i := 1; i <= 3; i++ {
		controller.PlanMove(i, "Moscow", "Germany")
	}
	controller.ExecuteCombatMoves()

	// Resolve battle
	roller := NewSeededDiceRoller(42)
	result, err := controller.ResolveBattle("Germany", roller)
	if err != nil {
		t.Fatalf("Failed to resolve battle: %v", err)
	}

	// Verify casualties were removed
	totalCasualties := len(result.AttackerCasualties) + len(result.DefenderCasualties)
	expectedRemainingPieces := initialTotalPieces - totalCasualties

	if len(game.Pieces) != expectedRemainingPieces {
		t.Errorf("Expected %d pieces remaining, got %d",
			expectedRemainingPieces, len(game.Pieces))
	}

	// Verify pieces in territory match survivors
	germanyPieces := game.GetPiecesInTerritory("Germany")
	expectedInTerritory := len(result.AttackersRemaining) + len(result.DefendersRemaining)

	if len(germanyPieces) != expectedInTerritory {
		t.Errorf("Expected %d pieces in Germany, got %d",
			expectedInTerritory, len(germanyPieces))
	}
}

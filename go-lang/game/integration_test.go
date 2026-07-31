package game

import (
	"boardgame/models"
	"fmt"
	"testing"
)

// TestFullTurnCycle tests a complete turn through all 6 phases
func TestFullTurnCycle(t *testing.T) {
	// Setup game
	game := createTestGame()
	controller := NewGameController(game)

	err := controller.StartGame()
	if err != nil {
		t.Fatalf("Failed to start game: %v", err)
	}

	player, _ := controller.GetCurrentPlayer()
	initialIPCs := player.IPCs

	// Verify starting state
	if game.CurrentPower != "USSR" {
		t.Errorf("Expected first player USSR, got %s", game.CurrentPower)
	}

	if game.CurrentPhase != models.PurchasePhase {
		t.Errorf("Expected Purchase phase, got %s", game.CurrentPhase)
	}

	// Phase 1: Purchase
	t.Run("PurchasePhase", func(t *testing.T) {
		// Purchase some units
		err := controller.PurchaseUnit("infantry", 3) // 3 * 3 = 9 IPCs
		if err != nil {
			t.Fatalf("Failed to purchase units: %v", err)
		}

		expectedIPCs := initialIPCs - 9
		if player.IPCs != expectedIPCs {
			t.Errorf("Expected %d IPCs, got %d", expectedIPCs, player.IPCs)
		}

		// Verify units are in purchased list
		purchased := game.PurchasedUnits[player.Name]
		if len(purchased) != 3 {
			t.Errorf("Expected 3 purchased units, got %d", len(purchased))
		}

		// Advance to next phase
		err = controller.AdvancePhase()
		if err != nil {
			t.Fatalf("Failed to advance phase: %v", err)
		}

		if game.CurrentPhase != models.CombatMovePhase {
			t.Errorf("Expected Combat Move phase, got %s", game.CurrentPhase)
		}
	})

	// Phase 2: Combat Move
	t.Run("CombatMovePhase", func(t *testing.T) {
		// Skip combat move for now (no movement implemented yet)
		err := controller.AdvancePhase()
		if err != nil {
			t.Fatalf("Failed to advance phase: %v", err)
		}

		if game.CurrentPhase != models.ConductCombatPhase {
			t.Errorf("Expected Conduct Combat phase, got %s", game.CurrentPhase)
		}
	})

	// Phase 3: Conduct Combat
	t.Run("ConductCombatPhase", func(t *testing.T) {
		// Skip combat for now (no battles set up)
		err := controller.AdvancePhase()
		if err != nil {
			t.Fatalf("Failed to advance phase: %v", err)
		}

		if game.CurrentPhase != models.NoncombatMovePhase {
			t.Errorf("Expected Noncombat Move phase, got %s", game.CurrentPhase)
		}
	})

	// Phase 4: Noncombat Move
	t.Run("NoncombatMovePhase", func(t *testing.T) {
		// Skip noncombat move for now
		err := controller.AdvancePhase()
		if err != nil {
			t.Fatalf("Failed to advance phase: %v", err)
		}

		if game.CurrentPhase != models.MobilizePhase {
			t.Errorf("Expected Mobilize phase, got %s", game.CurrentPhase)
		}
	})

	// Phase 5: Mobilize
	t.Run("MobilizePhase", func(t *testing.T) {
		// Units are built at an industrial complex.
		giveProductionCentre(t, game, "Moscow")
		initialPieceCount := len(game.Pieces)

		// Place the purchased units in Moscow
		for i := 0; i < 3; i++ {
			err := controller.MobilizeUnit("Moscow", "infantry")
			if err != nil {
				t.Fatalf("Failed to mobilize unit %d: %v", i+1, err)
			}
		}

		// Verify pieces were placed
		expectedPieceCount := initialPieceCount + 3
		if len(game.Pieces) != expectedPieceCount {
			t.Errorf("Expected %d pieces, got %d", expectedPieceCount, len(game.Pieces))
		}

		// Verify purchased units list is empty
		purchased := game.PurchasedUnits[player.Name]
		if len(purchased) != 0 {
			t.Errorf("Expected 0 purchased units remaining, got %d", len(purchased))
		}

		// Advance to next phase
		err := controller.AdvancePhase()
		if err != nil {
			t.Fatalf("Failed to advance phase: %v", err)
		}

		if game.CurrentPhase != models.CollectIncomePhase {
			t.Errorf("Expected Collect Income phase, got %s", game.CurrentPhase)
		}
	})

	// Phase 6: Collect Income
	t.Run("CollectIncomePhase", func(t *testing.T) {
		ipcsBeforeCollection := player.IPCs

		// Collect income
		err := controller.CollectIncome()
		if err != nil {
			t.Fatalf("Failed to collect income: %v", err)
		}

		// USSR should get 8 IPCs from Moscow
		expectedIPCs := ipcsBeforeCollection + 8
		if player.IPCs != expectedIPCs {
			t.Errorf("Expected %d IPCs after collection, got %d", expectedIPCs, player.IPCs)
		}

		// Advancing from Collect Income should move to next player
		currentTurn := game.Turn
		err = controller.AdvancePhase()
		if err != nil {
			t.Fatalf("Failed to advance from collect income: %v", err)
		}

		if game.CurrentPower == "USSR" {
			t.Error("Expected to advance to next player after collect income")
		}

		if game.CurrentPower != "Germany" {
			t.Errorf("Expected next player to be Germany, got %s", game.CurrentPower)
		}

		if game.CurrentPhase != models.PurchasePhase {
			t.Errorf("Expected to reset to Purchase phase, got %s", game.CurrentPhase)
		}

		if game.Turn != currentTurn {
			t.Errorf("Turn should not increment yet, got turn %d", game.Turn)
		}
	})
}

// TestMultipleTurnsFullCycle tests multiple players going through full turns
func TestMultipleTurnsFullCycle(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	initialTurn := game.Turn

	// Each player takes a full turn
	expectedPlayers := []string{"USSR", "Germany", "UK", "Japan", "USA"}

	for i, expectedPlayer := range expectedPlayers {
		if game.CurrentPower != expectedPlayer {
			t.Errorf("Round 1, Player %d: expected %s, got %s", i, expectedPlayer, game.CurrentPower)
		}

		// Go through all 6 phases
		for phase := 0; phase < 6; phase++ {
			err := controller.AdvancePhase()
			if err != nil {
				t.Fatalf("Failed to advance phase for %s: %v", expectedPlayer, err)
			}
		}
	}

	// After all players have gone, turn should increment
	if game.Turn != initialTurn+1 {
		t.Errorf("Expected turn to increment to %d, got %d", initialTurn+1, game.Turn)
	}

	// Should be back to first player
	if game.CurrentPower != "USSR" {
		t.Errorf("Expected to wrap back to USSR, got %s", game.CurrentPower)
	}
}

// TestPurchaseAndMobilizeIntegration tests the full purchase->mobilize cycle
func TestPurchaseAndMobilizeIntegration(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	player, _ := controller.GetCurrentPlayer()

	// Purchase multiple unit types
	t.Run("PurchaseDifferentUnits", func(t *testing.T) {
		err := controller.PurchaseUnit("infantry", 2) // 6 IPCs
		if err != nil {
			t.Fatalf("Failed to purchase infantry: %v", err)
		}

		err = controller.PurchaseUnit("armor", 1) // 5 IPCs
		if err != nil {
			t.Fatalf("Failed to purchase armor: %v", err)
		}

		err = controller.PurchaseUnit("fighter", 1) // 10 IPCs
		if err != nil {
			t.Fatalf("Failed to purchase fighter: %v", err)
		}

		// Total: 21 IPCs spent
		expectedIPCs := 50 - 21 // Started with 50
		if player.IPCs != expectedIPCs {
			t.Errorf("Expected %d IPCs, got %d", expectedIPCs, player.IPCs)
		}

		// Should have 4 units purchased
		purchased := game.PurchasedUnits[player.Name]
		if len(purchased) != 4 {
			t.Errorf("Expected 4 purchased units, got %d", len(purchased))
		}
	})

	// Advance to Mobilize phase
	for i := 0; i < 4; i++ {
		controller.AdvancePhase()
	}

	if game.CurrentPhase != models.MobilizePhase {
		t.Fatalf("Expected Mobilize phase, got %s", game.CurrentPhase)
	}

	t.Run("MobilizeDifferentUnits", func(t *testing.T) {
		// Units are built at an industrial complex.
		giveProductionCentre(t, game, "Moscow")

		// Place infantry
		err := controller.MobilizeUnit("Moscow", "infantry")
		if err != nil {
			t.Fatalf("Failed to mobilize first infantry: %v", err)
		}

		err = controller.MobilizeUnit("Moscow", "infantry")
		if err != nil {
			t.Fatalf("Failed to mobilize second infantry: %v", err)
		}

		// Place armor
		err = controller.MobilizeUnit("Moscow", "armor")
		if err != nil {
			t.Fatalf("Failed to mobilize armor: %v", err)
		}

		// Place fighter
		err = controller.MobilizeUnit("Moscow", "fighter")
		if err != nil {
			t.Fatalf("Failed to mobilize fighter: %v", err)
		}

		// All units should be placed
		purchased := game.PurchasedUnits[player.Name]
		if len(purchased) != 0 {
			t.Errorf("Expected all units to be placed, %d remaining", len(purchased))
		}

		// Verify units are in Moscow
		pieces := game.GetPiecesInTerritory("Moscow")
		foundInfantry := 0
		foundArmor := 0
		foundFighter := 0

		for _, piece := range pieces {
			switch piece.Name {
			case "infantry":
				foundInfantry++
			case "armor":
				foundArmor++
			case "fighter":
				foundFighter++
			}
		}

		// Should have 2 infantry, 1 armor, 1 fighter (plus any that were there initially)
		if foundInfantry < 2 {
			t.Errorf("Expected at least 2 infantry in Moscow, found %d", foundInfantry)
		}
		if foundArmor < 1 {
			t.Errorf("Expected at least 1 armor in Moscow, found %d", foundArmor)
		}
		if foundFighter < 1 {
			t.Errorf("Expected at least 1 fighter in Moscow, found %d", foundFighter)
		}
	})
}

// TestIncomeCalculationIntegration tests income calculation and collection
func TestIncomeCalculationIntegration(t *testing.T) {
	game := models.NewGame()
	game.PlayerOrder = []string{"USSR"}
	player := game.GetOrCreatePlayer("USSR")
	player.IPCs = 10

	// Add multiple territories with different production values
	game.AddTerritory("Moscow", models.Land, "USSR", 8)
	game.AddTerritory("Stalingrad", models.Land, "USSR", 3)
	game.AddTerritory("Leningrad", models.Land, "USSR", 2)

	controller := NewGameController(game)
	controller.StartGame()

	// Calculate income
	income, err := controller.CalculateIncome("USSR")
	if err != nil {
		t.Fatalf("Failed to calculate income: %v", err)
	}

	expectedIncome := 8 + 3 + 2
	if income != expectedIncome {
		t.Errorf("Expected income %d, got %d", expectedIncome, income)
	}

	// Advance to collect income phase
	for i := 0; i < 5; i++ {
		controller.AdvancePhase()
	}

	// Collect income
	initialIPCs := player.IPCs
	err = controller.CollectIncome()
	if err != nil {
		t.Fatalf("Failed to collect income: %v", err)
	}

	expectedIPCs := initialIPCs + expectedIncome
	if player.IPCs != expectedIPCs {
		t.Errorf("Expected %d IPCs after collection, got %d", expectedIPCs, player.IPCs)
	}
}

// TestVictoryConditionIntegration tests victory checking during gameplay
func TestVictoryConditionIntegration(t *testing.T) {
	game := models.NewGame()
	game.PlayerOrder = []string{"Germany", "USSR"}

	game.GetOrCreatePlayer("Germany")
	game.GetOrCreatePlayer("USSR")

	// Give Germany 13 victory cities for immediate victory
	for i := 0; i < 13; i++ {
		game.AddTerritory(fmt.Sprintf("City%d", i), models.Land, "Germany", 1)
		game.Board[fmt.Sprintf("City%d", i)].IsVictoryCity = true
	}

	// Give USSR 1 victory city
	game.AddTerritory("Moscow", models.Land, "USSR", 8)
	game.Board["Moscow"].IsVictoryCity = true

	controller := NewGameController(game)

	// Check victory condition
	winner, hasWon, err := controller.CheckVictoryCondition()
	if err != nil {
		t.Fatalf("Failed to check victory: %v", err)
	}

	if !hasWon {
		t.Error("Expected Germany to have won with 13 cities")
	}

	if winner != "Axis" {
		t.Errorf("Expected Axis to win, got %s", winner)
	}

	// Verify count
	axis, allies := controller.GetVictoryCityCounts()
	if axis != 13 {
		t.Errorf("Expected 13 Axis cities, got %d", axis)
	}
	if allies != 1 {
		t.Errorf("Expected 1 Allied city, got %d", allies)
	}
}

// TestCombatIntegration tests combat with game controller
func TestCombatIntegration(t *testing.T) {
	game := createTestGame()

	// Add piece templates if not already added
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.AddPieceTemplate("armor", models.Land, 2, 3, 3, 5)

	// Place pieces in territories
	game.PlacePieces("Moscow", "infantry", 3)
	game.PlacePieces("Germany", "armor", 2)

	// Create a battle
	roller := NewSeededDiceRoller(42)
	battle := NewBattle("France", LandBattle, "Germany", "USSR")

	// Add pieces to battle
	germanyPieces := game.GetPiecesInTerritory("Germany")
	moscowPieces := game.GetPiecesInTerritory("Moscow")

	for _, piece := range germanyPieces[:2] { // 2 armor
		battle.AddAttacker(piece)
	}

	for _, piece := range moscowPieces[:2] { // 2 infantry
		battle.AddDefender(piece)
	}

	// Resolve combat
	result, err := ResolveCombat(battle, roller, 10)
	if err != nil {
		t.Fatalf("Combat failed: %v", err)
	}

	// Verify result structure
	if !result.AttackerWins && !result.DefenderWins {
		t.Error("Expected one side to win")
	}

	// Total casualties + remaining should equal starting forces
	totalAttackers := len(result.AttackersRemaining) + len(result.AttackerCasualties)
	totalDefenders := len(result.DefendersRemaining) + len(result.DefenderCasualties)

	if totalAttackers != 2 {
		t.Errorf("Expected 2 total attackers, got %d", totalAttackers)
	}

	if totalDefenders != 2 {
		t.Errorf("Expected 2 total defenders, got %d", totalDefenders)
	}
}

// TestIndustrialComplexDamageIntegration tests IC damage affecting production
func TestIndustrialComplexDamageIntegration(t *testing.T) {
	game := createTestGame()
	controller := NewGameController(game)
	controller.StartGame()

	player, _ := controller.GetCurrentPlayer()
	moscow := game.Board["Moscow"]

	// Damage the IC with strategic bombing (simulated)
	moscow.ICDamage = 5

	// Production capacity is now reduced
	effectiveProduction := moscow.Production - moscow.ICDamage
	if effectiveProduction != 3 { // 8 - 5 = 3
		t.Errorf("Expected effective production 3, got %d", effectiveProduction)
	}

	// Repair 3 points of damage
	initialIPCs := player.IPCs
	err := controller.RepairIndustrialComplex("Moscow", 3)
	if err != nil {
		t.Fatalf("Failed to repair IC: %v", err)
	}

	// Should cost 3 IPCs
	if player.IPCs != initialIPCs-3 {
		t.Errorf("Expected %d IPCs, got %d", initialIPCs-3, player.IPCs)
	}

	// Damage should be reduced
	if moscow.ICDamage != 2 {
		t.Errorf("Expected 2 damage remaining, got %d", moscow.ICDamage)
	}
}

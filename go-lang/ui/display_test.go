package ui

import (
	"boardgame/game"
	"boardgame/models"
	"testing"
)

// TestDisplayFunctions tests that display functions don't panic
func TestDisplayFunctions(t *testing.T) {
	// Create a test game
	g := models.NewGame()
	g.PlayerOrder = []string{"USSR", "Germany"}

	ussr := g.GetOrCreatePlayer("USSR")
	ussr.IPCs = 24
	ussr.NPC = false

	germany := g.GetOrCreatePlayer("Germany")
	germany.IPCs = 40
	germany.NPC = true

	// Add territories
	g.AddTerritory("Moscow", models.Land, "USSR", 8)
	g.AddTerritory("Germany", models.Land, "Germany", 10)
	g.Board["Moscow"].IsVictoryCity = true
	g.Board["Germany"].IsVictoryCity = true

	// Add unit templates
	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("armor", models.Land, 2, 3, 3, 5)

	// Place some pieces
	g.PlacePieces("Moscow", "infantry", 5)
	g.PlacePieces("Germany", "armor", 3)

	// Connect territories
	g.ConnectTerritories("Moscow", "Germany")

	controller := game.NewGameController(g)
	controller.StartGame()

	// Test all display functions (they should not panic)
	t.Run("DisplayGameStatus", func(t *testing.T) {
		DisplayGameStatus(g)
	})

	t.Run("DisplayPlayerStatus", func(t *testing.T) {
		DisplayPlayerStatus(ussr)
	})

	t.Run("DisplayAllPlayers", func(t *testing.T) {
		DisplayAllPlayers(g)
	})

	t.Run("DisplayTerritory", func(t *testing.T) {
		DisplayTerritory(g, "Moscow")
		DisplayTerritory(g, "NonExistent") // Should handle gracefully
	})

	t.Run("DisplayBoard", func(t *testing.T) {
		DisplayBoard(g)
	})

	t.Run("DisplayIncome", func(t *testing.T) {
		DisplayIncome(controller)
	})

	t.Run("DisplayVictoryCities", func(t *testing.T) {
		DisplayVictoryCities(controller)
	})

	t.Run("DisplayPhaseHelp", func(t *testing.T) {
		for phase := models.PurchasePhase; phase <= models.CollectIncomePhase; phase++ {
			DisplayPhaseHelp(phase)
		}
	})

	t.Run("DisplayPurchasedUnits", func(t *testing.T) {
		// With no units
		DisplayPurchasedUnits(g, "USSR")

		// With purchased units
		g.PurchasedUnits["USSR"] = []*models.PendingUnit{
			{Type: "infantry", Cost: 3},
			{Type: "infantry", Cost: 3},
			{Type: "armor", Cost: 5},
		}
		DisplayPurchasedUnits(g, "USSR")
	})
}

// TestDisplayBattle tests battle display
func TestDisplayBattle(t *testing.T) {
	battle := game.NewBattle("France", game.LandBattle, "Germany", "UK")

	infantry := &models.Piece{Name: "infantry", Attack: 1, Defend: 2}
	tank := &models.Piece{Name: "tank", Attack: 3, Defend: 3}

	battle.AddAttacker(infantry)
	battle.AddAttacker(tank)
	battle.AddDefender(infantry)
	battle.AddDefender(infantry)

	DisplayBattle(battle)
}

// TestDisplayBattleResult tests battle result display
func TestDisplayBattleResult(t *testing.T) {
	roller := game.NewSeededDiceRoller(42)
	battle := game.NewBattle("France", game.LandBattle, "Germany", "UK")

	for i := 0; i < 3; i++ {
		battle.AddAttacker(&models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3})
	}

	battle.AddDefender(&models.Piece{Name: "infantry", Attack: 1, Defend: 2, Cost: 3})

	result, err := game.ResolveCombat(battle, roller, 10)
	if err != nil {
		t.Fatalf("Combat failed: %v", err)
	}

	DisplayBattleResult(result)
}

// TestClearScreen tests clear screen (just ensure it doesn't panic)
func TestClearScreen(t *testing.T) {
	ClearScreen()
}

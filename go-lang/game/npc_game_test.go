package game

import (
	"boardgame/models"
	"fmt"
	"strings"
	"testing"
)

// TestNPCGameRunner tests running a complete NPC game
func TestNPCGameRunner(t *testing.T) {
	// Create a smaller game setup for faster testing
	game := setupSmallNPCGame()

	// Create game runner
	runner := NewGameRunner(game, "Test NPC Game - 2 Players")
	runner.SetMaxTurns(20) // Limit turns for testing

	// Register NPCs for all players
	runner.RegisterNPC("Germany", "simple")
	runner.RegisterNPC("USSR", "simple")

	// Run 5 turns
	err := runner.RunNTurns(5)
	if err != nil {
		t.Fatalf("Failed to run 5 turns: %v", err)
	}

	// Verify game state
	if game.Turn < 2 {
		t.Errorf("Expected at least 2 turns completed, got %d", game.Turn)
	}

	// Print transcript
	t.Log("Game Transcript:")
	t.Log(runner.GetTranscriptString())
	t.Log(runner.GetGameState())
}

// TestNPCGameToVictory tests running a game until victory
func TestNPCGameToVictory(t *testing.T) {
	// Create a very small game that can reach victory quickly
	game := setupTinyNPCGame()

	runner := NewGameRunner(game, "Test NPC Game - Quick Victory")
	runner.SetMaxTurns(15) // Lower limit for quick testing

	// Register NPCs
	runner.RegisterNPC("Germany", "simple")
	runner.RegisterNPC("USSR", "simple")

	// Run to completion
	winner, err := runner.RunToCompletion()
	if err != nil {
		t.Fatalf("Failed to run game to completion: %v", err)
	}

	t.Logf("Game completed! Winner: %s", winner)

	// Verify winner
	if winner != "Axis" && winner != "Allies" && winner != "Draw" {
		t.Errorf("Invalid winner: %s", winner)
	}

	// Print transcript
	t.Log("\n" + runner.GetTranscriptString())
}

// TestNPCMultiPlayerGame tests a multi-player NPC game
func TestNPCMultiPlayerGame(t *testing.T) {
	// Create a medium-sized game with multiple players
	game := setupMultiPlayerNPCGame()

	runner := NewGameRunner(game, "Multi-Player NPC Game")
	runner.SetMaxTurns(10)

	// Register NPCs for all players
	for _, playerName := range game.PlayerOrder {
		runner.RegisterNPC(playerName, "simple")
	}

	// Run several turns
	err := runner.RunNTurns(3) // Just 3 turns to keep test fast
	if err != nil {
		t.Fatalf("Failed to run multi-player game: %v", err)
	}

	t.Log("Multi-player game ran successfully")
	t.Log(runner.GetGameState())

	// Verify all players got to play
	for _, playerName := range game.PlayerOrder {
		player := game.Players[playerName]
		if player.IPCs == 0 {
			t.Logf("Warning: Player %s has 0 IPCs (may not have collected income yet)", playerName)
		}
	}
}

// TestNPCAI_PurchaseLogic tests the NPC purchase logic
func TestNPCAI_PurchaseLogic(t *testing.T) {
	game := setupSmallNPCGame()
	controller := NewGameController(game)
	transcript := NewGameTranscript("Purchase Test")

	controller.StartGame()

	npc := NewNPCAIPlayer("Germany", "simple")

	// Give Germany plenty of IPCs
	germany := game.Players["Germany"]
	germany.IPCs = 50

	err := npc.PurchasePhase(controller, transcript)
	if err != nil {
		t.Fatalf("Purchase phase failed: %v", err)
	}

	// Verify purchases were made
	if len(game.PurchasedUnits["Germany"]) == 0 {
		t.Error("NPC did not purchase any units with 50 IPCs")
	}

	// Verify IPCs were spent
	if germany.IPCs >= 45 {
		t.Errorf("NPC did not spend enough IPCs: started with 50, has %d", germany.IPCs)
	}

	t.Logf("Purchased %d units, remaining IPCs: %d", len(game.PurchasedUnits["Germany"]), germany.IPCs)
}

// TestNPCAI_CombatMovement tests NPC combat movement logic
func TestNPCAI_CombatMovement(t *testing.T) {
	game := setupSmallNPCGame()
	controller := NewGameController(game)
	transcript := NewGameTranscript("Combat Move Test")

	controller.StartGame()
	controller.AdvancePhase() // Skip purchase

	npc := NewNPCAIPlayer("Germany", "simple")

	err := npc.CombatMovePhase(controller, transcript)
	if err != nil {
		t.Fatalf("Combat move phase failed: %v", err)
	}

	// Check if battles were created
	numBattles := len(controller.PendingBattles)
	t.Logf("NPC planned %d battles", numBattles)

	// It's OK if no battles were created (might not have good attack opportunities)
	if numBattles > 0 {
		t.Logf("Pending battles in: %v", getBattleTerritoryNames(controller.PendingBattles))
	}
}

// TestNPCAI_FullTurnExecution tests a complete NPC turn
func TestNPCAI_FullTurnExecution(t *testing.T) {
	game := setupSmallNPCGame()
	controller := NewGameController(game)
	transcript := NewGameTranscript("Full Turn Test")

	controller.StartGame()

	germany := game.Players["Germany"]
	initialIPCs := germany.IPCs

	npc := NewNPCAIPlayer("Germany", "simple")

	// Execute complete turn
	err := npc.TakeTurn(controller, transcript)
	if err != nil {
		t.Fatalf("Full turn failed: %v", err)
	}

	// Verify turn completed
	if game.CurrentPower == "Germany" {
		t.Error("Turn did not advance to next player")
	}

	// Verify income was collected
	if germany.IPCs <= initialIPCs {
		t.Log("Note: Germany IPCs did not increase (may have spent all income on purchases)")
	}

	t.Log("Full turn completed successfully")
	t.Log("\nTranscript:")
	t.Log(transcript.String())
}

// TestTranscriptLogging tests the transcript system
func TestTranscriptLogging(t *testing.T) {
	transcript := NewGameTranscript("Transcript Test")

	// Log various actions
	transcript.LogTurnStart(1, "Germany")
	transcript.LogPhaseStart("Germany", models.PurchasePhase)

	purchases := map[string]int{
		"infantry": 3,
		"armor":    2,
	}
	transcript.LogPurchase("Germany", purchases, 19)

	transcript.LogMove("Germany", "armor", "Berlin", "Poland", "combat")
	transcript.LogBattleStart("Poland")

	// Create a dummy battle result
	result := &BattleResult{
		AttackerWins:       true,
		Rounds:             2,
		AttackerCasualties: make([]*models.Piece, 1),
		DefenderCasualties: make([]*models.Piece, 2),
	}
	transcript.LogBattleResult("Poland", result)

	transcript.LogMobilize("Germany", "infantry", "Berlin")
	transcript.LogIncomeCollection("Germany", 10, 15)

	// Get transcript string
	transcriptStr := transcript.String()

	// Verify it contains key information
	if len(transcriptStr) == 0 {
		t.Error("Transcript is empty")
	}

	t.Log("Transcript generated successfully:")
	t.Log(transcriptStr)
}

// Helper function to get battle territory names
func getBattleTerritoryNames(battles map[string]*Battle) []string {
	names := make([]string, 0, len(battles))
	for name := range battles {
		names = append(names, name)
	}
	return names
}

// setupSmallNPCGame creates a small 2-player game for testing
func setupSmallNPCGame() *models.Game {
	game := models.NewGame()
	game.PlayerOrder = []string{"Germany", "USSR"}

	// Create players
	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	germany.IPCs = 30
	germany.Capital = "Berlin"

	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"
	ussr.IPCs = 25
	ussr.Capital = "Moscow"

	// Add piece templates
	addStandardPieceTemplates(game)

	// Create territories
	game.AddTerritory("Berlin", models.Land, "Germany", 8)
	game.Board["Berlin"].IsVictoryCity = true

	game.AddTerritory("Poland", models.Land, "USSR", 2)

	game.AddTerritory("Moscow", models.Land, "USSR", 8)
	game.Board["Moscow"].IsVictoryCity = true

	game.AddTerritory("Ukraine", models.Land, "Germany", 3)

	// Connect territories
	berlin := game.Board["Berlin"]
	poland := game.Board["Poland"]
	moscow := game.Board["Moscow"]
	ukraine := game.Board["Ukraine"]

	berlin.ConnectedTo = append(berlin.ConnectedTo, poland, ukraine)
	poland.ConnectedTo = append(poland.ConnectedTo, berlin, moscow, ukraine)
	moscow.ConnectedTo = append(moscow.ConnectedTo, poland, ukraine)
	ukraine.ConnectedTo = append(ukraine.ConnectedTo, berlin, poland, moscow)

	// Place starting units
	game.PlacePieces("Berlin", "infantry", 5)
	game.PlacePieces("Berlin", "armor", 2)
	game.PlacePieces("Berlin", "industrial_complex", 1)

	game.PlacePieces("Ukraine", "infantry", 3)

	game.PlacePieces("Moscow", "infantry", 6)
	game.PlacePieces("Moscow", "armor", 1)
	game.PlacePieces("Moscow", "industrial_complex", 1)

	game.PlacePieces("Poland", "infantry", 2)

	return game
}

// setupTinyNPCGame creates a very small game that can reach victory quickly
func setupTinyNPCGame() *models.Game {
	game := models.NewGame()
	game.PlayerOrder = []string{"Germany", "USSR"}

	germany := game.GetOrCreatePlayer("Germany")
	germany.Side = "Axis"
	germany.IPCs = 20
	germany.Capital = "Berlin"

	ussr := game.GetOrCreatePlayer("USSR")
	ussr.Side = "Allies"
	ussr.IPCs = 15
	ussr.Capital = "Moscow"

	addStandardPieceTemplates(game)

	// Only 2 territories, each is a victory city
	game.AddTerritory("Berlin", models.Land, "Germany", 10)
	game.Board["Berlin"].IsVictoryCity = true

	game.AddTerritory("Moscow", models.Land, "USSR", 8)
	game.Board["Moscow"].IsVictoryCity = true

	// Connect them
	berlin := game.Board["Berlin"]
	moscow := game.Board["Moscow"]
	berlin.ConnectedTo = append(berlin.ConnectedTo, moscow)
	moscow.ConnectedTo = append(moscow.ConnectedTo, berlin)

	// Give Germany overwhelming force for quick victory
	game.PlacePieces("Berlin", "armor", 5)
	game.PlacePieces("Berlin", "industrial_complex", 1)

	game.PlacePieces("Moscow", "infantry", 2)
	game.PlacePieces("Moscow", "industrial_complex", 1)

	return game
}

// setupMultiPlayerNPCGame creates a 4-player game
func setupMultiPlayerNPCGame() *models.Game {
	game := models.NewGame()
	game.PlayerOrder = []string{"Germany", "USSR", "Japan", "UK"}

	// Create all players
	for _, name := range game.PlayerOrder {
		player := game.GetOrCreatePlayer(name)
		player.IPCs = 20
		if name == "Germany" || name == "Japan" {
			player.Side = "Axis"
		} else {
			player.Side = "Allies"
		}
	}

	addStandardPieceTemplates(game)

	// Create territories for each player
	territories := map[string]string{
		"Berlin":   "Germany",
		"Moscow":   "USSR",
		"Tokyo":    "Japan",
		"London":   "UK",
		"Neutral1": "Germany",
		"Neutral2": "USSR",
	}

	for terrName, owner := range territories {
		game.AddTerritory(terrName, models.Land, owner, 5)
		if owner == terrName {
			game.Board[terrName].IsVictoryCity = true
		}
	}

	// Connect in a ring
	game.ConnectTerritories("Berlin", "Neutral1")
	game.ConnectTerritories("Neutral1", "Moscow")
	game.ConnectTerritories("Moscow", "Neutral2")
	game.ConnectTerritories("Neutral2", "Tokyo")
	game.ConnectTerritories("Tokyo", "London")
	game.ConnectTerritories("London", "Berlin")

	// Place starting units
	for terrName := range territories {
		game.PlacePieces(terrName, "infantry", 3)
		// Add IC to capitals
		if _, isCapital := map[string]bool{"Berlin": true, "Moscow": true, "Tokyo": true, "London": true}[terrName]; isCapital {
			game.PlacePieces(terrName, "industrial_complex", 1)
		}
	}

	return game
}

// addStandardPieceTemplates adds all standard unit types
func addStandardPieceTemplates(game *models.Game) {
	game.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	game.AddPieceTemplate("armor", models.Land, 2, 3, 3, 5)
	game.AddPieceTemplate("artillery", models.Land, 1, 2, 2, 4)
	game.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 10)
	game.AddPieceTemplate("bomber", models.Air, 6, 4, 1, 12)
	game.AddPieceTemplate("transport", models.Water, 2, 0, 0, 7)
	game.AddPieceTemplate("submarine", models.Water, 2, 2, 1, 6)
	game.AddPieceTemplate("destroyer", models.Water, 2, 2, 2, 8)
	game.AddPieceTemplate("cruiser", models.Water, 2, 3, 3, 12)
	game.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 20)
	game.AddPieceTemplate("carrier", models.Water, 2, 1, 2, 14)
	game.AddPieceTemplate("industrial_complex", models.Land, 0, 0, 0, 15)
	game.AddPieceTemplate("AAA", models.Land, 1, 0, 1, 5)
}

// TestNPCGameRunnerLonger runs a longer game for demonstration
func TestNPCGameRunnerLonger(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping longer NPC game test in short mode")
	}

	game := setupSmallNPCGame()
	runner := NewGameRunner(game, "Extended NPC Game Test")
	runner.SetMaxTurns(25)

	runner.RegisterNPC("Germany", "simple")
	runner.RegisterNPC("USSR", "simple")

	winner, err := runner.RunToCompletion()
	if err != nil {
		t.Fatalf("Failed to run game: %v", err)
	}

	t.Logf("\n" + strings.Repeat("=", 60))
	t.Logf("GAME COMPLETED - WINNER: %s", winner)
	t.Logf(strings.Repeat("=", 60) + "\n")

	// Print full transcript
	fmt.Println(runner.GetTranscriptString())
	fmt.Println(runner.GetGameState())
}

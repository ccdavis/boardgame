package main

import (
	"boardgame/game"
	"boardgame/models"
	"fmt"
	"os"
)

// This demonstrates running a full NPC vs NPC game until victory
func main() {
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  Axis & Allies NPC Game Demonstration               ║")
	fmt.Println("║  Running automated game to completion...            ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create game
	g := setupDemoGame()

	// Create game runner
	runner := game.NewGameRunner(g, "Axis & Allies - NPC Demo Game")
	runner.SetMaxTurns(30) // Allow up to 30 turns

	// Register all players as NPCs
	runner.RegisterNPC("Germany", "simple")
	runner.RegisterNPC("USSR", "simple")
	runner.RegisterNPC("Japan", "simple")
	runner.RegisterNPC("UK", "simple")

	fmt.Println("Players:")
	for _, playerName := range g.PlayerOrder {
		player := g.Players[playerName]
		fmt.Printf("  • %s (%s) - %d IPCs, %d territories\n",
			player.Name, player.Side, player.IPCs, len(player.Territories))
	}
	fmt.Println()
	fmt.Println("Starting game...\n")

	// Run game to completion
	winner, err := runner.RunToCompletion()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Game error: %v\n", err)
		os.Exit(1)
	}

	// Print full transcript
	fmt.Println(runner.GetTranscriptString())

	// Print final game state
	fmt.Println(runner.GetGameState())

	fmt.Printf("\n🎉 Game complete! Winner: %s\n", winner)
}

// setupDemoGame creates a balanced 4-player game
func setupDemoGame() *models.Game {
	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "USSR", "Japan", "UK"}

	// Create players with balanced starting resources
	powers := map[string]struct {
		side    string
		ipcs    int
		capital string
	}{
		"Germany": {"Axis", 30, "Berlin"},
		"USSR":    {"Allies", 28, "Moscow"},
		"Japan":   {"Axis", 26, "Tokyo"},
		"UK":      {"Allies", 28, "London"},
	}

	for name, data := range powers {
		player := g.GetOrCreatePlayer(name)
		player.Side = data.side
		player.IPCs = data.ipcs
		player.Capital = data.capital
	}

	// Add unit templates
	addAllUnitTemplates(g)

	// Create territories (simplified map)
	territories := map[string]struct {
		owner      string
		production int
		isVC       bool
	}{
		"Berlin":      {"Germany", 10, true},
		"Paris":       {"Germany", 6, true},
		"Rome":        {"Germany", 5, false},
		"Moscow":      {"USSR", 8, true},
		"Stalingrad":  {"USSR", 3, false},
		"Leningrad":   {"USSR", 2, false},
		"Tokyo":       {"Japan", 8, true},
		"Manchuria":   {"Japan", 3, false},
		"Philippines": {"Japan", 2, false},
		"London":      {"UK", 8, true},
		"India":       {"UK", 3, true},
		"Egypt":       {"UK", 2, false},
	}

	for name, data := range territories {
		g.AddTerritory(name, models.Land, data.owner, data.production)
		if data.isVC {
			g.Board[name].IsVictoryCity = true
		}
	}

	// Connect territories in a balanced way
	connections := map[string][]string{
		"Berlin":      {"Paris", "Moscow", "Rome"},
		"Paris":       {"Berlin", "London"},
		"Rome":        {"Berlin", "Egypt"},
		"Moscow":      {"Berlin", "Stalingrad", "Leningrad"},
		"Stalingrad":  {"Moscow", "Manchuria"},
		"Leningrad":   {"Moscow"},
		"Tokyo":       {"Manchuria", "Philippines"},
		"Manchuria":   {"Tokyo", "Stalingrad"},
		"Philippines": {"Tokyo", "India"},
		"London":      {"Paris", "India"},
		"India":       {"London", "Philippines", "Egypt"},
		"Egypt":       {"India", "Rome"},
	}

	for from, toList := range connections {
		for _, to := range toList {
			g.ConnectTerritories(from, to)
		}
	}

	// Place starting forces
	startingForces := map[string]map[string]int{
		"Berlin":      {"infantry": 6, "armor": 2, "fighter": 1, "industrial_complex": 1},
		"Paris":       {"infantry": 3, "armor": 1},
		"Rome":        {"infantry": 2},
		"Moscow":      {"infantry": 8, "armor": 2, "industrial_complex": 1},
		"Stalingrad":  {"infantry": 3},
		"Leningrad":   {"infantry": 2},
		"Tokyo":       {"infantry": 6, "armor": 1, "fighter": 1, "industrial_complex": 1},
		"Manchuria":   {"infantry": 3},
		"Philippines": {"infantry": 2},
		"London":      {"infantry": 6, "fighter": 2, "industrial_complex": 1},
		"India":       {"infantry": 3},
		"Egypt":       {"infantry": 2},
	}

	for territory, units := range startingForces {
		for unitType, count := range units {
			g.PlacePieces(territory, unitType, count)
		}
	}

	return g
}

// addAllUnitTemplates adds standard A&A unit types
func addAllUnitTemplates(g *models.Game) {
	// Name, Terrain, Movement, Attack, Defend, Cost
	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("armor", models.Land, 2, 3, 3, 5)
	g.AddPieceTemplate("artillery", models.Land, 1, 2, 2, 4)
	g.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 10)
	g.AddPieceTemplate("bomber", models.Air, 6, 4, 1, 12)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 0, 7)
	g.AddPieceTemplate("submarine", models.Water, 2, 2, 1, 6)
	g.AddPieceTemplate("destroyer", models.Water, 2, 2, 2, 8)
	g.AddPieceTemplate("cruiser", models.Water, 2, 3, 3, 12)
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 20)
	g.AddPieceTemplate("carrier", models.Water, 2, 1, 2, 14)
	g.AddPieceTemplate("industrial_complex", models.Land, 0, 0, 0, 15)
	g.AddPieceTemplate("AAA", models.Land, 1, 0, 1, 5)
}

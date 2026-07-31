package main

import (
	"boardgame/game"
	"boardgame/models"
	"boardgame/parser"
	"fmt"
	"os"
	"path/filepath"
)

// This demonstrates running a full NPC vs NPC game until victory
func main() {
	fmt.Println("╔══════════════════════════════════════════════════════╗")
	fmt.Println("║  Axis & Allies NPC Game Demonstration               ║")
	fmt.Println("║  Running automated game to completion...            ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()

	// Load game from the real Axis & Allies 1942 map
	g, err := loadRealGame()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load game: %v\n", err)
		os.Exit(1)
	}

	// Create game runner
	runner := game.NewGameRunner(g, "Axis & Allies - NPC Demo Game")
	runner.SetMaxTurns(30) // Allow up to 30 turns

	// Register main powers as NPCs (Germany, USA, USSR, UK, Japan are in the game)
	runner.RegisterNPC("Germany", "simple")
	runner.RegisterNPC("USA", "simple")
	runner.RegisterNPC("USSR", "simple")
	runner.RegisterNPC("UK", "simple")
	runner.RegisterNPC("Japan", "simple")

	fmt.Println("Players:")
	for _, playerName := range g.PlayerOrder {
		player := g.Players[playerName]
		fmt.Printf("  • %s (%s) - %d IPCs, %d territories\n",
			player.Name, player.Side, player.IPCs, len(player.Territories))
	}
	fmt.Println()
	fmt.Print("Starting game...\n\n")

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

// loadRealGame loads the real Axis & Allies 1942 map from aaa.gdf
func loadRealGame() (*models.Game, error) {
	// Find the aaa.gdf file (it's in the parent directory)
	gdfPath := filepath.Join("..", "aaa.gdf")

	// Parse the game definition file
	p, err := parser.NewParser(gdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %v", err)
	}

	g, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse game: %v", err)
	}

	// Set player metadata (side affiliations and capitals)
	playerData := map[string]struct {
		side    string
		capital string
	}{
		"Germany": {"Axis", "Germany"},
		"Japan":   {"Axis", "Japan"},
		"USSR":    {"Allies", "Russia"},
		"UK":      {"Allies", "Britain"},
		"USA":     {"Allies", "Eastern US"},
	}

	for name, data := range playerData {
		if player, exists := g.Players[name]; exists {
			player.Side = data.side
			player.Capital = data.capital
		}
	}

	// Mark victory cities according to Axis & Allies 1942 2nd Edition rules
	victoryCities := []string{
		"Germany",           // Berlin
		"Western Europe",    // Paris
		"Southern Europe",   // Rome
		"Karelia",          // Leningrad
		"Russia",           // Moscow
		"Caucases",         // Stalingrad region
		"Japan",            // Tokyo
		"Philipines",       // Manila
		"Kwantung Eastern China", // Hong Kong
		"India",            // Calcutta
		"Britain",          // London
		"Eastern US",       // Washington
		"Western US",       // San Francisco (some editions)
	}

	for _, cityName := range victoryCities {
		if territory, exists := g.Board[cityName]; exists {
			territory.IsVictoryCity = true
		}
	}

	// Remove "Neutral" from player order - it's not a real player that takes turns
	newPlayerOrder := make([]string, 0)
	for _, playerName := range g.PlayerOrder {
		if playerName != "Neutral" {
			newPlayerOrder = append(newPlayerOrder, playerName)
		}
	}
	g.PlayerOrder = newPlayerOrder

	return g, nil
}

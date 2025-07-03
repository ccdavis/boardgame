package main

import (
	"fmt"
	"log"
	"os"

	"github.com/boardgame/Go/game"
	"github.com/boardgame/Go/parsing"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: persist_demo <gdf_file> [save_file]")
		fmt.Println("  gdf_file: Path to the game definition file")
		fmt.Println("  save_file: Optional path to save/load game state (default: game_save.json)")
		os.Exit(1)
	}

	gdfFile := os.Args[1]
	saveFile := "game_save.json"
	if len(os.Args) > 2 {
		saveFile = os.Args[2]
	}

	// Check if a saved game exists
	if game.FileExists(saveFile) {
		fmt.Printf("Loading saved game from %s...\n", saveFile)
		loadedGame, err := game.LoadGameFromFile(saveFile)
		if err != nil {
			log.Fatalf("Failed to load saved game: %v", err)
		}

		fmt.Println("Game loaded successfully!")
		fmt.Printf("Players: %d\n", len(loadedGame.Players))
		fmt.Printf("Territories: %d\n", len(loadedGame.Board))
		fmt.Printf("Pieces: %d\n", len(loadedGame.Pieces))

		// Show some game state
		for i, player := range loadedGame.Players {
			fmt.Printf("\nPlayer %d: %s\n", i+1, player.Name)
			fmt.Printf("  Territories owned: %d\n", len(player.Territories))
			if len(player.Territories) > 0 {
				fmt.Printf("  First territory: %s\n", player.Territories[0].Name)
			}
		}
		return
	}

	// Otherwise, load from GDF file
	fmt.Printf("Loading game from %s...\n", gdfFile)
	
	file, err := os.Open(gdfFile)
	if err != nil {
		log.Fatalf("Failed to open game file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	newGame, err := game.NewGame(gameState)
	if err != nil {
		log.Fatalf("Failed to create game: %v", err)
	}

	fmt.Println("Game created successfully!")
	fmt.Printf("Players: %d\n", len(newGame.Players))
	fmt.Printf("Territories: %d\n", len(newGame.Board))
	fmt.Printf("Pieces: %d\n", len(newGame.Pieces))

	// Make a change to demonstrate persistence
	if len(newGame.Board) > 0 && len(newGame.Players) > 1 {
		territory := newGame.Board[0]
		newOwner := newGame.Players[1]
		oldOwner := territory.Owner
		
		fmt.Printf("\nChanging ownership of %s from %s to %s\n", 
			territory.Name, oldOwner.Name, newOwner.Name)
		
		game.ChangeOwnership(territory, newOwner)
	}

	// Save the game
	fmt.Printf("\nSaving game to %s...\n", saveFile)
	err = newGame.SaveToFile(saveFile)
	if err != nil {
		log.Fatalf("Failed to save game: %v", err)
	}

	fmt.Println("Game saved successfully!")
	fmt.Println("\nRun the program again to load the saved game.")
}
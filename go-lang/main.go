package main

import (
	"boardgame/game"
	"boardgame/parser"
	"boardgame/ui"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: boardgame <input.gdf>")
		fmt.Println("  Loads a game definition file and starts an interactive game")
		os.Exit(1)
	}

	inputFile := os.Args[1]

	fmt.Printf("Loading game from: %s\n", inputFile)

	// Parse the game file
	p, err := parser.NewParser(inputFile)
	if err != nil {
		fmt.Printf("Error creating parser: %v\n", err)
		os.Exit(1)
	}

	gameModel, err := p.Parse()
	if err != nil {
		fmt.Printf("Error parsing game: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== Game Loaded Successfully ===")
	fmt.Printf("Players: %d, Territories: %d, Pieces: %d\n",
		len(gameModel.Players), len(gameModel.Board), len(gameModel.Pieces))
	fmt.Println()

	// Create game controller
	controller := game.NewGameController(gameModel)

	// Create and run terminal interface
	terminal := ui.NewTerminal(controller)
	err = terminal.Run()
	if err != nil {
		fmt.Printf("Game error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nThanks for playing!")
}

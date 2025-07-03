package main

import (
	"fmt"
	"log"
	"os"

	"github.com/boardgame/Go/game"
	"github.com/boardgame/Go/parsing"
)

func main() {
	fmt.Println("Opening aaa.gdf...")
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open aaa.gdf: %v", err)
	}
	defer file.Close()

	fmt.Println("Creating parser...")
	parser := parsing.NewGameParser(file)
	
	fmt.Println("Parsing game state...")
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	fmt.Printf("Parse complete! Players: %d, Territories: %d\n", 
		len(gameState.Players), len(gameState.Territories))

	fmt.Println("Creating game object...")
	g, err := game.NewGame(gameState)
	if err != nil {
		log.Fatalf("Failed to create game: %v", err)
	}

	fmt.Printf("Game created successfully!\n")
	fmt.Printf("Players: %d\n", len(g.Players))
	fmt.Printf("Territories: %d\n", len(g.Board))
	fmt.Printf("Pieces: %d\n", len(g.Pieces))
}
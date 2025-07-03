package main

import (
	"fmt"
	"log"
	"os"

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
	
	fmt.Println("Loading game state...")
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	fmt.Printf("Parse complete! Players: %d, Territories: %d\n", 
		len(gameState.Players), len(gameState.Territories))
}
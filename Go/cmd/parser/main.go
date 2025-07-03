package main

import (
	"fmt"
	"log"
	"os"

	"github.com/boardgame/Go/parsing"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <game.gdf>\n", os.Args[0])
		os.Exit(1)
	}

	filename := os.Args[1]
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open file %s: %v", filename, err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	json, err := gameState.AsJSON()
	if err != nil {
		log.Fatalf("Failed to convert to JSON: %v", err)
	}

	fmt.Println(json)
}
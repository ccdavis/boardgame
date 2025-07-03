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
	
	// Parse with debug output
	fmt.Println("=== Parsing Players ===")
	gs := parsing.NewGameState()
	if err := parser.Load(); err != nil {
		log.Fatalf("Parse error: %v", err)
	}
	
	json, _ := gs.AsJSON()
	fmt.Println(json)
}
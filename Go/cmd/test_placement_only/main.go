package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Simple placement content
	content := `Placement
Germany: 5 infantry;
Norway: 2 infantry;`

	parser := parsing.NewGameParser(strings.NewReader(content))
	gs := parsing.NewGameState()
	
	fmt.Println("Parsing placement section...")
	err := parser.ParsePlacement(gs)
	if err != nil {
		log.Fatalf("Failed to parse placement: %v", err)
	}
	
	fmt.Printf("Success! Parsed %d placement entries\n", len(gs.Placement))
	for territory, units := range gs.Placement {
		fmt.Printf("  %s: %v\n", territory, units)
	}
}
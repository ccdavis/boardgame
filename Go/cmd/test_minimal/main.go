package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Test with a minimal placement section
	content := `Placement
Germany: 5 infantry;
Norway: 2 infantry;`

	parser := parsing.NewGameParser(strings.NewReader(content))
	
	fmt.Println("Parsing minimal game state...")
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	fmt.Printf("Parse complete! Placement entries: %d\n", len(gameState.Placement))
	for territory, units := range gameState.Placement {
		fmt.Printf("  %s: %v\n", territory, units)
	}
}
package main

import (
	"fmt"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Minimal but complete content
	content := `Players  
Germany, USA;

Turn 1;

Territories
Germany: land, Germany, 10;

Map
Germany: Germany;

Units
infantry: land, 1 attack, 2 defense;

Containers
transport: 2, infantry;

Placement
Germany: 5 infantry;`

	fmt.Println("Testing with minimal but complete content")
	
	parser := parsing.NewGameParser(strings.NewReader(content))
	gs, err := parser.Load()
	
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success!\n")
		fmt.Printf("Players: %v\n", gs.Players)
		fmt.Printf("Turn: %d\n", gs.Turn)
		fmt.Printf("Territories: %d\n", len(gs.Territories))
		fmt.Printf("Placement entries: %d\n", len(gs.Placement))
	}
}
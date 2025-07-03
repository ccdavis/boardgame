package main

import (
	"fmt"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Test with just players
	content := `Players  
Germany, USA, USSR, UK, Japan, Neutral;`

	fmt.Println("Testing with minimal content:")
	fmt.Println(content)
	fmt.Println()
	
	parser := parsing.NewGameParser(strings.NewReader(content))
	gs, err := parser.Load()
	
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Success! Players: %v\n", gs.Players)
	}
}
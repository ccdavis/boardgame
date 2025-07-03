package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Simple test case that should work
	content := `Placement
Germany: 5 infantry;`

	parser := parsing.NewGameParser(strings.NewReader(content))
	
	// Match PLACEMENT
	if err := parser.Match(parsing.PLACEMENT); err != nil {
		log.Fatalf("Failed to match PLACEMENT: %v", err)
	}
	
	fmt.Println("After PLACEMENT, next token:", parser.NextToken())
	
	// First iteration - should work
	fmt.Println("\n=== First iteration ===")
	fmt.Println("Check EOF:", parser.NextToken().Type == parsing.END_OF_FILE)
	
	// Parse territory name
	var parts []string
	for parser.NextToken().Type == parsing.IDENTIFIER {
		parser.Skip()
		parts = append(parts, parser.LastTokenAsString())
	}
	territory := strings.Join(parts, " ")
	fmt.Printf("Territory: %q, next token: %v\n", territory, parser.NextToken())
	
	// Skip rest of entry
	parser.Match(parsing.COLON)
	parser.Match(parsing.INTEGER)
	parser.Match(parsing.IDENTIFIER)
	parser.Match(parsing.SEMICOLON)
	
	fmt.Println("\n=== Second iteration (should be at EOF) ===")
	fmt.Println("Next token:", parser.NextToken())
	fmt.Println("Is EOF?", parser.NextToken().Type == parsing.END_OF_FILE)
	
	// Try to parse territory name again
	parts = nil
	for parser.NextToken().Type == parsing.IDENTIFIER {
		parser.Skip()
		parts = append(parts, parser.LastTokenAsString())
	}
	territory = strings.Join(parts, " ")
	fmt.Printf("Territory: %q (should be empty)\n", territory)
	fmt.Println("Still at same token?", parser.NextToken())
	
	// This is where the infinite loop happens - we're not at EOF, but tryParseTerritoryName returns empty
}
package main

import (
	"fmt"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Minimal content that should work
	content := `Placement
Hawaian Sea: 1 sub;`

	parser := parsing.NewGameParser(strings.NewReader(content))
	
	// Match PLACEMENT
	parser.Match(parsing.PLACEMENT)
	
	iteration := 0
	for {
		iteration++
		fmt.Printf("\n=== Iteration %d ===\n", iteration)
		
		// Check if we've reached the end
		if parser.NextToken().Type == parsing.END_OF_FILE {
			fmt.Println("At EOF, breaking")
			break
		}
		
		fmt.Printf("Not at EOF, next token: %v\n", parser.NextToken())
		
		// Try to parse territory name  
		var parts []string
		for parser.NextToken().Type == parsing.IDENTIFIER {
			parser.Skip()
			parts = append(parts, parser.LastTokenAsString())
		}
		
		territory := strings.Join(parts, " ")
		fmt.Printf("Territory: %q\n", territory)
		
		if territory == "" {
			fmt.Println("Empty territory")
			// If we're not at EOF, consume the unexpected token to avoid infinite loop
			if parser.NextToken().Type != parsing.END_OF_FILE {
				fmt.Printf("Skipping token: %v\n", parser.NextToken())
				parser.Skip()
			}
			fmt.Println("Breaking from loop")
			break
		}
		
		// Parse rest of entry
		parser.Match(parsing.COLON)
		for parser.NextToken().Type != parsing.SEMICOLON {
			parser.Skip()
		}
		parser.Match(parsing.SEMICOLON)
		
		fmt.Println("Entry parsed successfully")
		
		if iteration > 5 {
			fmt.Println("Too many iterations, stopping")
			break
		}
	}
	
	fmt.Printf("\nExited loop after %d iterations\n", iteration)
}
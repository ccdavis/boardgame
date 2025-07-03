package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Get the last few lines of the file
	content, err := os.ReadFile("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	lines := strings.Split(string(content), "\n")
	
	// Find the last placement entry
	lastPlacementLine := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], ":") && strings.Contains(lines[i], ";") {
			lastPlacementLine = i
			break
		}
	}
	
	if lastPlacementLine == -1 {
		log.Fatal("Could not find last placement entry")
	}
	
	fmt.Printf("Last placement line %d: %q\n", lastPlacementLine, lines[lastPlacementLine])
	fmt.Printf("Next line %d: %q\n", lastPlacementLine+1, lines[lastPlacementLine+1])
	
	// Create content starting from the last entry
	testContent := "Placement\n" + lines[lastPlacementLine]
	for i := lastPlacementLine + 1; i < len(lines); i++ {
		testContent += "\n" + lines[i]
	}
	
	fmt.Printf("\nTest content:\n%q\n", testContent)
	
	// Parse this content
	parser := parsing.NewGameParser(strings.NewReader(testContent))
	
	// Match PLACEMENT
	parser.Match(parsing.PLACEMENT)
	
	// Manually trace through one iteration
	fmt.Println("\n=== First iteration ===")
	fmt.Printf("1. Check EOF: %v\n", parser.NextToken().Type == parsing.END_OF_FILE)
	fmt.Printf("2. Next token: %v\n", parser.NextToken())
	
	// Try to parse territory
	var parts []string
	tokenCount := 0
	for parser.NextToken().Type == parsing.IDENTIFIER {
		tokenCount++
		parser.Skip()
		parts = append(parts, parser.LastTokenAsString())
		fmt.Printf("   Got identifier #%d: %q, next: %v\n", tokenCount, parser.LastTokenAsString(), parser.NextToken())
	}
	
	territory := strings.Join(parts, " ")
	fmt.Printf("3. Territory: %q\n", territory)
	fmt.Printf("4. After territory parse, next token: %v\n", parser.NextToken())
	
	// Skip the entry
	if parser.NextToken().Type == parsing.COLON {
		parser.Match(parsing.COLON)
		// Skip units
		for parser.NextToken().Type != parsing.SEMICOLON && parser.NextToken().Type != parsing.END_OF_FILE {
			parser.Skip()
		}
		parser.Match(parsing.SEMICOLON)
	}
	
	fmt.Println("\n=== After first entry ===")
	fmt.Printf("5. Next token: %v\n", parser.NextToken())
	fmt.Printf("6. Is EOF? %v\n", parser.NextToken().Type == parsing.END_OF_FILE)
	
	// Try second iteration
	fmt.Println("\n=== Second iteration (should detect EOF) ===")
	
	// This is what the loop does
	if parser.NextToken().Type == parsing.END_OF_FILE {
		fmt.Println("Loop would exit here")
	} else {
		fmt.Printf("Loop continues, token: %v\n", parser.NextToken())
		
		// Try to parse territory
		parts = nil
		for parser.NextToken().Type == parsing.IDENTIFIER {
			parser.Skip()
			parts = append(parts, parser.LastTokenAsString())
		}
		
		territory = strings.Join(parts, " ")
		fmt.Printf("Territory attempt 2: %q\n", territory)
		
		if territory == "" {
			fmt.Printf("Empty territory. Next token: %v\n", parser.NextToken())
			fmt.Println("This is where the infinite loop happens!")
		}
	}
}
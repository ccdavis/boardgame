package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Read the file
	content, err := os.ReadFile("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	// Find where placement section starts
	placementIdx := strings.Index(string(content), "Placement")
	if placementIdx == -1 {
		log.Fatal("Could not find Placement section")
	}
	
	// Get content from placement to end
	placementContent := string(content[placementIdx:])
	
	// Find the last semicolon
	lastSemi := strings.LastIndex(placementContent, ";")
	if lastSemi == -1 {
		log.Fatal("No semicolon found")
	}
	
	// Get what's after the last semicolon
	afterLast := placementContent[lastSemi+1:]
	fmt.Printf("After last semicolon: %q\n", afterLast)
	fmt.Printf("Length: %d\n", len(afterLast))
	
	// Parse those tokens
	parser := parsing.NewParser(strings.NewReader(afterLast))
	fmt.Println("\nTokens after last semicolon:")
	for i := 0; i < 5; i++ {
		tok := parser.NextToken()
		fmt.Printf("  Token %d: %v\n", i+1, tok)
		if tok.Type == parsing.END_OF_FILE {
			break
		}
		parser.Skip()
	}
}
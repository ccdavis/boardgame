package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

type DebugParser struct {
	*parsing.Parser
	counter int
}

func NewDebugParser(content string) *DebugParser {
	return &DebugParser{
		Parser: parsing.NewParser(strings.NewReader(content)),
	}
}

func (dp *DebugParser) Skip() {
	dp.counter++
	if dp.counter > 1000 {
		panic("Too many tokens processed - infinite loop detected")
	}
	fmt.Printf("Token %d: %v\n", dp.counter, dp.NextToken())
	dp.Parser.Skip()
}

func main() {
	// Read just the placement section
	content, err := os.ReadFile("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	// Find the PLACEMENT section
	placementIdx := strings.Index(string(content), "Placement")
	if placementIdx == -1 {
		log.Fatal("Could not find Placement section")
	}
	
	// Get just the placement section
	placementContent := string(content[placementIdx:])
	fmt.Printf("Placement content (first 500 chars):\n%s\n", placementContent[:min(500, len(placementContent))])
	
	// Test with debug parser
	dp := NewDebugParser(placementContent)
	
	// Skip "Placement" token
	if dp.NextToken().Type != parsing.PLACEMENT {
		log.Fatal("Expected PLACEMENT token")
	}
	dp.Skip()
	
	fmt.Println("\nProcessing placement entries...")
	
	// Try to parse one entry manually
	fmt.Println("Next token:", dp.NextToken())
	
	// Parse territory name
	var parts []string
	for dp.NextToken().Type == parsing.IDENTIFIER {
		dp.Skip()
		parts = append(parts, dp.LastTokenAsString())
	}
	
	territory := strings.Join(parts, " ")
	fmt.Printf("Territory: %q\n", territory)
	fmt.Printf("Next token after territory: %v\n", dp.NextToken())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
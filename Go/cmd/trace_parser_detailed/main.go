package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

type TracingGameParser struct {
	*parsing.Parser
	depth int
}

func NewTracingGameParser(content string) *TracingGameParser {
	return &TracingGameParser{
		Parser: parsing.NewParser(strings.NewReader(content)),
	}
}

func (tgp *TracingGameParser) tryParseTerritoryName() string {
	tgp.depth++
	defer func() { tgp.depth-- }()
	
	fmt.Printf("%sEntering tryParseTerritoryName, next token: %v\n", strings.Repeat("  ", tgp.depth), tgp.NextToken())
	
	var parts []string
	for tgp.NextToken().Type == parsing.IDENTIFIER {
		tgp.Skip()
		part := tgp.LastTokenAsString()
		parts = append(parts, part)
		fmt.Printf("%s  Got part: %q, next token: %v\n", strings.Repeat("  ", tgp.depth), part, tgp.NextToken())
	}

	result := strings.Join(parts, " ")
	fmt.Printf("%sReturning territory name: %q\n", strings.Repeat("  ", tgp.depth), result)
	return result
}

func main() {
	// Read file
	content, err := os.ReadFile("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	// Find the last few placement entries
	lines := strings.Split(string(content), "\n")
	placementIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "Placement" {
			placementIdx = i
			break
		}
	}
	
	if placementIdx == -1 {
		log.Fatal("Could not find Placement section")
	}
	
	// Get just the last few placement entries
	lastFewLines := lines[len(lines)-5:]
	testContent := "Placement\n" + strings.Join(lastFewLines, "\n")
	
	fmt.Printf("Testing with content:\n%s\n", testContent)
	fmt.Println("\nStarting parse...")
	
	parser := NewTracingGameParser(testContent)
	
	// Match PLACEMENT
	if err := parser.Match(parsing.PLACEMENT); err != nil {
		log.Fatalf("Failed to match PLACEMENT: %v", err)
	}
	
	// Parse entries
	entryCount := 0
	for parser.NextToken().Type != parsing.END_OF_FILE {
		entryCount++
		fmt.Printf("\n=== Entry %d, next token: %v ===\n", entryCount, parser.NextToken())
		
		territory := parser.tryParseTerritoryName()
		if territory == "" {
			fmt.Printf("Got empty territory name, next token: %v\n", parser.NextToken())
			if parser.NextToken().Type != parsing.END_OF_FILE {
				fmt.Println("Not at EOF but got empty territory - this would cause infinite loop!")
			}
			break
		}
		
		fmt.Printf("Territory: %q\n", territory)
		
		// Skip the rest of the entry for this test
		if parser.NextToken().Type == parsing.COLON {
			parser.Skip() // COLON
			// Skip units
			for parser.NextToken().Type != parsing.SEMICOLON && parser.NextToken().Type != parsing.END_OF_FILE {
				parser.Skip()
			}
			if parser.NextToken().Type == parsing.SEMICOLON {
				parser.Skip()
			}
		}
		
		if entryCount > 10 {
			fmt.Println("Too many entries, breaking")
			break
		}
	}
	
	fmt.Printf("\nProcessed %d entries\n", entryCount)
}
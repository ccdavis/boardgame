package main

import (
	"fmt"
	"log"
	"os"
	"github.com/boardgame/Go/parsing"
)

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Create a scanner to manually trace
	scanner := parsing.NewScanner(file)
	
	// Look for problematic patterns
	lineCount := 0
	inMap := false
	mapEntries := 0
	
	for {
		tok := scanner.NextToken()
		
		if tok.Type == parsing.END_OF_FILE {
			break
		}
		
		if tok.Line > lineCount {
			lineCount = tok.Line
		}
		
		// Track Map section
		if tok.Type == parsing.MAP {
			fmt.Printf("Found MAP at line %d\n", tok.Line)
			inMap = true
		} else if tok.Type == parsing.UNITS {
			fmt.Printf("Found UNITS at line %d\n", tok.Line)
			if inMap {
				fmt.Printf("Map section had %d entries\n", mapEntries)
			}
			inMap = false
		}
		
		if inMap && tok.Type == parsing.COLON {
			mapEntries++
			if mapEntries % 10 == 0 {
				fmt.Printf("  Map entry %d at line %d\n", mapEntries, tok.Line)
			}
		}
		
		// Look for the specific problematic line numbers
		if tok.Line >= 270 && tok.Line <= 280 {
			fmt.Printf("Line %d: %v = '%s'\n", tok.Line, tok.Type, tok.StrValue)
		}
	}
	
	fmt.Printf("\nTotal lines: %d\n", lineCount)
}
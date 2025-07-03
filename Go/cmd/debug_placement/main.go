package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Read the entire file content
	content, err := os.ReadFile("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	// Find the PLACEMENT section
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
	
	// Print last 10 lines of the file
	fmt.Println("Last 10 lines of file:")
	start := len(lines) - 10
	if start < 0 {
		start = 0
	}
	for i := start; i < len(lines); i++ {
		fmt.Printf("%d: %q\n", i, lines[i])
	}
	
	// Now test the parser
	fmt.Println("\nTesting parser...")
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	fmt.Printf("Parse complete! Placement entries: %d\n", len(gameState.Placement))
}
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

func main() {
	// Read file and remove placement section
	content, err := os.ReadFile("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	// Find where placement starts
	placementIdx := strings.Index(string(content), "\nPlacement")
	if placementIdx == -1 {
		log.Fatal("Could not find Placement section")
	}
	
	// Get content up to placement
	contentWithoutPlacement := string(content[:placementIdx])
	
	fmt.Printf("Testing parser without placement section (content length: %d)\n", len(contentWithoutPlacement))
	
	parser := parsing.NewGameParser(strings.NewReader(contentWithoutPlacement))
	gs, err := parser.Load()
	
	if err != nil {
		fmt.Printf("Parser failed: %v\n", err)
	} else {
		fmt.Printf("Parser succeeded!\n")
		fmt.Printf("Players: %d\n", len(gs.Players))
		fmt.Printf("Territories: %d\n", len(gs.Territories))
		fmt.Printf("Map entries: %d\n", len(gs.GameMap))
		fmt.Printf("Units: %d\n", len(gs.Units))
		fmt.Printf("Containers: %d\n", len(gs.Containers))
		fmt.Printf("Placement: %d\n", len(gs.Placement))
	}
}
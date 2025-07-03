package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/boardgame/Go/parsing"
)

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gs := parsing.NewGameState()

	sections := []struct {
		name string
		fn   func(*parsing.GameState) error
	}{
		{"players", parser.ParsePlayers},
		{"turn", parser.ParseTurn},
		{"territories", parser.ParseTerritories},
		{"map", parser.ParseMap},
		{"units", parser.ParseUnits},
		{"containers", parser.ParseContainers},
		{"placement", parser.ParsePlacement},
	}

	for _, section := range sections {
		fmt.Printf("Testing %s section... ", section.name)
		start := time.Now()
		
		done := make(chan error)
		go func(fn func(*parsing.GameState) error) {
			done <- fn(gs)
		}(section.fn)
		
		select {
		case err := <-done:
			if err != nil {
				fmt.Printf("ERROR: %v\n", err)
				return
			}
			fmt.Printf("OK (%.2fs)\n", time.Since(start).Seconds())
		case <-time.After(2 * time.Second):
			fmt.Printf("TIMEOUT - hanging!\n")
			return
		}
	}

	fmt.Printf("\nSuccess! All sections parsed.\n")
	fmt.Printf("Players: %d\n", len(gs.Players))
	fmt.Printf("Territories: %d\n", len(gs.Territories))
	fmt.Printf("Units: %d\n", len(gs.Units))
	fmt.Printf("Placement entries: %d\n", len(gs.Placement))
}
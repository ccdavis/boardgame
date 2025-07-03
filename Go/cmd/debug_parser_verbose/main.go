package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/boardgame/Go/parsing"
)

func main() {
	fmt.Println("Opening aaa_clean.gdf...")
	file, err := os.Open("../aaa_clean.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	fmt.Println("Creating parser...")
	parser := parsing.NewGameParser(file)
	
	fmt.Println("Starting to parse...")
	
	// Use a goroutine with timeout to detect hang
	done := make(chan bool)
	var parseErr error
	var gameState *parsing.GameState
	
	go func() {
		gameState, parseErr = parser.Load()
		done <- true
	}()
	
	select {
	case <-done:
		if parseErr != nil {
			log.Fatalf("Failed to parse game: %v", parseErr)
		}
		fmt.Printf("Parse complete!\n")
		fmt.Printf("Players: %d\n", len(gameState.Players))
		fmt.Printf("Territories: %d\n", len(gameState.Territories))
		fmt.Printf("Map entries: %d\n", len(gameState.GameMap))
	case <-time.After(5 * time.Second):
		fmt.Println("Parser appears to be hanging...")
		fmt.Println("This is likely due to a syntax error in the Map section")
		
		// Try to parse just up to the Map section
		fmt.Println("\nTrying to parse sections individually...")
		
		// Create a new parser
		file2, _ := os.Open("../aaa_clean.gdf")
		defer file2.Close()
		parser2 := parsing.NewGameParser(file2)
		
		// Try parsing manually
		gs := parsing.NewGameState()
		
		// Parse players
		if err := parser2.Match(parsing.PLAYERS); err == nil {
			fmt.Println("✓ PLAYERS section parsed")
		}
		
		// Check for Units section
		file3, _ := os.Open("../aaa_clean.gdf")
		scanner := parsing.NewScanner(file3)
		tokenCount := 0
		lastToken := ""
		
		for {
			tok := scanner.NextToken()
			tokenCount++
			
			if tok.Type == parsing.EOF {
				break
			}
			
			if tok.Type == parsing.MAP {
				fmt.Printf("\nFound MAP keyword at token %d (line %d)\n", tokenCount, tok.Line)
				fmt.Printf("Previous tokens: %s\n", lastToken)
				break
			}
			
			if tok.Type == parsing.ERROR {
				fmt.Printf("\nScanner error at line %d: %s\n", tok.Line, tok.Value)
				break
			}
			
			lastToken = fmt.Sprintf("%s(%v)", tok.Type, tok.Value)
		}
		file3.Close()
	}
}
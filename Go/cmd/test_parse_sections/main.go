package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/boardgame/Go/parsing"
)

func parseWithTimeout(parser *parsing.GameParser, section string, parseFunc func() error) error {
	done := make(chan error)
	
	go func() {
		fmt.Printf("Parsing %s...\n", section)
		err := parseFunc()
		done <- err
	}()
	
	select {
	case err := <-done:
		if err != nil {
			fmt.Printf("ERROR in %s: %v\n", section, err)
		} else {
			fmt.Printf("✓ %s parsed successfully\n", section)
		}
		return err
	case <-time.After(5 * time.Second):
		fmt.Printf("✗ %s parsing timed out!\n", section)
		return fmt.Errorf("%s parsing timed out", section)
	}
}

func main() {
	fmt.Println("Opening aaa.gdf...")
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open aaa.gdf: %v", err)
	}
	defer file.Close()

	fmt.Println("Creating parser...")
	parser := parsing.NewGameParser(file)
	
	fmt.Println("Testing Load() function with timeout...")
	
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
		fmt.Printf("✓ Parse complete!\n")
		fmt.Printf("Players: %d\n", len(gameState.Players))
		fmt.Printf("Territories: %d\n", len(gameState.Territories))
		fmt.Printf("Map entries: %d\n", len(gameState.GameMap))
		fmt.Printf("Units: %d\n", len(gameState.Units))
		fmt.Printf("Containers: %d\n", len(gameState.Containers))
		fmt.Printf("Placement entries: %d\n", len(gameState.Placement))
	case <-time.After(10 * time.Second):
		fmt.Println("\n✗ Parser timed out after 10 seconds!")
		fmt.Println("The parser is hanging somewhere in the Load() function")
		fmt.Println("\nLet's check the token stream around the problem area...")
		
		// Create a new scanner to debug
		file2, _ := os.Open("../aaa.gdf")
		defer file2.Close()
		scanner := parsing.NewScanner(file2)
		
		// Skip to around where we think the problem is
		tokenCount := 0
		var lastTokens []string
		
		for tokenCount < 10000 {
			tok := scanner.NextToken()
			tokenCount++
			
			// Keep last 10 tokens
			tokenStr := fmt.Sprintf("%d:%v(%v)", tok.Line, tok.Type, tok.StrValue)
			lastTokens = append(lastTokens, tokenStr)
			if len(lastTokens) > 10 {
				lastTokens = lastTokens[1:]
			}
			
			// Look for specific sections
			if tok.Type == parsing.END_OF_FILE {
				fmt.Printf("\nReached EOF at token %d\n", tokenCount)
				break
			}
		}
		
		fmt.Println("\nLast tokens seen:")
		for _, t := range lastTokens {
			fmt.Println("  ", t)
		}
	}
}
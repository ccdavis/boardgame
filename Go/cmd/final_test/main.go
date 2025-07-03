package main

import (
	"fmt"
	"os"
	"time"

	"github.com/boardgame/Go/parsing"
)

func main() {
	fmt.Println("Test 1: Simple parsing test")
	file1, _ := os.Open("../test_game.gdf")
	parser1 := parsing.NewGameParser(file1)
	gs1, err1 := parser1.Load()
	file1.Close()
	if err1 != nil {
		fmt.Printf("ERROR: %v\n", err1)
	} else {
		fmt.Printf("✓ test_game.gdf parsed: %d territories\n", len(gs1.Territories))
	}

	fmt.Println("\nTest 2: Full aaa.gdf with timeout")
	file2, _ := os.Open("../aaa.gdf")
	defer file2.Close()
	parser2 := parsing.NewGameParser(file2)
	
	done := make(chan error)
	var gs2 *parsing.GameState
	
	go func() {
		var err error
		gs2, err = parser2.Load()
		done <- err
	}()
	
	select {
	case err := <-done:
		if err != nil {
			fmt.Printf("ERROR: %v\n", err)
		} else {
			fmt.Printf("✓ aaa.gdf parsed: %d territories, %d map entries\n", 
				len(gs2.Territories), len(gs2.GameMap))
		}
	case <-time.After(3 * time.Second):
		fmt.Println("✗ Timeout! Parser is hanging")
		
		// Let's manually check what's different
		fmt.Println("\nChecking for differences...")
		
		// Check if it's the placement section
		file3, _ := os.Open("../aaa.gdf")
		scanner := parsing.NewScanner(file3)
		
		foundPlacement := false
		placementEntries := 0
		for {
			tok := scanner.NextToken()
			if tok.Type == parsing.END_OF_FILE {
				break
			}
			
			if tok.Type == parsing.PLACEMENT {
				foundPlacement = true
				fmt.Printf("Found PLACEMENT at line %d\n", tok.Line)
			}
			
			if foundPlacement && tok.Type == parsing.COLON {
				placementEntries++
			}
		}
		file3.Close()
		
		fmt.Printf("Placement entries found: %d\n", placementEntries)
	}
}
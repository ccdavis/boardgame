package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/boardgame/Go/parsing"
)

type DebugGameParser struct {
	*parsing.Parser
	entryCount int
}

func (dgp *DebugGameParser) ParsePlacement(gs *parsing.GameState) error {
	fmt.Println("Starting ParsePlacement")
	if err := dgp.Match(parsing.PLACEMENT); err != nil {
		return err
	}
	fmt.Println("Matched PLACEMENT token")

	for dgp.NextToken().Type != parsing.END_OF_FILE {
		dgp.entryCount++
		fmt.Printf("\n--- Entry %d ---\n", dgp.entryCount)
		fmt.Printf("Next token: %v\n", dgp.NextToken())
		
		// Try to parse territory name
		territory := dgp.tryParseTerritoryName()
		if territory == "" {
			fmt.Println("Got empty territory name, breaking")
			break
		}
		fmt.Printf("Territory: %s\n", territory)
		
		if err := dgp.Match(parsing.COLON); err != nil {
			return err
		}
		fmt.Println("Matched COLON")

		placement := make(map[string]int)
		unitCount := 0

		for dgp.NextToken().Type != parsing.SEMICOLON {
			unitCount++
			if unitCount > 20 {
				fmt.Printf("ERROR: Too many units in entry %d\n", dgp.entryCount)
				return fmt.Errorf("too many units")
			}
			
			if dgp.NextToken().Type == parsing.END_OF_FILE {
				return fmt.Errorf("unexpected EOF in placement")
			}
			
			fmt.Printf("  Unit %d, next token: %v\n", unitCount, dgp.NextToken())
			
			if err := dgp.Match(parsing.INTEGER); err != nil {
				return err
			}
			count := int(dgp.LastTokenAsInteger())

			if err := dgp.Match(parsing.IDENTIFIER); err != nil {
				return err
			}
			unitType := dgp.LastTokenAsString()
			fmt.Printf("  Parsed: %d %s\n", count, unitType)

			placement[unitType] = count

			if dgp.NextToken().Type == parsing.COMMA {
				dgp.Skip()
				fmt.Println("  Skipped COMMA")
			}
		}

		fmt.Println("Exited unit loop, matching SEMICOLON")
		if err := dgp.Match(parsing.SEMICOLON); err != nil {
			return err
		}
		fmt.Println("Matched SEMICOLON")

		gs.Placement[territory] = placement
		
		if dgp.entryCount >= 72 {
			fmt.Printf("\nProcessed entry %d (expecting this to be the last)\n", dgp.entryCount)
			fmt.Printf("Next token after entry: %v\n", dgp.NextToken())
		}
	}

	fmt.Printf("\nExited main loop after %d entries\n", dgp.entryCount)
	return nil
}

func (dgp *DebugGameParser) tryParseTerritoryName() string {
	var parts []string
	for dgp.NextToken().Type == parsing.IDENTIFIER {
		dgp.Skip()
		parts = append(parts, dgp.LastTokenAsString())
	}
	return strings.Join(parts, " ")
}

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	baseParser := parsing.NewParser(file)
	parser := &DebugGameParser{Parser: baseParser}
	gs := parsing.NewGameState()

	// We need to create a new parser from the beginning
	file.Seek(0, 0)
	gp := parsing.NewGameParser(file)
	
	// Parse everything up to placement
	gp.ParsePlayers(gs)
	gp.ParseTurn(gs)
	gp.ParseTerritories(gs)
	gp.ParseMap(gs)
	gp.ParseUnits(gs)
	gp.ParseContainers(gs)
	
	// Now use debug parser for placement
	parser.Parser = gp.Parser
	
	done := make(chan error)
	go func() {
		done <- parser.ParsePlacement(gs)
	}()
	
	select {
	case err := <-done:
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Success! Parsed %d entries\n", parser.entryCount)
		}
	case <-time.After(5 * time.Second):
		fmt.Printf("\nTIMEOUT after %d entries\n", parser.entryCount)
	}
}
package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/boardgame/Go/parsing"
)

type CountingGameParser struct {
	*parsing.GameParser
	placementCount int
}

func (cgp *CountingGameParser) ParsePlacementWithCount(gs *parsing.GameState) error {
	if err := cgp.Match(parsing.PLACEMENT); err != nil {
		return err
	}

	for {
		// Check if we've reached the end
		if cgp.NextToken().Type == parsing.END_OF_FILE {
			fmt.Printf("Reached EOF after %d entries\n", cgp.placementCount)
			break
		}
		
		// Try to parse territory name  
		territory := cgp.tryParseTerritoryName()
		if territory == "" {
			fmt.Printf("Got empty territory after %d entries, next token: %v\n", cgp.placementCount, cgp.NextToken())
			// If we're not at EOF, consume the unexpected token to avoid infinite loop
			if cgp.NextToken().Type != parsing.END_OF_FILE {
				cgp.Skip()
			}
			break
		}

		cgp.placementCount++
		if cgp.placementCount % 10 == 0 {
			fmt.Printf("Processed %d placement entries...\n", cgp.placementCount)
		}

		if err := cgp.Match(parsing.COLON); err != nil {
			return err
		}

		placement := make(map[string]int)

		for cgp.NextToken().Type == parsing.INTEGER {
			cgp.Skip()
			count := int(cgp.LastTokenAsInteger())

			if err := cgp.Match(parsing.IDENTIFIER); err != nil {
				return err
			}
			unitType := cgp.LastTokenAsString()

			placement[unitType] = count

			if cgp.NextToken().Type == parsing.COMMA {
				cgp.Skip()
			}
		}

		if err := cgp.Match(parsing.SEMICOLON); err != nil {
			return err
		}

		gs.Placement[territory] = placement
	}

	return nil
}

func (cgp *CountingGameParser) tryParseTerritoryName() string {
	var parts []string

	for cgp.NextToken().Type == parsing.IDENTIFIER {
		cgp.Skip()
		parts = append(parts, cgp.LastTokenAsString())
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ")
}

func main() {
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	baseParser := parsing.NewGameParser(file)
	parser := &CountingGameParser{GameParser: baseParser}
	gs := parsing.NewGameState()

	// Parse everything up to placement
	parser.ParsePlayers(gs)
	parser.ParseTurn(gs)
	parser.ParseTerritories(gs)
	parser.ParseMap(gs)
	parser.ParseUnits(gs)
	parser.ParseContainers(gs)
	
	// Parse placement with timeout
	fmt.Println("Starting placement parse...")
	done := make(chan error)
	go func() {
		done <- parser.ParsePlacementWithCount(gs)
	}()
	
	select {
	case err := <-done:
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			fmt.Printf("Success! Parsed %d placement entries\n", parser.placementCount)
		}
	case <-time.After(3 * time.Second):
		fmt.Printf("Timeout! Processed %d entries before hanging\n", parser.placementCount)
	}
}
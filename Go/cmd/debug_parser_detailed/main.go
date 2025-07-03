package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/boardgame/Go/parsing"
)

// Custom parser with debug output
type DebugGameParser struct {
	*parsing.Parser
}

func NewDebugGameParser(r *os.File) *DebugGameParser {
	return &DebugGameParser{
		Parser: parsing.NewParser(r),
	}
}

func (gp *DebugGameParser) Load() (*parsing.GameState, error) {
	gs := parsing.NewGameState()

	fmt.Println("Parsing PLAYERS...")
	// Parse players
	if err := gp.Match(parsing.PLAYERS); err != nil {
		return nil, err
	}
	for gp.NextToken().Type == parsing.IDENTIFIER {
		gp.Skip()
		player := gp.LastTokenAsString()
		gs.Players = append(gs.Players, player)
		fmt.Printf("  Added player: %s\n", player)
		if gp.NextToken().Type == parsing.COMMA {
			gp.Skip()
		}
	}
	if err := gp.Match(parsing.SEMICOLON); err != nil {
		return nil, err
	}

	fmt.Println("Parsing TURN...")
	// Parse turn
	if err := gp.Match(parsing.TURN); err != nil {
		return nil, err
	}
	if err := gp.Match(parsing.INTEGER); err != nil {
		return nil, err
	}
	gs.Turn = int(gp.LastTokenAsInteger())
	if err := gp.Match(parsing.SEMICOLON); err != nil {
		return nil, err
	}

	fmt.Println("Parsing TERRITORIES...")
	// Skip territories for now
	if err := gp.Match(parsing.TERRITORIES); err != nil {
		return nil, err
	}
	territoryCount := 0
	for {
		name := gp.tryParseTerritoryName()
		if name == "" {
			break
		}
		territoryCount++
		if territoryCount%10 == 0 {
			fmt.Printf("  Parsed %d territories...\n", territoryCount)
		}
		
		// Skip the rest of territory data
		if err := gp.Match(parsing.COLON); err != nil {
			return nil, err
		}
		// Type
		if err := gp.Match(parsing.IDENTIFIER); err != nil {
			return nil, err
		}
		if err := gp.Match(parsing.COMMA); err != nil {
			return nil, err
		}
		// Owner
		ownerName := gp.tryParseTerritoryName()
		if ownerName == "" {
			return nil, fmt.Errorf("expected owner name")
		}
		if err := gp.Match(parsing.COMMA); err != nil {
			return nil, err
		}
		// Production
		if err := gp.Match(parsing.INTEGER); err != nil {
			return nil, err
		}
		if err := gp.Match(parsing.SEMICOLON); err != nil {
			return nil, err
		}
		
		gs.Territories[name] = map[string]string{
			"type": "land",
			"owner": ownerName,
		}
	}
	fmt.Printf("  Total territories: %d\n", territoryCount)

	fmt.Println("Parsing MAP...")
	// Parse map
	if err := gp.Match(parsing.MAP); err != nil {
		return nil, err
	}
	
	mapCount := 0
	for {
		// Try to parse territory name
		name := gp.tryParseTerritoryName()
		if name == "" {
			fmt.Println("  No more territory names, exiting Map section")
			break
		}
		mapCount++
		
		if mapCount%10 == 0 {
			fmt.Printf("  Parsing map entry %d: %s\n", mapCount, name)
		}

		if err := gp.Match(parsing.COLON); err != nil {
			return nil, err
		}

		adjacents := make([]string, 0)
		adjCount := 0

		for gp.NextToken().Type != parsing.SEMICOLON {
			adjName := gp.tryParseTerritoryName()
			if adjName == "" {
				// Check if it's because we hit the semicolon
				if gp.NextToken().Type == parsing.SEMICOLON {
					break
				}
				fmt.Printf("  ERROR: Could not parse adjacent territory for %s\n", name)
				fmt.Printf("  Next token: %v\n", gp.NextToken())
				return nil, fmt.Errorf("expected territory name in adjacency list for %s", name)
			}
			adjacents = append(adjacents, adjName)
			adjCount++

			if gp.NextToken().Type == parsing.COMMA {
				gp.Skip()
			}
		}

		if err := gp.Match(parsing.SEMICOLON); err != nil {
			return nil, err
		}

		gs.GameMap[name] = adjacents
		if mapCount%10 == 0 {
			fmt.Printf("    %s has %d connections\n", name, adjCount)
		}
	}
	fmt.Printf("  Total map entries: %d\n", mapCount)

	return gs, nil
}

func (gp *DebugGameParser) tryParseTerritoryName() string {
	var parts []string

	for gp.NextToken().Type == parsing.IDENTIFIER {
		gp.Skip()
		parts = append(parts, gp.LastTokenAsString())
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ")
}

func main() {
	fmt.Println("Opening aaa.gdf...")
	file, err := os.Open("../aaa.gdf")
	if err != nil {
		log.Fatalf("Failed to open aaa.gdf: %v", err)
	}
	defer file.Close()

	fmt.Println("Creating debug parser...")
	parser := NewDebugGameParser(file)
	
	fmt.Println("Starting to parse...")
	gameState, err := parser.Load()
	if err != nil {
		log.Fatalf("Failed to parse game: %v", err)
	}

	fmt.Printf("\nParse complete!\n")
	fmt.Printf("Players: %d\n", len(gameState.Players))
	fmt.Printf("Territories: %d\n", len(gameState.Territories))
	fmt.Printf("Map entries: %d\n", len(gameState.GameMap))
}
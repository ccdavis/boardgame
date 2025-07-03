package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/boardgame/Go/game"
	"github.com/boardgame/Go/parsing"
)

// JSONGameData represents the C++ parser output
type JSONGameData struct {
	Players     []string                       `json:"players"`
	Turn        int                            `json:"turn"`
	Territories map[string]JSONTerritory       `json:"territories"`
	GameMap     map[string][]string            `json:"game_map"`
	Units       map[string]JSONUnit            `json:"units"`
	Containers  map[string]JSONContainer       `json:"containers"`
	Placement   map[string]map[string]int      `json:"placement"`
}

type JSONTerritory struct {
	Name       string `json:"name"`
	Owner      string `json:"owner"`
	Production string `json:"production"`
	Type       string `json:"type"`
}

type JSONUnit struct {
	Attack   int `json:"attack"`
	Cost     int `json:"cost"`
	Defend   int `json:"defend"`
	Movement int `json:"movement"`
	Type     string `json:"type"`
	Land     int `json:"land"`
	Water    int `json:"water"`
	Air      int `json:"air"`
}

type JSONContainer struct {
	Capacity int `json:"capacity"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: json_to_game <input.json>")
		os.Exit(1)
	}

	// Read JSON file
	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to read JSON file: %v", err)
	}

	// Parse JSON
	var jsonData JSONGameData
	if err := json.Unmarshal(data, &jsonData); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	// Convert to GameState format expected by our parser
	gameState := &parsing.GameState{
		Players:     jsonData.Players,
		Turn:        jsonData.Turn,
		Territories: make(map[string]map[string]string),
		GameMap:     jsonData.GameMap,
		Units:       make(map[string]map[string]int),
		Containers:  make(map[string]map[string]int),
		Placement:   jsonData.Placement,
	}

	// Convert territories
	for name, terr := range jsonData.Territories {
		gameState.Territories[name] = map[string]string{
			"type":       terr.Type,
			"owner":      terr.Owner,
			"production": terr.Production,
		}
	}

	// Convert units
	for name, unit := range jsonData.Units {
		unitData := make(map[string]int)
		unitData["attack"] = unit.Attack
		unitData["defend"] = unit.Defend
		unitData["movement"] = unit.Movement
		unitData["cost"] = unit.Cost
		
		// Map unit type from JSON fields
		if unit.Land > 0 {
			unitData["land"] = unit.Land
		}
		if unit.Water > 0 {
			unitData["water"] = unit.Water
		}
		if unit.Air > 0 {
			unitData["air"] = unit.Air
		}
		
		// Fallback to Type field if needed
		if unit.Land == 0 && unit.Water == 0 && unit.Air == 0 && unit.Type != "" {
			switch unit.Type {
			case "land":
				unitData["land"] = 1
			case "water":
				unitData["water"] = 1
			case "air":
				unitData["air"] = 1
			}
		}
		
		gameState.Units[name] = unitData
	}

	// Convert containers
	for name, container := range jsonData.Containers {
		gameState.Containers[name] = map[string]int{
			"capacity": container.Capacity,
		}
	}

	// Create game from state
	g, err := game.NewGame(gameState)
	if err != nil {
		log.Fatalf("Failed to create game: %v", err)
	}

	fmt.Printf("Game loaded from JSON successfully!\n")
	fmt.Printf("Players: %d\n", len(g.Players))
	fmt.Printf("Territories: %d\n", len(g.Board))
	fmt.Printf("Pieces: %d\n", len(g.Pieces))
	
	// Count territories by type
	landCount := 0
	seaCount := 0
	for _, terr := range g.Board {
		if coords, ok := game.TerritoryPositions[terr.Name]; ok {
			if coords.IsLand {
				landCount++
			} else {
				seaCount++
			}
		}
	}
	fmt.Printf("Land territories: %d, Sea zones: %d\n", landCount, seaCount)

	// Save as a game file that can be loaded by mapserver
	saveFile := "aaa_from_json.json"
	if err := g.SaveToFile(saveFile); err != nil {
		log.Fatalf("Failed to save game: %v", err)
	}

	fmt.Printf("\nGame saved to %s\n", saveFile)
	fmt.Println("You can now run: go run cmd/mapserver/main.go", saveFile)
}
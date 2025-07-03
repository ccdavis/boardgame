package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/boardgame/Go/game"
	"github.com/boardgame/Go/parsing"
)

var currentGame *game.Game

type MapData struct {
	Territories []TerritoryData `json:"territories"`
	Players     []PlayerData    `json:"players"`
}

type TerritoryData struct {
	Name        string                      `json:"name"`
	Owner       string                      `json:"owner"`
	Pieces      map[string]int              `json:"pieces"`
	Coordinates game.TerritoryCoordinates   `json:"coordinates"`
	Connections []string                    `json:"connections"`
}

type PlayerData struct {
	Name  string              `json:"name"`
	Color game.PlayerColor    `json:"color"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: mapserver <game_file>")
		fmt.Println("  game_file: Path to .gdf file or .json save file")
		os.Exit(1)
	}

	gameFile := os.Args[1]
	port := "8080"
	if len(os.Args) > 2 {
		port = os.Args[2]
	}

	// Load the game
	var err error
	if len(gameFile) > 5 && gameFile[len(gameFile)-5:] == ".json" {
		currentGame, err = game.LoadGameFromFile(gameFile)
		if err != nil {
			log.Fatalf("Failed to load saved game: %v", err)
		}
	} else {
		file, err := os.Open(gameFile)
		if err != nil {
			log.Fatalf("Failed to open game file: %v", err)
		}
		defer file.Close()

		parser := parsing.NewGameParser(file)
		gameState, err := parser.Load()
		if err != nil {
			log.Fatalf("Failed to parse game: %v", err)
		}

		currentGame, err = game.NewGame(gameState)
		if err != nil {
			log.Fatalf("Failed to create game: %v", err)
		}
	}

	fmt.Printf("Game loaded successfully!\n")
	fmt.Printf("Players: %d, Territories: %d, Pieces: %d\n", 
		len(currentGame.Players), len(currentGame.Board), len(currentGame.Pieces))

	// Set up HTTP routes
	http.HandleFunc("/", handleScrollableMap)
	http.HandleFunc("/calibrate", handleCalibration)
	http.HandleFunc("/api/game", handleGameAPI)
	http.HandleFunc("/api/image-coordinates", handleImageCoordinatesAPI)
	
	// Serve static files (like images)
	http.Handle("/hires_aaa_map.jpg", http.FileServer(http.Dir("cmd/mapserver")))

	fmt.Printf("Starting map server on http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleScrollableMap(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "cmd/mapserver/scrollable_worldmap.html")
}

func handleCalibration(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "cmd/mapserver/calibration_tool.html")
}

func handleImageCoordinatesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game.ImageMapTerritoryCoordinates)
}

func handleGameAPI(w http.ResponseWriter, r *http.Request) {
	mapData := MapData{
		Territories: make([]TerritoryData, 0),
		Players:     make([]PlayerData, 0),
	}

	// Add player data
	for _, player := range currentGame.Players {
		color, ok := game.PlayerColors[player.Name]
		if !ok {
			color = game.PlayerColors["default"]
		}
		mapData.Players = append(mapData.Players, PlayerData{
			Name:  player.Name,
			Color: color,
		})
	}

	// Add territory data
	for _, territory := range currentGame.Board {
		// Count pieces by type
		pieceCounts := make(map[string]int)
		for _, pieceID := range territory.Pieces {
			piece, ok := currentGame.Pieces[pieceID]
			if ok {
				pieceCounts[piece.Name]++
			}
		}

		// Get connections
		connections := make([]string, 0)
		for _, conn := range territory.ConnectedTo {
			connections = append(connections, conn.Name)
		}

		// Get coordinates
		coords, ok := game.TerritoryPositions[territory.Name]
		if !ok {
			// Default position if not found
			coords = game.TerritoryCoordinates{X: 50, Y: 50, IsLand: true}
		}

		mapData.Territories = append(mapData.Territories, TerritoryData{
			Name:        territory.Name,
			Owner:       territory.Owner.Name,
			Pieces:      pieceCounts,
			Coordinates: coords,
			Connections: connections,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapData)
}
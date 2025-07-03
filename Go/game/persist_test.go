package game

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/boardgame/Go/parsing"
)

func TestGamePersistence(t *testing.T) {
	file, err := os.Open("../../test_game.gdf")
	if err != nil {
		t.Fatalf("Failed to open game file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	originalGame, err := NewGame(gameState)
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}

	saved := originalGame.ToSavedGame()
	if saved == nil {
		t.Fatal("ToSavedGame returned nil")
	}

	if len(saved.Players) != len(originalGame.Players) {
		t.Errorf("Expected %d players, got %d", len(originalGame.Players), len(saved.Players))
	}

	if len(saved.Territories) != len(originalGame.Board) {
		t.Errorf("Expected %d territories, got %d", len(originalGame.Board), len(saved.Territories))
	}

	if len(saved.Pieces) != len(originalGame.Pieces) {
		t.Errorf("Expected %d pieces, got %d", len(originalGame.Pieces), len(saved.Pieces))
	}

	restoredGame, err := FromSavedGame(saved)
	if err != nil {
		t.Fatalf("Failed to restore game: %v", err)
	}

	if len(restoredGame.Players) != len(originalGame.Players) {
		t.Errorf("Restored game has %d players, original had %d", len(restoredGame.Players), len(originalGame.Players))
	}

	if len(restoredGame.Board) != len(originalGame.Board) {
		t.Errorf("Restored game has %d territories, original had %d", len(restoredGame.Board), len(originalGame.Board))
	}

	if len(restoredGame.Pieces) != len(originalGame.Pieces) {
		t.Errorf("Restored game has %d pieces, original had %d", len(restoredGame.Pieces), len(originalGame.Pieces))
	}

	for i, player := range originalGame.Players {
		restoredPlayer := restoredGame.Players[i]
		if player.Name != restoredPlayer.Name {
			t.Errorf("Player %d name mismatch: %s vs %s", i, player.Name, restoredPlayer.Name)
		}
		if len(player.Territories) != len(restoredPlayer.Territories) {
			t.Errorf("Player %s territory count mismatch: %d vs %d", player.Name, 
				len(player.Territories), len(restoredPlayer.Territories))
		}
	}

	for i, territory := range originalGame.Board {
		restoredTerritory := restoredGame.Board[i]
		if territory.Name != restoredTerritory.Name {
			t.Errorf("Territory %d name mismatch: %s vs %s", i, territory.Name, restoredTerritory.Name)
		}
		if territory.Owner.Name != restoredTerritory.Owner.Name {
			t.Errorf("Territory %s owner mismatch: %s vs %s", territory.Name, 
				territory.Owner.Name, restoredTerritory.Owner.Name)
		}
		if len(territory.Pieces) != len(restoredTerritory.Pieces) {
			t.Errorf("Territory %s piece count mismatch: %d vs %d", territory.Name,
				len(territory.Pieces), len(restoredTerritory.Pieces))
		}
		if len(territory.ConnectedTo) != len(restoredTerritory.ConnectedTo) {
			t.Errorf("Territory %s connection count mismatch: %d vs %d", territory.Name,
				len(territory.ConnectedTo), len(restoredTerritory.ConnectedTo))
		}
	}
}

func TestSaveToFile(t *testing.T) {
	file, err := os.Open("../../test_game.gdf")
	if err != nil {
		t.Fatalf("Failed to open game file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	game, err := NewGame(gameState)
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}

	tempFile, err := ioutil.TempFile("", "test_game_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFileName := tempFile.Name()
	tempFile.Close()
	defer os.Remove(tempFileName)

	err = game.SaveToFile(tempFileName)
	if err != nil {
		t.Fatalf("Failed to save game: %v", err)
	}

	if !FileExists(tempFileName) {
		t.Fatal("Save file was not created")
	}

	loadedGame, err := LoadGameFromFile(tempFileName)
	if err != nil {
		t.Fatalf("Failed to load game: %v", err)
	}

	if len(loadedGame.Players) != len(game.Players) {
		t.Errorf("Loaded game has %d players, original had %d", len(loadedGame.Players), len(game.Players))
	}

	if len(loadedGame.Board) != len(game.Board) {
		t.Errorf("Loaded game has %d territories, original had %d", len(loadedGame.Board), len(game.Board))
	}
}

func TestSaveToJSON(t *testing.T) {
	file, err := os.Open("../../test_game.gdf")
	if err != nil {
		t.Fatalf("Failed to open game file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	game, err := NewGame(gameState)
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}

	jsonData, err := game.SaveToJSON()
	if err != nil {
		t.Fatalf("Failed to save game to JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Fatal("JSON data is empty")
	}

	loadedGame, err := LoadGameFromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to load game from JSON: %v", err)
	}

	if len(loadedGame.Players) != len(game.Players) {
		t.Errorf("Loaded game has %d players, original had %d", len(loadedGame.Players), len(game.Players))
	}
}

func TestGameStateAfterChanges(t *testing.T) {
	file, err := os.Open("../../test_game.gdf")
	if err != nil {
		t.Fatalf("Failed to open game file: %v", err)
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		t.Fatalf("Failed to parse game: %v", err)
	}

	game, err := NewGame(gameState)
	if err != nil {
		t.Fatalf("Failed to create game: %v", err)
	}

	if len(game.Board) < 2 || len(game.Players) < 2 {
		t.Skip("Not enough territories or players for ownership change test")
	}

	originalOwner := game.Board[0].Owner
	newOwner := game.Players[1]
	if originalOwner == newOwner {
		newOwner = game.Players[0]
	}

	ChangeOwnership(game.Board[0], newOwner)

	jsonData, err := game.SaveToJSON()
	if err != nil {
		t.Fatalf("Failed to save modified game: %v", err)
	}

	loadedGame, err := LoadGameFromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to load modified game: %v", err)
	}

	if loadedGame.Board[0].Owner.Name != newOwner.Name {
		t.Errorf("Territory ownership not preserved: expected %s, got %s", 
			newOwner.Name, loadedGame.Board[0].Owner.Name)
	}

	found := false
	for _, territory := range loadedGame.Players[1].Territories {
		if territory.Name == loadedGame.Board[0].Name {
			found = true
			break
		}
	}

	if loadedGame.Players[1] == newOwner && !found {
		t.Error("New owner's territory list not updated correctly")
	}
}
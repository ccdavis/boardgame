package game

import (
	"os"
	"testing"

	"github.com/boardgame/Go/parsing"
)

func TestGameLoader(t *testing.T) {
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

	if len(game.Players) == 0 {
		t.Error("Expected at least one player")
	}

	for _, territory := range game.Board {
		if territory.Name == "" {
			t.Error("Territory should have a name")
		}
		if territory.Owner == nil {
			t.Errorf("Territory %s should have an owner", territory.Name)
		}
	}
}

func TestChangeOwnership(t *testing.T) {
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

	if len(game.Players) < 5 {
		t.Fatal("Need at least 5 players for this test")
	}

	player3 := game.Players[3]
	player4 := game.Players[4]

	if len(player3.Territories) == 0 {
		t.Fatal("Player 3 should have at least one territory")
	}

	territory := player3.Territories[0]
	originalOwner := territory.Owner

	if originalOwner != player3 {
		t.Error("Territory owner should be player 3 initially")
	}

	if originalOwner.Name == player4.Name {
		t.Error("Initial owner should not be player 4")
	}

	t.Logf("Territory name: %s", territory.Name)

	ChangeOwnership(territory, player4)

	t.Logf("Territory name after ownership change: %s", territory.Name)

	if territory.Owner.Name != player4.Name {
		t.Errorf("Territory owner should be player 4 after change, got %s", territory.Owner.Name)
	}

	originalPlayer4Name := player4.Name
	player4.Name = "fake"

	if territory.Owner.Name != "fake" {
		t.Error("Territory owner name should update when player name changes")
	}

	found := false
	for _, t := range player3.Territories {
		if t == territory {
			found = true
			break
		}
	}
	if found {
		t.Error("Territory should no longer be in player 3's territories")
	}

	found = false
	for _, t := range player4.Territories {
		if t == territory {
			found = true
			break
		}
	}
	if !found {
		t.Error("Territory should be in player 4's territories")
	}

	player4.Name = originalPlayer4Name
}
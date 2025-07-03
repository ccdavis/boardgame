package main

import (
	"fmt"
	"os"

	"github.com/boardgame/Go/game"
	"github.com/boardgame/Go/parsing"
)

func gameLoader() {
	file, err := os.Open("../test_game.gdf")
	if err != nil {
		panic(fmt.Sprintf("Failed to open game file: %v", err))
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to parse game: %v", err))
	}

	g, err := game.NewGame(gameState)
	if err != nil {
		panic(fmt.Sprintf("Failed to create game: %v", err))
	}

	if len(g.Players) == 0 {
		panic("Expected at least one player")
	}

	fmt.Println("Game loader test passed")
}

func changeOwnership() {
	file, err := os.Open("../test_game.gdf")
	if err != nil {
		panic(fmt.Sprintf("Failed to open game file: %v", err))
	}
	defer file.Close()

	parser := parsing.NewGameParser(file)
	gameState, err := parser.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to parse game: %v", err))
	}

	g, err := game.NewGame(gameState)
	if err != nil {
		panic(fmt.Sprintf("Failed to create game: %v", err))
	}

	player0 := g.Players[0]
	player2 := g.Players[2]
	player3 := g.Players[3]
	player4 := g.Players[4]

	_ = player0
	_ = player2

	if len(player3.Territories) == 0 {
		panic("Player 3 should have territories")
	}

	t3 := player3.Territories[0]

	if t3.Owner != player3 {
		panic("Territory should be owned by player 3")
	}

	if t3.Owner.Name == player4.Name {
		panic("Territory should not be owned by player 4 initially")
	}

	fmt.Printf("t3.name == %s\n", t3.Name)

	game.ChangeOwnership(t3, player4)

	fmt.Printf("t3.name == %s\n", t3.Name)

	if t3.Owner.Name != player4.Name {
		panic("Territory should now be owned by player 4")
	}

	player4.Name = "fake"
	fmt.Printf("addresses p4: %p t3.owner: %p\n", player4, t3.Owner)

	fmt.Printf("t3.owner->name == %s\n", t3.Owner.Name)
	if t3.Owner.Name != "fake" {
		panic("Owner name should have changed")
	}

	if len(player3.Territories) > 0 && player3.Territories[0] == t3 {
		panic("Territory should no longer be in player 3's territories")
	}

	fmt.Println("Change ownership test passed")
}

func main() {
	gameLoader()
	changeOwnership()
}
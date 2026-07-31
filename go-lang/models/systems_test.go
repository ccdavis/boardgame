package models

import (
	"testing"
)

func TestChangeOwnership(t *testing.T) {
	// Create a test game
	game := NewGame()

	// Add players
	player1 := game.GetOrCreatePlayer("Player1")
	player2 := game.GetOrCreatePlayer("Player2")

	// Add a territory owned by player1
	err := game.AddTerritory("TestTerritory", Land, "Player1", 5)
	if err != nil {
		t.Fatalf("Failed to add territory: %v", err)
	}

	territory := game.Board["TestTerritory"]

	// Verify initial state
	if territory.Owner != player1 {
		t.Errorf("Expected territory to be owned by Player1")
	}
	if len(player1.Territories) != 1 {
		t.Errorf("Expected Player1 to have 1 territory, got %d", len(player1.Territories))
	}
	if len(player2.Territories) != 0 {
		t.Errorf("Expected Player2 to have 0 territories, got %d", len(player2.Territories))
	}

	// Change ownership
	ChangeOwnership(territory, player2)

	// Verify new state
	if territory.Owner != player2 {
		t.Errorf("Expected territory to be owned by Player2")
	}
	if len(player1.Territories) != 0 {
		t.Errorf("Expected Player1 to have 0 territories after transfer, got %d", len(player1.Territories))
	}
	if len(player2.Territories) != 1 {
		t.Errorf("Expected Player2 to have 1 territory after transfer, got %d", len(player2.Territories))
	}

	// Verify the territory in player2's list is the same one
	if player2.Territories[0] != territory {
		t.Errorf("Territory in Player2's list is not the same territory object")
	}
}

func TestMovePiece(t *testing.T) {
	// Create a test game
	game := NewGame()

	// Add territories
	game.AddTerritory("Territory1", Land, "Player1", 5)
	game.AddTerritory("Territory2", Land, "Player1", 3)

	// Add a piece template
	game.AddPieceTemplate("infantry", Land, 1, 1, 2, 3)

	// Place a piece on Territory1
	err := game.PlacePieces("Territory1", "infantry", 1)
	if err != nil {
		t.Fatalf("Failed to place piece: %v", err)
	}

	territory1 := game.Board["Territory1"]
	territory2 := game.Board["Territory2"]

	// Verify initial state
	if len(territory1.Pieces) != 1 {
		t.Fatalf("Expected Territory1 to have 1 piece, got %d", len(territory1.Pieces))
	}
	if len(territory2.Pieces) != 0 {
		t.Fatalf("Expected Territory2 to have 0 pieces, got %d", len(territory2.Pieces))
	}

	pieceID := territory1.Pieces[0]

	// Move the piece
	err = game.MovePiece(pieceID, "Territory1", "Territory2")
	if err != nil {
		t.Fatalf("Failed to move piece: %v", err)
	}

	// Verify new state
	if len(territory1.Pieces) != 0 {
		t.Errorf("Expected Territory1 to have 0 pieces after move, got %d", len(territory1.Pieces))
	}
	if len(territory2.Pieces) != 1 {
		t.Errorf("Expected Territory2 to have 1 piece after move, got %d", len(territory2.Pieces))
	}
	if territory2.Pieces[0] != pieceID {
		t.Errorf("Expected piece ID %d in Territory2, got %d", pieceID, territory2.Pieces[0])
	}
}

func TestCountPiecesByType(t *testing.T) {
	game := NewGame()

	// Add territories
	game.AddTerritory("Territory1", Land, "Player1", 5)
	game.AddTerritory("Territory2", Water, "Player1", 0)

	// Add piece templates
	game.AddPieceTemplate("infantry", Land, 1, 1, 2, 3)
	game.AddPieceTemplate("armor", Land, 2, 3, 2, 5)
	game.AddPieceTemplate("battleship", Water, 2, 4, 4, 24)

	// Place pieces
	game.PlacePieces("Territory1", "infantry", 5)
	game.PlacePieces("Territory1", "armor", 3)
	game.PlacePieces("Territory2", "battleship", 2)

	// Count pieces
	counts := game.CountPiecesByType()

	if counts["infantry"] != 5 {
		t.Errorf("Expected 5 infantry, got %d", counts["infantry"])
	}
	if counts["armor"] != 3 {
		t.Errorf("Expected 3 armor, got %d", counts["armor"])
	}
	if counts["battleship"] != 2 {
		t.Errorf("Expected 2 battleships, got %d", counts["battleship"])
	}
}

func TestGetTerritoriesByOwner(t *testing.T) {
	game := NewGame()

	// Add territories for different players
	game.AddTerritory("Territory1", Land, "Player1", 5)
	game.AddTerritory("Territory2", Land, "Player1", 3)
	game.AddTerritory("Territory3", Land, "Player2", 4)

	// Get territories for Player1
	territories := game.GetTerritoriesByOwner("Player1")
	if len(territories) != 2 {
		t.Errorf("Expected Player1 to have 2 territories, got %d", len(territories))
	}

	// Get territories for Player2
	territories = game.GetTerritoriesByOwner("Player2")
	if len(territories) != 1 {
		t.Errorf("Expected Player2 to have 1 territory, got %d", len(territories))
	}

	// Get territories for non-existent player
	territories = game.GetTerritoriesByOwner("NonExistent")
	if territories != nil {
		t.Errorf("Expected nil for non-existent player")
	}
}

// MovePiece must refuse rather than corrupt.
//
// It used to return nil for an unknown territory, and when the piece was not
// in the source territory it appended it to the destination anyway -- the same
// piece then stood in two territories at once.
func TestMovePiece_RefusesBadMoves(t *testing.T) {
	g := NewGame()
	g.GetOrCreatePlayer("A")
	g.AddTerritory("Here", Land, "A", 1)
	g.AddTerritory("There", Land, "A", 1)
	g.AddPieceTemplate("infantry", Land, 1, 1, 2, 3)
	g.PlacePieces("Here", "infantry", 1)
	id := g.Board["Here"].Pieces[0]

	if err := g.MovePiece(id, "Nowhere", "There"); err == nil {
		t.Error("moving from an unknown territory reported success")
	}
	if err := g.MovePiece(id, "Here", "Nowhere"); err == nil {
		t.Error("moving to an unknown territory reported success")
	}
	if err := g.MovePiece(id, "There", "Here"); err == nil {
		t.Error("moving a piece from a territory it is not in reported success")
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("refused moves still corrupted the board: %v", problems)
	}
	if len(g.Board["Here"].Pieces) != 1 || len(g.Board["There"].Pieces) != 0 {
		t.Error("refused moves changed piece positions")
	}
}

package models

import "fmt"

// ChangeOwnership transfers ownership of a territory from one player to another
// This properly updates both the territory's owner and the player territory lists
func ChangeOwnership(territory *Territory, newOwner *Player) {
	if territory == nil || newOwner == nil {
		return
	}

	oldOwner := territory.Owner

	// Update territory's owner
	territory.Owner = newOwner

	// Add territory to new owner's list
	newOwner.Territories = append(newOwner.Territories, territory)

	// Remove territory from old owner's list
	if oldOwner != nil {
		newTerritories := make([]*Territory, 0, len(oldOwner.Territories)-1)
		for _, t := range oldOwner.Territories {
			if t != territory {
				newTerritories = append(newTerritories, t)
			}
		}
		oldOwner.Territories = newTerritories
	}
}

// MovePiece moves a piece from one territory to another.
//
// Every failure is an error, never a silent no-op. This used to return nil for
// an unknown territory, and -- worse -- when the piece was not actually in the
// source territory it still appended it to the destination, leaving the same
// piece listed in two places. Callers all check the error; they were being
// told everything was fine.
func (g *Game) MovePiece(pieceID int, fromTerritory, toTerritory string) error {
	from, exists := g.Board[fromTerritory]
	if !exists {
		return fmt.Errorf("territory %q not found", fromTerritory)
	}

	to, exists := g.Board[toTerritory]
	if !exists {
		return fmt.Errorf("territory %q not found", toTerritory)
	}

	// Remove piece from source territory. (Capacity is len, not len-1: an
	// empty source made the old len-1 capacity negative, which panics.)
	newPieces := make([]int, 0, len(from.Pieces))
	found := false
	for _, id := range from.Pieces {
		if id == pieceID && !found {
			found = true
			continue
		}
		newPieces = append(newPieces, id)
	}
	if !found {
		return fmt.Errorf("piece %d is not in %q", pieceID, fromTerritory)
	}
	from.Pieces = newPieces

	// Add piece to destination territory
	to.Pieces = append(to.Pieces, pieceID)

	return nil
}

// GetTerritoriesByOwner returns all territories owned by a specific player
func (g *Game) GetTerritoriesByOwner(playerName string) []*Territory {
	player, exists := g.Players[playerName]
	if !exists {
		return nil
	}
	return player.Territories
}

// GetPiecesInTerritory returns all pieces in a specific territory
func (g *Game) GetPiecesInTerritory(territoryName string) []*Piece {
	territory, exists := g.Board[territoryName]
	if !exists {
		return nil
	}

	pieces := make([]*Piece, 0, len(territory.Pieces))
	for _, pieceID := range territory.Pieces {
		if piece, exists := g.Pieces[pieceID]; exists {
			pieces = append(pieces, piece)
		}
	}
	return pieces
}

// CountPiecesByType counts how many pieces of each type are in the game
func (g *Game) CountPiecesByType() map[string]int {
	counts := make(map[string]int)
	for _, piece := range g.Pieces {
		counts[piece.Name]++
	}
	return counts
}

// LoadPiece loads a piece onto a transport (container)
// Removes the piece from its territory and adds it to the transport's Holding
func (g *Game) LoadPiece(transportID, pieceID int) error {
	transport, exists := g.Pieces[transportID]
	if !exists {
		return fmt.Errorf("transport %d not found", transportID)
	}

	piece, exists := g.Pieces[pieceID]
	if !exists {
		return fmt.Errorf("piece %d not found", pieceID)
	}

	// Check if transport has capacity
	if transport.Capacity == 0 {
		return fmt.Errorf("%s cannot carry units (capacity=0)", transport.Name)
	}

	// Check if transport is full
	if len(transport.Holding) >= int(transport.Capacity) {
		return fmt.Errorf("%s is at full capacity (%d/%d)", transport.Name, len(transport.Holding), transport.Capacity)
	}

	// Check if transport can carry this piece type
	canCarry := false
	for _, allowedType := range transport.CanCarry {
		if allowedType == piece.Name {
			canCarry = true
			break
		}
	}
	if !canCarry {
		return fmt.Errorf("%s cannot carry %s units", transport.Name, piece.Name)
	}

	// Check if piece is already in a transport
	for _, otherTransport := range g.Pieces {
		for _, heldID := range otherTransport.Holding {
			if heldID == pieceID {
				return fmt.Errorf("piece %d is already loaded in another transport", pieceID)
			}
		}
	}

	// Find the piece's current territory and remove it
	var foundTerritory *Territory
	for _, territory := range g.Board {
		for i, id := range territory.Pieces {
			if id == pieceID {
				// Remove piece from this territory
				territory.Pieces = append(territory.Pieces[:i], territory.Pieces[i+1:]...)
				foundTerritory = territory
				break
			}
		}
		if foundTerritory != nil {
			break
		}
	}

	if foundTerritory == nil {
		return fmt.Errorf("piece %d not found in any territory", pieceID)
	}

	// Load the piece
	transport.Holding = append(transport.Holding, pieceID)

	return nil
}

// UnloadPiece unloads a piece from a transport to a destination territory
// Removes the piece from the transport's Holding and adds it to the destination territory
func (g *Game) UnloadPiece(transportID, pieceID int, destinationName string) error {
	transport, exists := g.Pieces[transportID]
	if !exists {
		return fmt.Errorf("transport %d not found", transportID)
	}

	destination, exists := g.Board[destinationName]
	if !exists {
		return fmt.Errorf("destination territory %s not found", destinationName)
	}

	// Find and remove the piece from Holding
	found := false
	newHolding := make([]int, 0, len(transport.Holding))
	for _, heldID := range transport.Holding {
		if heldID == pieceID && !found {
			found = true
			continue
		}
		newHolding = append(newHolding, heldID)
	}

	if !found {
		return fmt.Errorf("piece %d is not in transport %d", pieceID, transportID)
	}

	transport.Holding = newHolding

	// Add piece to destination territory
	destination.Pieces = append(destination.Pieces, pieceID)

	return nil
}

// GetTransportForPiece returns the transport ID that is carrying a piece, or -1 if not loaded
func (g *Game) GetTransportForPiece(pieceID int) int {
	for transportID, transport := range g.Pieces {
		for _, heldID := range transport.Holding {
			if heldID == pieceID {
				return transportID
			}
		}
	}
	return -1
}

// IsLoaded checks if a piece is currently loaded in a transport
func (g *Game) IsLoaded(pieceID int) bool {
	return g.GetTransportForPiece(pieceID) != -1
}

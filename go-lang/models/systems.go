package models

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

// MovePiece moves a piece from one territory to another
func (g *Game) MovePiece(pieceID int, fromTerritory, toTerritory string) error {
	from, exists := g.Board[fromTerritory]
	if !exists {
		return nil
	}

	to, exists := g.Board[toTerritory]
	if !exists {
		return nil
	}

	// Remove piece from source territory
	newPieces := make([]int, 0, len(from.Pieces)-1)
	found := false
	for _, id := range from.Pieces {
		if id == pieceID && !found {
			found = true
			continue
		}
		newPieces = append(newPieces, id)
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

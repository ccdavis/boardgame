package game

import (
	"boardgame/models"
	"fmt"
)

// BlitzMove represents a tank blitzing through territories
type BlitzMove struct {
	PieceID            int
	StartTerritory     string
	IntermediateStop   string // Hostile territory passed through (must be empty)
	FinalDestination   string
	MovementUsed       int
}

// ValidateTankBlitz checks if a tank can blitz through a territory
// Rules:
// - Unit must be a tank (or other blitz-capable unit)
// - Intermediate territory must be hostile (enemy or neutral)
// - Intermediate territory must have NO enemy units
// - Tank must have movement points (typically 2)
func ValidateTankBlitz(
	game *models.Game,
	pieceID int,
	startTerritory string,
	intermediateTerritory string,
	finalDestination string,
	playerName string,
) error {
	piece, exists := game.Pieces[pieceID]
	if !exists {
		return fmt.Errorf("piece %d not found", pieceID)
	}

	// Check if unit can blitz (only tanks in standard A&A)
	if piece.Name != "tank" && piece.Name != "armor" {
		return fmt.Errorf("only tanks can blitz, not %s", piece.Name)
	}

	// Check movement capacity (tanks have 2 movement)
	if piece.Movement < 2 {
		return fmt.Errorf("tank needs at least 2 movement to blitz")
	}

	player, exists := game.Players[playerName]
	if !exists {
		return fmt.Errorf("player %s not found", playerName)
	}

	// Validate start territory
	startTerr, exists := game.Board[startTerritory]
	if !exists {
		return fmt.Errorf("start territory %s not found", startTerritory)
	}

	// Validate intermediate territory
	intermediateTerr, exists := game.Board[intermediateTerritory]
	if !exists {
		return fmt.Errorf("intermediate territory %s not found", intermediateTerritory)
	}

	// Validate final destination
	_, exists = game.Board[finalDestination]
	if !exists {
		return fmt.Errorf("final destination %s not found", finalDestination)
	}

	// Check territories are connected: start -> intermediate
	startConnectedToIntermediate := false
	for _, neighbor := range startTerr.ConnectedTo {
		if neighbor.Name == intermediateTerritory {
			startConnectedToIntermediate = true
			break
		}
	}
	if !startConnectedToIntermediate {
		return fmt.Errorf("%s is not connected to %s", startTerritory, intermediateTerritory)
	}

	// Check territories are connected: intermediate -> final
	intermediateConnectedToFinal := false
	for _, neighbor := range intermediateTerr.ConnectedTo {
		if neighbor.Name == finalDestination {
			intermediateConnectedToFinal = true
			break
		}
	}
	if !intermediateConnectedToFinal {
		return fmt.Errorf("%s is not connected to %s", intermediateTerritory, finalDestination)
	}

	// Check intermediate territory is hostile (not owned by player)
	if intermediateTerr.Owner == player {
		return fmt.Errorf("cannot blitz through friendly territory %s", intermediateTerritory)
	}

	// Check intermediate territory has NO enemy units (this is the key blitz requirement)
	enemyUnitsInIntermediate := 0
	for range intermediateTerr.Pieces {
		enemyUnitsInIntermediate++
	}

	if enemyUnitsInIntermediate > 0 {
		return fmt.Errorf("cannot blitz through %s - contains %d enemy units", intermediateTerritory, enemyUnitsInIntermediate)
	}

	return nil
}

// ExecuteTankBlitz executes a tank blitz move
// The tank captures the intermediate territory and continues to final destination
func ExecuteTankBlitz(game *models.Game, blitzMove *BlitzMove, playerName string) error {
	err := ValidateTankBlitz(
		game,
		blitzMove.PieceID,
		blitzMove.StartTerritory,
		blitzMove.IntermediateStop,
		blitzMove.FinalDestination,
		playerName,
	)
	if err != nil {
		return err
	}

	player := game.Players[playerName]
	intermediateTerr := game.Board[blitzMove.IntermediateStop]

	// Capture intermediate territory (since it's unoccupied)
	intermediateTerr.Owner = player

	// Move tank to final destination
	// (In a full implementation, this would interact with the movement system)
	// For now, we just validate the move is legal

	blitzMove.MovementUsed = 2 // Blitz uses 2 movement

	return nil
}

// RetreatMove represents a retreat from combat
type RetreatMove struct {
	BattleLocation     string
	RetreatDestination string
	RetreatingUnits    []*models.Piece
	Round              int // Which round the retreat happened
}

// ValidateRetreat checks if attackers can retreat to a territory
// Rules:
// - Only attackers can retreat (defenders cannot)
// - Can only retreat to a friendly territory
// - Must retreat to a territory that at least one attacking unit came from
// - Cannot retreat after round 1 if amphibious assault
func ValidateRetreat(
	game *models.Game,
	battleLocation string,
	retreatDestination string,
	attackerName string,
	isAmphibious bool,
	currentRound int,
) error {
	// Check battle location exists
	_, exists := game.Board[battleLocation]
	if !exists {
		return fmt.Errorf("battle location %s not found", battleLocation)
	}

	// Check retreat destination exists
	retreatTerr, exists := game.Board[retreatDestination]
	if !exists {
		return fmt.Errorf("retreat destination %s not found", retreatDestination)
	}

	attacker, exists := game.Players[attackerName]
	if !exists {
		return fmt.Errorf("attacker %s not found", attackerName)
	}

	// Must retreat to friendly territory
	if retreatTerr.Owner != attacker {
		return fmt.Errorf("can only retreat to friendly territory, %s is not friendly", retreatDestination)
	}

	// Cannot retreat from amphibious assault
	if isAmphibious {
		return fmt.Errorf("cannot retreat from amphibious assault")
	}

	// In a full implementation, would verify retreat destination is where units came from
	// For now, just check it's adjacent to battle location
	battleTerr := game.Board[battleLocation]
	isAdjacent := false
	for _, neighbor := range battleTerr.ConnectedTo {
		if neighbor.Name == retreatDestination {
			isAdjacent = true
			break
		}
	}

	if !isAdjacent {
		return fmt.Errorf("retreat destination %s is not adjacent to battle location %s", retreatDestination, battleLocation)
	}

	return nil
}

// ExecuteRetreat moves attacking units from battle to retreat destination
func ExecuteRetreat(
	game *models.Game,
	battle *Battle,
	retreatDestination string,
	attackerName string,
) (*RetreatMove, error) {
	err := ValidateRetreat(game, battle.Location, retreatDestination, attackerName, battle.Type == AmphibiousAssault, battle.Round)
	if err != nil {
		return nil, err
	}

	retreatMove := &RetreatMove{
		BattleLocation:     battle.Location,
		RetreatDestination: retreatDestination,
		RetreatingUnits:    make([]*models.Piece, len(battle.Attackers)),
		Round:              battle.Round,
	}

	// Copy retreating units
	copy(retreatMove.RetreatingUnits, battle.Attackers)

	// In a full implementation, would move pieces on the board
	// For now, just record the retreat

	return retreatMove, nil
}

// CanBlitz checks if a piece is capable of blitzing
func CanBlitz(piece *models.Piece) bool {
	return piece.Name == "tank" || piece.Name == "armor"
}

// GetBlitzCapableUnits returns all units that can blitz from a list
func GetBlitzCapableUnits(units []*models.Piece) []*models.Piece {
	blitzers := make([]*models.Piece, 0)
	for _, unit := range units {
		if CanBlitz(unit) {
			blitzers = append(blitzers, unit)
		}
	}
	return blitzers
}

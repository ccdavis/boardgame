package game

import (
	"boardgame/models"
	"fmt"
)

// CarrierLaunchRecord tracks which fighters were launched from which carriers
type CarrierLaunchRecord struct {
	CarrierPieceID int
	FighterPieceID int
	LaunchLocation string
}

// AirUnitMovement tracks air unit movement and landing requirements
type AirUnitMovement struct {
	PieceID          int
	StartTerritory   string
	PlannedRoute     []string
	LandingZone      string
	MovementUsed     int
	LaunchedFromCarrier bool
	SourceCarrierID  int
}

// ValidateFighterLaunch checks if a fighter can be launched from a carrier
func ValidateFighterLaunch(game *models.Game, carrierPieceID, fighterPieceID int, territory string) error {
	carrier, exists := game.Pieces[carrierPieceID]
	if !exists {
		return fmt.Errorf("carrier piece %d not found", carrierPieceID)
	}

	if carrier.Name != "carrier" {
		return fmt.Errorf("piece %d is not a carrier", carrierPieceID)
	}

	fighter, exists := game.Pieces[fighterPieceID]
	if !exists {
		return fmt.Errorf("fighter piece %d not found", fighterPieceID)
	}

	if fighter.Name != "fighter" {
		return fmt.Errorf("piece %d is not a fighter", fighterPieceID)
	}

	// Check if fighter is on the carrier (in Holding array)
	fighterOnCarrier := false
	for _, heldPieceID := range carrier.Holding {
		if heldPieceID == fighterPieceID {
			fighterOnCarrier = true
			break
		}
	}

	if !fighterOnCarrier {
		return fmt.Errorf("fighter %d is not on carrier %d", fighterPieceID, carrierPieceID)
	}

	return nil
}

// LaunchFighter launches a fighter from a carrier
// The fighter is removed from the carrier's Holding and placed in the territory
func LaunchFighter(game *models.Game, carrierPieceID, fighterPieceID int, territoryName string) (*CarrierLaunchRecord, error) {
	err := ValidateFighterLaunch(game, carrierPieceID, fighterPieceID, territoryName)
	if err != nil {
		return nil, err
	}

	carrier := game.Pieces[carrierPieceID]

	// Remove fighter from carrier's Holding
	newHolding := make([]int, 0, len(carrier.Holding)-1)
	for _, heldPieceID := range carrier.Holding {
		if heldPieceID != fighterPieceID {
			newHolding = append(newHolding, heldPieceID)
		}
	}
	carrier.Holding = newHolding

	// Add fighter to territory's pieces (if not already there)
	territory, exists := game.Board[territoryName]
	if !exists {
		return nil, fmt.Errorf("territory %s not found", territoryName)
	}

	// Check if fighter is already in territory pieces
	alreadyInTerritory := false
	for _, pieceID := range territory.Pieces {
		if pieceID == fighterPieceID {
			alreadyInTerritory = true
			break
		}
	}

	if !alreadyInTerritory {
		territory.Pieces = append(territory.Pieces, fighterPieceID)
	}

	record := &CarrierLaunchRecord{
		CarrierPieceID: carrierPieceID,
		FighterPieceID: fighterPieceID,
		LaunchLocation: territoryName,
	}

	return record, nil
}

// LandFighterOnCarrier lands a fighter back onto a carrier
func LandFighterOnCarrier(game *models.Game, fighterPieceID, carrierPieceID int, territoryName string) error {
	fighter, exists := game.Pieces[fighterPieceID]
	if !exists {
		return fmt.Errorf("fighter piece %d not found", fighterPieceID)
	}

	if fighter.Name != "fighter" {
		return fmt.Errorf("piece %d is not a fighter", fighterPieceID)
	}

	carrier, exists := game.Pieces[carrierPieceID]
	if !exists {
		return fmt.Errorf("carrier piece %d not found", carrierPieceID)
	}

	if carrier.Name != "carrier" {
		return fmt.Errorf("piece %d is not a carrier", carrierPieceID)
	}

	// Check if carrier has capacity (typically 2 fighters)
	if len(carrier.Holding) >= int(carrier.Capacity) {
		return fmt.Errorf("carrier %d is at full capacity (%d/%d)", carrierPieceID, len(carrier.Holding), carrier.Capacity)
	}

	// Check if both fighter and carrier are in the same territory
	territory, exists := game.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	fighterInTerritory := false
	carrierInTerritory := false

	for _, pieceID := range territory.Pieces {
		if pieceID == fighterPieceID {
			fighterInTerritory = true
		}
		if pieceID == carrierPieceID {
			carrierInTerritory = true
		}
	}

	if !fighterInTerritory {
		return fmt.Errorf("fighter %d is not in territory %s", fighterPieceID, territoryName)
	}

	if !carrierInTerritory {
		return fmt.Errorf("carrier %d is not in territory %s", carrierPieceID, territoryName)
	}

	// Add fighter to carrier's Holding
	carrier.Holding = append(carrier.Holding, fighterPieceID)

	return nil
}

// FindAvailableLandingZones finds all valid landing zones for an air unit
// Air units can land on:
// 1. Friendly territories within movement range
// 2. Friendly carriers within movement range
func FindAvailableLandingZones(
	game *models.Game,
	pieceID int,
	currentTerritory string,
	movementRemaining int,
	playerName string,
) (territories []string, carriers []int, err error) {
	piece, exists := game.Pieces[pieceID]
	if !exists {
		return nil, nil, fmt.Errorf("piece %d not found", pieceID)
	}

	if piece.Terrain != models.Air {
		return nil, nil, fmt.Errorf("piece %d is not an air unit", pieceID)
	}

	territories = make([]string, 0)
	carriers = make([]int, 0)

	player, exists := game.Players[playerName]
	if !exists {
		return nil, nil, fmt.Errorf("player %s not found", playerName)
	}

	// Find all territories within movement range
	reachable := findReachableTerritories(game, currentTerritory, movementRemaining)

	// Check each reachable territory
	for _, terrName := range reachable {
		terr := game.Board[terrName]

		// Can land on friendly territories
		if terr.Owner == player {
			territories = append(territories, terrName)
		}

		// Check for friendly carriers with capacity
		for _, carrierPieceID := range terr.Pieces {
			carrierPiece := game.Pieces[carrierPieceID]
			if carrierPiece.Name == "carrier" {
				// Check if carrier is friendly (this is simplified - in full game would check ownership)
				// For now, assume all carriers in friendly territories are friendly
				if terr.Owner == player && len(carrierPiece.Holding) < int(carrierPiece.Capacity) {
					carriers = append(carriers, carrierPieceID)
				}
			}
		}
	}

	return territories, carriers, nil
}

// findReachableTerritories performs BFS to find all territories within movement range
func findReachableTerritories(game *models.Game, startTerritory string, maxMovement int) []string {
	if maxMovement <= 0 {
		return []string{startTerritory}
	}

	visited := make(map[string]bool)
	reachable := make([]string, 0)
	queue := []struct {
		territory string
		distance  int
	}{{startTerritory, 0}}

	visited[startTerritory] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		reachable = append(reachable, current.territory)

		// If we can move further, add connected territories
		if current.distance < maxMovement {
			territory := game.Board[current.territory]
			for _, neighbor := range territory.ConnectedTo {
				if !visited[neighbor.Name] {
					visited[neighbor.Name] = true
					queue = append(queue, struct {
						territory string
						distance  int
					}{neighbor.Name, current.distance + 1})
				}
			}
		}
	}

	return reachable
}

// ValidateAirUnitLanding checks if an air unit has a valid landing zone
func ValidateAirUnitLanding(
	game *models.Game,
	pieceID int,
	startTerritory string,
	destinationTerritory string,
	movementUsed int,
	playerName string,
) error {
	piece, exists := game.Pieces[pieceID]
	if !exists {
		return fmt.Errorf("piece %d not found", pieceID)
	}

	if piece.Terrain != models.Air {
		return fmt.Errorf("piece %d is not an air unit", pieceID)
	}

	// Check if destination is within movement range
	movementRemaining := int(piece.Movement) - movementUsed
	if movementRemaining < 0 {
		return fmt.Errorf("air unit has no movement remaining")
	}

	territories, carriers, err := FindAvailableLandingZones(game, pieceID, startTerritory, movementRemaining, playerName)
	if err != nil {
		return err
	}

	// Check if destination is a valid territory
	for _, terr := range territories {
		if terr == destinationTerritory {
			return nil // Valid landing zone
		}
	}

	// Check if there's a valid carrier in the destination
	destTerr := game.Board[destinationTerritory]
	for _, carrierID := range carriers {
		carrier := game.Pieces[carrierID]
		// Check if carrier is in destination territory
		for _, pieceInDest := range destTerr.Pieces {
			if pieceInDest == carrierID && len(carrier.Holding) < int(carrier.Capacity) {
				return nil // Valid carrier landing
			}
		}
	}

	return fmt.Errorf("no valid landing zone for air unit at %s (movement: %d)", destinationTerritory, movementRemaining)
}

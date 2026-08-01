package game

// A human player's amphibious assault.
//
// The NPC runs assaults through its PlanBook; a human needs the same mechanics
// without the multi-turn campaign machinery. A PlannedLanding is booked during
// the combat-move phase -- cargo aboard transports, aimed at a hostile shore --
// and comes ashore when combat moves are executed, AFTER the fleet has sailed.
// That order matters: under the rules a transport may load, sail and land in
// the same phase, so the drop zone is only known once moves have run; and the
// troops must join the battle as registered attackers, which is exactly what
// registerLandedAttacker does. Unloading into hostile territory immediately at
// click time would instead leave the troops standing among the DEFENDERS of
// the next battle there.

import (
	"fmt"

	"boardgame/models"
)

// PlannedLanding is one booked assault: these cargo pieces, onto that shore.
type PlannedLanding struct {
	CargoIDs []int
	Target   string
}

// PlanLanding books an amphibious assault for the current player's cargo.
func (gc *GameController) PlanLanding(cargoIDs []int, targetName string) error {
	if gc.Game.CurrentPhase != models.CombatMovePhase {
		return fmt.Errorf("an assault landing is a combat move; plan it during the Combat Move phase")
	}
	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}
	target := gc.Game.Board[targetName]
	if target == nil {
		return fmt.Errorf("territory %s not found", targetName)
	}
	if target.Terrain == models.Water {
		return fmt.Errorf("%s is water; troops land on shores", targetName)
	}
	if target.Owner != nil && (target.Owner == player || areAllies(target.Owner, player)) {
		return fmt.Errorf("%s is friendly ground; unload there without a fight", targetName)
	}
	// A landing on a neutral obeys the same rules as an overland invasion.
	if !canAttackNeutral(target, player) {
		if target.NeutralType == models.StrictNeutral {
			return fmt.Errorf("violating %s, a strict neutral, costs %d IPCs paid to the bank; %s has only %d",
				targetName, NeutralViolationCost, player.Name, player.IPCs)
		}
		return fmt.Errorf("%s may not attack %s, a %s territory",
			player.Name, targetName, target.NeutralType)
	}
	if len(cargoIDs) == 0 {
		return fmt.Errorf("no units to land")
	}

	for _, cargoID := range cargoIDs {
		piece, ok := gc.Game.Pieces[cargoID]
		if !ok {
			return fmt.Errorf("piece %d not found", cargoID)
		}
		if piece.Owner != player {
			return fmt.Errorf("%s %d is not yours", piece.Name, cargoID)
		}
		transportID := gc.Game.GetTransportForPiece(cargoID)
		if transportID == -1 {
			return fmt.Errorf("%s %d is not aboard a transport", piece.Name, cargoID)
		}
		if gc.landingBooked(cargoID) {
			return fmt.Errorf("%s %d is already booked for a landing", piece.Name, cargoID)
		}
		// The transport must end the phase beside the target: either it is
		// there now, or a planned move takes it there. The landing re-checks
		// at execution -- a transport sunk or stopped short strands its cargo
		// aboard rather than teleporting it ashore.
		zone := territoryOf(gc.Game, transportID)
		reaches := zone != nil && areConnected(zone, target)
		if !reaches {
			for _, move := range gc.MoveTracker.Moves {
				if move.PieceID != transportID {
					continue
				}
				if dest := gc.Game.Board[move.To]; dest != nil && areConnected(dest, target) {
					reaches = true
					break
				}
			}
		}
		if !reaches {
			return fmt.Errorf("the transport carrying %s %d will not be adjacent to %s",
				piece.Name, cargoID, targetName)
		}
	}

	gc.PlannedLandings = append(gc.PlannedLandings, &PlannedLanding{
		CargoIDs: append([]int{}, cargoIDs...),
		Target:   targetName,
	})
	return nil
}

// CancelLanding removes a piece from its booked landing.
func (gc *GameController) CancelLanding(pieceID int) error {
	for i, landing := range gc.PlannedLandings {
		for j, id := range landing.CargoIDs {
			if id != pieceID {
				continue
			}
			landing.CargoIDs = append(landing.CargoIDs[:j], landing.CargoIDs[j+1:]...)
			if len(landing.CargoIDs) == 0 {
				gc.PlannedLandings = append(gc.PlannedLandings[:i], gc.PlannedLandings[i+1:]...)
			}
			return nil
		}
	}
	return fmt.Errorf("piece %d has no landing booked", pieceID)
}

// GetPlannedLandings lists the bookings, for display.
func (gc *GameController) GetPlannedLandings() []*PlannedLanding {
	return gc.PlannedLandings
}

func (gc *GameController) landingBooked(pieceID int) bool {
	for _, landing := range gc.PlannedLandings {
		for _, id := range landing.CargoIDs {
			if id == pieceID {
				return true
			}
		}
	}
	return false
}

// executePlannedLandings puts booked troops ashore. Called from
// ExecuteCombatMoves after the ordinary moves have run, so the transports
// booked to sail this phase have actually arrived in their drop zones.
func (gc *GameController) executePlannedLandings(player *models.Player) {
	if len(gc.PlannedLandings) == 0 {
		return
	}

	// Which drop zones actually landed troops on which targets: bombardment
	// support comes from those zones, attached once per zone after the waves.
	landedFrom := make(map[string]map[string]bool)

	for _, landing := range gc.PlannedLandings {
		target := gc.Game.Board[landing.Target]
		if target == nil {
			continue
		}
		// Ownership may have changed since booking; never land against an ally.
		if target.Owner != nil && target.Owner != player && areAllies(target.Owner, player) {
			continue
		}
		for _, cargoID := range landing.CargoIDs {
			transportID := gc.Game.GetTransportForPiece(cargoID)
			if transportID == -1 {
				continue // sunk, or unloaded by other means
			}
			zone := territoryOf(gc.Game, transportID)
			if zone == nil || !areConnected(zone, target) {
				continue // the transport never arrived; the cargo stays aboard
			}
			if err := gc.Game.UnloadPiece(transportID, cargoID, landing.Target); err != nil {
				continue
			}
			gc.registerLandedAttacker(landing.Target, cargoID, player.Name, "")
			if landedFrom[landing.Target] == nil {
				landedFrom[landing.Target] = make(map[string]bool)
			}
			landedFrom[landing.Target][zone.Name] = true
		}
	}

	for targetName, zones := range landedFrom {
		for zone := range zones {
			gc.attachShoreBombardmentAt(targetName, zone, player.Name)
		}
	}

	gc.PlannedLandings = nil
}

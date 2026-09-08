package game

import (
	"fmt"
	"sort"

	"boardgame/models"
)

// Strategic bombing raids.
//
// A bomber sent against an enemy industrial complex does not fight the
// garrison: it flies to the territory, the anti-aircraft defence fires, and
// each surviving bomber rolls a die of damage against the complex (capped at
// twice the territory's production). The damage costs the owner one IPC a
// point to repair and cuts the complex's output until it is. The bombers stay
// over the target until noncombat movement, when they must fly home.
//
// The dice and damage mechanics already existed (ResolveStrategicBombing,
// ApplyICDamage); what was missing was a way to ORDER a raid. Neither the
// computer players nor the browser could, so no factory was ever bombed.

// Raid is a bombing raid booked against one territory.
type Raid struct {
	Target     string
	AttackerID string
	BomberIDs  []int
}

// RaidResult reports what a raid did.
type RaidResult struct {
	Target          string
	Bombers         int
	BombersLost     int
	Damage          int // damage actually applied
	DamageRolled    int // before the cap
	AAHits          []Hit
	DamageRolls     []int
	LostBomberNames []string
}

// PlanBombingRaid books a bomber to raid an enemy industrial complex. It is a
// combat move: the bomber must reach the target with movement to spare for
// the flight home, and the target must be enemy-held land with a complex.
func (gc *GameController) PlanBombingRaid(pieceID int, from, to string) error {
	if gc.Game.CurrentPhase != models.CombatMovePhase {
		return fmt.Errorf("a bombing raid is a combat move; plan it during the Combat Move phase")
	}
	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}
	piece, ok := gc.Game.Pieces[pieceID]
	if !ok {
		return fmt.Errorf("piece %d not found", pieceID)
	}
	if !gc.Game.Units().For(piece).CanBomb {
		return fmt.Errorf("%s cannot fly a bombing raid; only bombers do", piece.Name)
	}
	target := gc.Game.Board[to]
	if target == nil {
		return fmt.Errorf("territory %s not found", to)
	}
	if target.Terrain != models.Land {
		return fmt.Errorf("%s is not land; raids hit industrial complexes", to)
	}
	if target.Owner == nil || target.Owner == player || areAllies(target.Owner, player) {
		return fmt.Errorf("%s is friendly; raids hit enemy industrial complexes", to)
	}
	if !hasProduction(gc.Game, target) {
		return fmt.Errorf("%s has no industrial complex to bomb", to)
	}
	if !canAttackNeutral(target, player) {
		return fmt.Errorf("%s may not attack %s", player.Name, to)
	}
	if err := ValidateMovement(gc.Game, pieceID, from, to, CombatMove); err != nil {
		return err
	}
	distance, _, err := CalculateMovementPathForPiece(gc.Game, piece, from, to, player, CombatMove)
	if err != nil {
		return fmt.Errorf("cannot fly %s from %s to %s: %v", piece.Name, from, to, err)
	}
	remaining := gc.MoveTracker.Remaining(pieceID, int(piece.Movement))
	if distance > remaining {
		return fmt.Errorf("%s cannot reach %s from %s (distance=%d, movement left=%d)",
			piece.Name, to, from, distance, remaining)
	}
	if err := gc.MoveTracker.AddMoveWithCost(pieceID, from, to, CombatMove, distance); err != nil {
		return err
	}
	gc.MoveTracker.Moves[len(gc.MoveTracker.Moves)-1].Bombing = true
	return nil
}

// bookRaid records a bomber's arrival over its target, at move execution.
func (gc *GameController) bookRaid(move *Move, player *models.Player) {
	if gc.PendingRaids == nil {
		gc.PendingRaids = make(map[string]*Raid)
	}
	raid, ok := gc.PendingRaids[move.To]
	if !ok {
		raid = &Raid{Target: move.To, AttackerID: player.Name}
		gc.PendingRaids[move.To] = raid
	}
	raid.BomberIDs = append(raid.BomberIDs, move.PieceID)
}

// RaidOrder lists the pending raids in a fixed order.
func (gc *GameController) RaidOrder() []string {
	names := make([]string, 0, len(gc.PendingRaids))
	for name := range gc.PendingRaids {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ResolveRaid flies a booked raid: anti-aircraft fire, then damage.
func (gc *GameController) ResolveRaid(targetName string, dice *DiceRoller) (*RaidResult, error) {
	raid, ok := gc.PendingRaids[targetName]
	if !ok {
		return nil, fmt.Errorf("no raid pending against %s", targetName)
	}
	if dice == nil {
		dice = gc.battleDice()
	}
	target := gc.Game.Board[targetName]

	bombers := make([]*models.Piece, 0, len(raid.BomberIDs))
	for _, id := range raid.BomberIDs {
		if piece, alive := gc.Game.Pieces[id]; alive {
			bombers = append(bombers, piece)
		}
	}
	result := &RaidResult{Target: targetName, Bombers: len(bombers)}
	delete(gc.PendingRaids, targetName)
	if len(bombers) == 0 || target == nil {
		return result, nil
	}

	outcome, err := ResolveStrategicBombing(bombers, dice)
	if err != nil {
		return nil, err
	}
	result.AAHits = outcome.AAHits
	result.DamageRolls = outcome.DamageRolls
	result.DamageRolled = outcome.TotalDamage
	result.BombersLost = outcome.BombersDestroyed
	for i := 0; i < outcome.BombersDestroyed && i < len(bombers); i++ {
		result.LostBomberNames = append(result.LostBomberNames, bombers[i].Name)
		gc.removePieceFromBoard(bombers[i], targetName)
	}
	// A complex that has already changed hands this phase is the raider's
	// own now; the bombs still fell, so the damage still applies.
	result.Damage = ApplyICDamage(target, outcome.TotalDamage)
	return result, nil
}

// canLandAfter reports whether an aircraft that ends a combat move in `at`
// with `remaining` movement could reach somewhere legal to land: friendly
// ground, or a sea zone with a free carrier seat. Used before committing a
// plane to a distant strike so it is not traded for nothing.
func (gc *GameController) canLandAfter(aircraft *models.Piece, at *models.Territory, remaining int, player *models.Player) bool {
	if remaining <= 0 {
		return false
	}
	seen := map[string]bool{at.Name: true}
	frontier := []*models.Territory{at}
	for depth := 1; depth <= remaining && len(frontier) > 0; depth++ {
		var next []*models.Territory
		for _, here := range frontier {
			for _, n := range here.ConnectedTo {
				if seen[n.Name] {
					continue
				}
				seen[n.Name] = true
				if n.Terrain == models.Water {
					if gc.CarrierSlotFree(aircraft, n, player) {
						return true
					}
				} else if n.Owner == player || areAllies(n.Owner, player) {
					return true
				}
				next = append(next, n)
			}
		}
		frontier = next
	}
	return false
}

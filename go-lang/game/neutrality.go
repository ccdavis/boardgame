package game

import (
	"fmt"

	"boardgame/models"
)

// The rules and the arithmetic of neutral countries, in one place.
//
// Rules: who may attack or peacefully activate which kind of neutral; what a
// neutral does when invaded (mobilise a garrison); what violating a strict
// neutral costs (a toll, and every other strict neutral turning hostile).
//
// Arithmetic: what the AI needs to price an attack on a neutral honestly --
// the garrison it will raise, and the production the chain would hand the
// enemy. These used to be spread across movement, the controller and the AI,
// with the garrison size computed four separate times.

// NeutralViolationCost is what an attacker pays the bank to violate a strict
// neutral's territory. Paid once per neutral violated, when the first attacker
// crosses the border.
const NeutralViolationCost = 3

// neutralGarrisonSize is how many defenders a neutral mobilises when invaded:
// one per point of production, and never fewer than one.
func neutralGarrisonSize(territory *models.Territory) int {
	if territory == nil {
		return 0
	}
	if territory.Production < 1 {
		return 1
	}
	return territory.Production
}

// isUnclaimedNeutral reports whether a territory is neutral land still under
// its own flag -- the kind the neutrality rules below apply to.
func isUnclaimedNeutral(territory *models.Territory) bool {
	return territory != nil && territory.Owner != nil && territory.Owner.Name == "Neutral" &&
		territory.Terrain == models.Land && territory.NeutralType != models.NotNeutral
}

// canAttackNeutral checks if a player can attack a neutral territory
func canAttackNeutral(territory *models.Territory, attacker *models.Player) bool {
	// Not a neutral territory
	if territory.Owner.Name != "Neutral" {
		return true // Normal attack rules apply
	}

	// Owned by the Neutral power but not a declared neutral country: open
	// water, in practice -- every ocean zone belongs to "Neutral" and is
	// NotNeutral. Falling through to the final `false` here forbade every
	// naval battle in a Neutral-flagged sea zone, which is why fleets never
	// fought in open ocean.
	if territory.NeutralType == models.NotNeutral {
		return true
	}

	// Violating a strict neutral is allowed but not free: it costs 3 IPCs paid
	// to the bank, the neutral raises a defending garrison, and every other
	// strict neutral turns hostile. The attack was previously forbidden
	// outright, which also made the chain-reaction rule unreachable dead code.
	if territory.NeutralType == models.StrictNeutral {
		return attacker.IPCs >= NeutralViolationCost
	}

	// Pro-Allied neutrals can be attacked by Axis, but they will defend
	if territory.NeutralType == models.ProAlliedNeutral {
		return attacker.Side == "Axis" // Only Axis can attack pro-Allied neutrals
	}

	// Pro-Axis neutrals can be attacked by Allies
	if territory.NeutralType == models.ProAxisNeutral {
		return attacker.Side == "Allies" // Only Allies can attack pro-Axis neutrals
	}

	return false
}

// CanAttackNeutral is the exported face of canAttackNeutral, so the web layer
// offers only shores the rules actually allow assaulting.
func CanAttackNeutral(territory *models.Territory, attacker *models.Player) bool {
	return canAttackNeutral(territory, attacker)
}

// bookedNeutralViolations lists the strict neutrals the current player has
// already booked attacks against this phase -- planned combat moves and
// booked landings both count.
func (gc *GameController) bookedNeutralViolations() map[string]bool {
	booked := make(map[string]bool)
	note := func(name string) {
		territory := gc.Game.Board[name]
		if territory != nil && isUnclaimedNeutral(territory) &&
			territory.NeutralType == models.StrictNeutral {
			booked[name] = true
		}
	}
	for _, move := range gc.MoveTracker.GetMovesByType(CombatMove) {
		note(move.To)
	}
	for _, landing := range gc.PlannedLandings {
		note(landing.Target)
	}
	return booked
}

// checkNeutralTollFunds refuses a NEW strict-neutral violation the treasury
// cannot cover on top of the violations already booked this phase. Each
// individual attack used to pass its own 3-IPC check, so two violations
// booked with 3 IPCs in hand executed both and the bank was quietly shorted
// -- the treasury floors at zero rather than overdrawing.
func (gc *GameController) checkNeutralTollFunds(target *models.Territory, player *models.Player) error {
	if target == nil || !isUnclaimedNeutral(target) ||
		target.NeutralType != models.StrictNeutral {
		return nil
	}
	booked := gc.bookedNeutralViolations()
	if booked[target.Name] {
		return nil // this violation is already priced in
	}
	needed := (len(booked) + 1) * NeutralViolationCost
	if player.IPCs < needed {
		return fmt.Errorf("violating %s needs %d IPCs -- %d for this toll and %d already committed to other neutrals -- and %s has only %d",
			target.Name, needed, NeutralViolationCost,
			len(booked)*NeutralViolationCost, player.Name, player.IPCs)
	}
	return nil
}

// CheckNeutralTollFunds is the exported form of checkNeutralTollFunds, for
// the web layer to light only the neutrals a player can afford to violate.
func (gc *GameController) CheckNeutralTollFunds(target *models.Territory, player *models.Player) error {
	return gc.checkNeutralTollFunds(target, player)
}

// CanActivateNeutral checks if a player can peacefully activate a neutral
// territory during noncombat move phase. Exported so the web layer can tell
// the client which neighbours are genuinely enterable, instead of guessing.
func CanActivateNeutral(territory *models.Territory, activator *models.Player) bool {
	// Not a neutral territory
	if territory.Owner.Name != "Neutral" {
		return false
	}

	// Cannot activate strict neutrals
	if territory.NeutralType == models.StrictNeutral {
		return false
	}

	// Pro-Allied neutrals can be activated by Allied powers
	if territory.NeutralType == models.ProAlliedNeutral {
		return activator.Side == "Allies"
	}

	// Pro-Axis neutrals can be activated by Axis powers
	if territory.NeutralType == models.ProAxisNeutral {
		return activator.Side == "Axis"
	}

	return false
}

// violateNeutral applies the price of attacking a neutral country: when the
// first attacker crosses the border, the country mobilises its garrison, and
// violating a *strict* neutral additionally costs the attacker the toll.
// Returns whether a strict neutral was violated, so the caller can rouse the
// rest.
//
// Idempotent within an attack: once the country has defenders under its own
// flag, a second attacker (or a landing joining an overland attack) neither
// re-levies the toll nor doubles the garrison. Shared by overland movement
// and amphibious landings -- the sea route used to dodge the entire price.
func (gc *GameController) violateNeutral(territory *models.Territory, attacker *models.Player) bool {
	if !isUnclaimedNeutral(territory) {
		return false
	}

	// Already mobilised: an earlier attacker paid the price this fight.
	for _, id := range territory.Pieces {
		if piece := gc.Game.Pieces[id]; piece != nil && piece.Owner == territory.Owner {
			return false
		}
	}

	strict := territory.NeutralType == models.StrictNeutral
	if strict && attacker != nil {
		attacker.IPCs -= NeutralViolationCost
		if attacker.IPCs < 0 {
			attacker.IPCs = 0 // gated at planning time; never overdraw
		}
	}

	if defender := bestDefenderName(gc.Game); defender != "" {
		gc.Game.PlacePieces(territory.Name, defender, neutralGarrisonSize(territory))
	}
	return strict
}

// TriggerStrictNeutralChainReaction converts all strict neutrals to be hostile
// to the attacker. This is triggered when any strict neutral is attacked.
func (gc *GameController) TriggerStrictNeutralChainReaction(attacker *models.Player) {
	// The neutrals join the attacker's enemies: the first opposing power in
	// turn order takes custody. Turn order rather than the players map (which
	// iterates differently on every run), and no hardcoded list of "major"
	// powers -- the board decides who plays.
	var enemyPower *models.Player
	for _, name := range gc.Game.PlayerOrder {
		player := gc.Game.Players[name]
		if player == nil || !player.TakesTurns || player.Side == "" {
			continue
		}
		if player.Side != attacker.Side {
			enemyPower = player
			break
		}
	}
	if enemyPower == nil {
		return
	}

	// Convert all strict neutrals to hostile (give them to the enemy power
	// with defenders), in a fixed order for replayability. Neutrals currently
	// being fought over are left out: the violated country is defending itself
	// under its own flag, and flipping its ownership mid-battle would hand the
	// battle a different defender than the one it was declared against.
	defender := bestDefenderName(gc.Game)
	for _, name := range sortedTerritoryNames(gc.Game) {
		territory := gc.Game.Board[name]
		if _, underAttack := gc.PendingBattles[name]; underAttack {
			continue
		}
		if territory.Owner.Name == "Neutral" && territory.NeutralType == models.StrictNeutral {
			models.ChangeOwnership(territory, enemyPower)

			// The defecting country arrives with its garrison already raised.
			if defender != "" {
				gc.Game.PlacePieces(territory.Name, defender, neutralGarrisonSize(territory))
			}
		}
	}
}

// ActivateNeutralTerritory peacefully transfers a pro-Allied or pro-Axis
// neutral to the activating power during noncombat move.
func (gc *GameController) ActivateNeutralTerritory(territoryName, activatorName string) error {
	territory, exists := gc.Game.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	activator, exists := gc.Game.Players[activatorName]
	if !exists {
		return fmt.Errorf("player %s not found", activatorName)
	}

	// Verify this is a neutral territory that can be activated, by this side.
	if !CanActivateNeutral(territory, activator) {
		return fmt.Errorf("%s cannot peacefully activate %s", activatorName, territoryName)
	}

	// Transfer ownership
	models.ChangeOwnership(territory, activator)

	// The activated country brings its standing garrison with it.
	if defender := bestDefenderName(gc.Game); defender != "" {
		gc.Game.PlacePieces(territory.Name, defender, neutralGarrisonSize(territory))
	}

	return nil
}

// --- What the AI needs to price a neutral honestly ---

// latentDefenders returns the garrison a neutral territory WILL raise when
// attacked, as phantom pieces for the success estimator.
//
// A neutral has no standing army until the first attacker crosses the border,
// so estimating against what is visibly there priced Turkey as a walkover --
// and the attack then met four mobilised infantry. The estimator must fight
// the country that will exist, not the one on the board.
func latentDefenders(g *models.Game, territory *models.Territory) []*models.Piece {
	if !isUnclaimedNeutral(territory) {
		return nil
	}
	for _, id := range territory.Pieces {
		if piece := g.Pieces[id]; piece != nil && piece.Owner == territory.Owner {
			return nil // already mobilised; the real garrison is on the board
		}
	}

	template, ok := g.GlobalPieceTemplates[bestDefenderName(g)]
	if !ok {
		return nil
	}
	phantoms := make([]*models.Piece, neutralGarrisonSize(territory))
	for i := range phantoms {
		phantoms[i] = &models.Piece{
			Name: template.Name, Terrain: template.Terrain,
			Attack: template.Attack, Defend: template.Defend, Cost: template.Cost,
		}
	}
	return phantoms
}

// expectedDefenders is what an attack on a territory would actually fight:
// the pieces present, plus the garrison a neutral would mobilise.
func expectedDefenders(g *models.Game, territory *models.Territory) []*models.Piece {
	defenders := g.GetPiecesInTerritory(territory.Name)
	return append(defenders, latentDefenders(g, territory)...)
}

// strictChainCost is the production the OTHER strict neutrals would hand the
// enemy side if this one were violated -- the diplomatic price of the attack.
// Once the chain has already fired there are no unflipped strict neutrals
// left and the price is zero.
func strictChainCost(g *models.Game, exclude string) int {
	cost := 0
	for _, name := range sortedTerritoryNames(g) {
		if name == exclude {
			continue
		}
		territory := g.Board[name]
		if territory.Owner != nil && territory.Owner.Name == "Neutral" &&
			territory.NeutralType == models.StrictNeutral {
			cost += territory.Production
		}
	}
	return cost
}

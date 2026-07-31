package game

import (
	"fmt"

	"boardgame/models"
)

// Working a plan during a turn.
//
// Two things happen in different phases. Gathering -- walking troops to the
// port and sailing shipping to meet them -- is noncombat movement, and takes as
// many turns as it takes. The assault itself is a combat move: under the rules
// a transport loads, sails and lands within one phase, so a plan that reports
// ready executes in a single turn.

// GatherForPlans moves committed units toward their staging point.
//
// Called during noncombat movement. Units already in place stay put; the rest
// walk or sail one leg closer.
func (npc *NPCAIPlayer) GatherForPlans(gc *GameController, player *models.Player, transcript *GameTranscript) int {
	if gc.Plans == nil {
		return 0
	}

	moved := 0
	for _, plan := range gc.Plans.Active(player.Name) {
		switch plan.State {
		case PlanReady:
			// Beside the target already; the landing is a combat move.
			continue
		case PlanEmbarked:
			// Loaded and under way: sail on toward the drop zone.
			moved += npc.sailConvoy(gc, player, plan, transcript)
		default:
			moved += npc.gatherTroops(gc, player, plan, transcript)
			moved += npc.gatherShipping(gc, player, plan, transcript)
			// Once everything is in place, get the troops aboard so the convoy
			// can start its crossing this turn.
			if plan.forceAssembled(gc.Game) && len(plan.Route) > 0 {
				if npc.embarkTroops(gc, player, plan, transcript) > 0 {
					plan.State = PlanEmbarked
					plan.LastProgress = gc.Game.Turn
					moved += npc.sailConvoy(gc, player, plan, transcript)
				}
			}
		}
	}
	return moved
}

// embarkTroops puts the landing force aboard at the port.
func (npc *NPCAIPlayer) embarkTroops(gc *GameController, player *models.Player, plan *AmphibiousPlan, transcript *GameTranscript) int {
	g := gc.Game
	staging := g.Board[plan.Staging]
	embark := g.Board[plan.Embark]
	if staging == nil || embark == nil {
		return 0
	}

	loaded := 0
	for _, shipID := range plan.Ships {
		ship, ok := g.Pieces[shipID]
		if !ok || !contains(embark.Pieces, shipID) {
			continue
		}
		room := int(ship.Capacity) - len(ship.Holding)
		for _, troopID := range plan.Troops {
			if room <= 0 {
				break
			}
			if !contains(staging.Pieces, troopID) || g.IsLoaded(troopID) {
				continue
			}
			if err := gc.LoadUnit(shipID, troopID); err != nil {
				continue
			}
			loaded++
			room--
		}
	}
	if loaded > 0 {
		transcript.LogAction(player.Name, fmt.Sprintf(
			"plan %d: %d troops embarked at %s, bound for %s",
			plan.ID, loaded, plan.Staging, plan.Target))
	}
	return loaded
}

// sailConvoy moves the loaded transports and their escorts one leg closer to
// the drop zone. A transport crosses two sea zones a turn, so a long crossing
// takes several turns -- during which the route is recomputed each turn in case
// an enemy fleet has closed it.
func (npc *NPCAIPlayer) sailConvoy(gc *GameController, player *models.Player, plan *AmphibiousPlan, transcript *GameTranscript) int {
	g := gc.Game
	moved := 0

	for _, id := range append(append([]int{}, plan.Ships...), plan.Escorts...) {
		piece, ok := g.Pieces[id]
		if !ok {
			continue
		}
		from := territoryOf(g, id)
		if from == nil || from.Name == plan.DropZone {
			continue
		}
		step := nextStepTowards(g, piece, from.Name, plan.DropZone, player, models.Water)
		if step == "" {
			continue
		}
		if err := gc.PlanMove(id, from.Name, step); err == nil {
			transcript.LogMove(player.Name, piece.Name, from.Name, step, "noncombat")
			moved++
		}
	}
	return moved
}

// ForceConvoysThrough moves a covered convoy into water an enemy fleet is
// holding.
//
// Called during combat movement, because entering an occupied sea zone is an
// attack. A stationed warship should slow a landing, not forbid it outright:
// with cover the convoy fights its way past, and without cover it waits for
// escorts rather than feeding transports to a destroyer.
func (npc *NPCAIPlayer) ForceConvoysThrough(gc *GameController, player *models.Player, transcript *GameTranscript) int {
	if gc.Plans == nil {
		return 0
	}
	g := gc.Game
	forced := 0

	for _, plan := range gc.Plans.Active(player.Name) {
		if plan.State != PlanEmbarked || plan.Contested == 0 {
			continue
		}
		// Only fight through if the convoy can expect to win the action.
		if plan.EscortStrength(g) < plan.WantEscort {
			continue
		}

		blocked := nextContestedLeg(g, plan, player)
		if blocked == "" {
			continue
		}

		escorts := 0
		for _, id := range plan.Escorts {
			from := territoryOf(g, id)
			if from == nil {
				continue
			}
			if err := gc.PlanMove(id, from.Name, blocked); err == nil {
				escorts++
			}
		}
		if escorts == 0 {
			continue // nothing could get there to fight; do not send the transports
		}

		for _, id := range plan.Ships {
			from := territoryOf(g, id)
			if from == nil || len(g.Pieces[id].Holding) == 0 {
				continue
			}
			if err := gc.PlanMove(id, from.Name, blocked); err == nil {
				forced++
			}
		}

		transcript.LogAction(player.Name, fmt.Sprintf(
			"plan %d: forcing a passage through %s with %d escorts",
			plan.ID, blocked, escorts))
		plan.LastProgress = g.Turn
	}
	return forced
}

// nextContestedLeg returns the enemy-held sea zone standing between the convoy
// and its destination, if the convoy is adjacent to one on its route.
func nextContestedLeg(g *models.Game, plan *AmphibiousPlan, player *models.Player) string {
	for _, id := range plan.Ships {
		ship, ok := g.Pieces[id]
		if !ok || len(ship.Holding) == 0 {
			continue
		}
		at := territoryOf(g, id)
		if at == nil {
			continue
		}
		for _, next := range at.ConnectedTo {
			if next.Terrain != models.Water || !occupiedByEnemy(g, next, player) {
				continue
			}
			// Only worth fighting for if it is actually on the way.
			if onward, _ := seaRouteCost(g, next.Name, plan.DropZone, player); len(onward) > 0 {
				return next.Name
			}
		}
	}
	return ""
}

// gatherTroops walks committed land units toward the port.
func (npc *NPCAIPlayer) gatherTroops(gc *GameController, player *models.Player, plan *AmphibiousPlan, transcript *GameTranscript) int {
	g := gc.Game
	staging := g.Board[plan.Staging]
	if staging == nil {
		return 0
	}

	moved := 0
	for _, id := range plan.Troops {
		piece, ok := g.Pieces[id]
		if !ok {
			continue
		}
		from := territoryOf(g, id)
		if from == nil || from.Name == plan.Staging {
			continue // already at the port
		}

		step := nextStepTowards(g, piece, from.Name, plan.Staging, player, models.Land)
		if step == "" {
			continue
		}
		if err := gc.PlanMove(id, from.Name, step); err == nil {
			transcript.LogMove(player.Name, piece.Name, from.Name, step, "noncombat")
			moved++
		}
	}
	return moved
}

// gatherShipping sails transports and escorts to the embarkation zone.
func (npc *NPCAIPlayer) gatherShipping(gc *GameController, player *models.Player, plan *AmphibiousPlan, transcript *GameTranscript) int {
	g := gc.Game
	moved := 0

	for _, id := range append(append([]int{}, plan.Ships...), plan.Escorts...) {
		piece, ok := g.Pieces[id]
		if !ok {
			continue
		}
		from := territoryOf(g, id)
		if from == nil || from.Name == plan.Embark {
			continue
		}

		step := nextStepTowards(g, piece, from.Name, plan.Embark, player, models.Water)
		if step == "" {
			continue
		}
		if err := gc.PlanMove(id, from.Name, step); err == nil {
			transcript.LogMove(player.Name, piece.Name, from.Name, step, "noncombat")
			moved++
		}
	}
	return moved
}

// ExecuteReadyPlans launches every plan that has its force assembled.
//
// Called during combat movement. Under the rules this is one continuous action:
// load at the port, sail to the sea zone beside the target, and land -- which
// creates an ordinary battle in the target territory.
func (npc *NPCAIPlayer) ExecuteReadyPlans(gc *GameController, player *models.Player, transcript *GameTranscript) int {
	if gc.Plans == nil {
		return 0
	}

	launched := 0
	for _, plan := range gc.Plans.Active(player.Name) {
		if plan.State != PlanReady {
			continue
		}
		if err := npc.launchAssault(gc, player, plan, transcript); err != nil {
			transcript.LogAction(player.Name,
				fmt.Sprintf("plan %d could not launch: %v", plan.ID, err))
			plan.State = PlanForming
			continue
		}
		launched++
	}
	return launched
}

// launchAssault lands a convoy that has reached the sea zone beside its target.
//
// The transports are already in the drop zone -- getting there took as many
// turns as the crossing needed -- so this is only the landing.
func (npc *NPCAIPlayer) launchAssault(gc *GameController, player *models.Player, plan *AmphibiousPlan, transcript *GameTranscript) error {
	g := gc.Game

	drop := g.Board[plan.DropZone]
	if drop == nil {
		return fmt.Errorf("drop zone %s has gone", plan.DropZone)
	}

	carried := make([]int, 0, len(plan.Ships))
	troops := 0
	for _, shipID := range plan.Ships {
		ship, ok := g.Pieces[shipID]
		if !ok || len(ship.Holding) == 0 || !contains(drop.Pieces, shipID) {
			continue
		}
		carried = append(carried, shipID)
		troops += len(ship.Holding)
	}
	if len(carried) == 0 {
		return fmt.Errorf("no loaded transport is in %s", plan.DropZone)
	}

	transcript.LogAction(player.Name, fmt.Sprintf(
		"amphibious assault on %s: %d troops landing from %s",
		plan.Target, troops, plan.DropZone))

	plan.pendingLanding = carried
	plan.LastProgress = g.Turn
	return nil
}

// LandAssaultTroops puts the troops ashore once their transports have arrived.
//
// Run after combat moves are executed, so the transports are physically in the
// drop zone. Landing in a hostile territory is an attack, and the units join
// the battle there like any other attacker.
func (gc *GameController) LandAssaultTroops(power string, transcript *GameTranscript) int {
	if gc.Plans == nil {
		return 0
	}
	g := gc.Game
	landed := 0

	for _, plan := range gc.Plans.Active(power) {
		landedHere := 0
		if len(plan.pendingLanding) == 0 {
			continue
		}
		target := g.Board[plan.Target]
		if target == nil {
			plan.pendingLanding = nil
			continue
		}
		// Never put troops ashore against an ally. Review retires a plan whose
		// target an ally has taken, but the landing must refuse on its own
		// account too -- it is the last gate before a battle is created.
		if attacker := g.Players[power]; attacker != nil &&
			target.Owner != nil && target.Owner.Name != power &&
			areAllies(target.Owner, attacker) {
			plan.pendingLanding = nil
			continue
		}

		for _, shipID := range plan.pendingLanding {
			ship, ok := g.Pieces[shipID]
			if !ok {
				continue
			}
			// Copy: unloading mutates Holding.
			cargo := append([]int{}, ship.Holding...)
			for _, troopID := range cargo {
				if err := gc.UnloadUnit(shipID, troopID, plan.Target); err != nil {
					continue
				}
				gc.registerAmphibiousAttacker(plan, troopID, target, power)
				landedHere++
			}
		}
		plan.pendingLanding = nil
		landed += landedHere

		// Per-plan count: with two landings in one turn, the second message
		// used to report the running total rather than its own troops.
		if landedHere > 0 && transcript != nil {
			transcript.LogAction(power, fmt.Sprintf(
				"%d troops landed in %s", landedHere, plan.Target))
		}
		if landedHere > 0 {
			if guns := gc.attachShoreBombardment(plan, power); guns > 0 && transcript != nil {
				transcript.LogAction(power, fmt.Sprintf(
					"%d warship(s) stand off %s to bombard %s", guns, plan.DropZone, plan.Target))
			}
		}
	}
	return landed
}

// attachShoreBombardment enrols the attacker's bombardment-capable warships in
// the drop zone as fire support for the landing battle. Returns how many.
//
// No support is attached while the drop zone itself is being fought over: a
// fleet in action cannot also bombard the shore (and per the rules, sea combat
// in the assault's sea zone forfeits the bombardment).
func (gc *GameController) attachShoreBombardment(plan *AmphibiousPlan, power string) int {
	battle, ok := gc.PendingBattles[plan.Target]
	if !ok {
		return 0 // the beach was undefended; nothing to soften up
	}
	if _, contested := gc.PendingBattles[plan.DropZone]; contested {
		return 0
	}
	drop := gc.Game.Board[plan.DropZone]
	if drop == nil {
		return 0
	}

	units := gc.Game.Units()
	for _, id := range drop.Pieces {
		ship := gc.Game.Pieces[id]
		if ship == nil || ship.Owner == nil || ship.Owner.Name != power {
			continue
		}
		if !units.For(ship).CanBombard {
			continue
		}
		battle.Bombarding = append(battle.Bombarding, ship)
	}
	return len(battle.Bombarding)
}

// registerAmphibiousAttacker enrols a landed unit in the battle for the target,
// creating the battle if this is the first attacker to arrive.
func (gc *GameController) registerAmphibiousAttacker(plan *AmphibiousPlan, pieceID int, target *models.Territory, power string) {
	if target.Owner != nil && target.Owner.Name == power {
		return // undefended and already ours; nothing to fight
	}

	battle, exists := gc.PendingBattles[plan.Target]
	if !exists {
		defender := "Neutral"
		if target.Owner != nil {
			defender = target.Owner.Name
		}
		battleType := LandBattle
		if target.Terrain == models.Water {
			battleType = SeaBattle
		}
		battle = NewBattle(plan.Target, battleType, power, defender)
		gc.PendingBattles[plan.Target] = battle
	}
	battle.AttackingPieceIDs = append(battle.AttackingPieceIDs, pieceID)
	// Each unit that comes ashore entitles one supporting warship to one
	// bombardment shot, so the battle counts its amphibious attackers.
	battle.AmphibiousUnits++
	if battle.AttackerOrigins == nil {
		battle.AttackerOrigins = make(map[int]string)
	}
	// Troops that break off a landing go back aboard conceptually; there is no
	// beach to retreat to, so their origin is the staging port.
	battle.AttackerOrigins[pieceID] = plan.Staging
}

// territoryOf finds where a piece currently is.
func territoryOf(g *models.Game, pieceID int) *models.Territory {
	for _, territory := range g.Board {
		if contains(territory.Pieces, pieceID) {
			return territory
		}
	}
	return nil
}

// nextStepTowards returns how far along the route to a goal a piece can get
// this turn, limited to one terrain type.
//
// It finds the shortest route and then walks as far down it as the piece's
// remaining movement allows. An earlier version returned the first neighbour
// BFS happened to dequeue when the goal was out of reach, which is an arbitrary
// direction rather than a step towards anything -- a transport could sit in the
// same sea zone for the whole game "moving towards" a port it never approached.
func nextStepTowards(g *models.Game, piece *models.Piece, from, goal string, player *models.Player, terrain models.TerrainType) string {
	if from == goal {
		return ""
	}
	allowance := int(piece.Movement)
	if allowance < 1 {
		return ""
	}

	start, ok := g.Board[from]
	if !ok {
		return ""
	}

	// BFS recording where each territory was reached from, so the route can be
	// reconstructed.
	cameFrom := map[string]string{from: ""}
	queue := []*models.Territory{start}
	found := false

	for len(queue) > 0 && !found {
		current := queue[0]
		queue = queue[1:]

		for _, next := range current.ConnectedTo {
			if _, seen := cameFrom[next.Name]; seen {
				continue
			}
			// The goal itself may be land while the journey is by sea, so accept
			// it whatever its terrain.
			if next.Name != goal && next.Terrain != terrain {
				continue
			}
			// A convoy under way cannot sail through an enemy fleet during
			// noncombat movement; route around it instead of stalling.
			if terrain == models.Water && next.Name != goal && player != nil &&
				occupiedByEnemy(g, next, player) {
				continue
			}
			cameFrom[next.Name] = current.Name
			if next.Name == goal {
				found = true
				break
			}
			queue = append(queue, next)
		}
	}
	if !found {
		return ""
	}

	// Walk back from the goal to build the route, then take as much of it as
	// this turn permits.
	route := []string{goal}
	for at := cameFrom[goal]; at != "" && at != from; at = cameFrom[at] {
		route = append([]string{at}, route...)
	}
	if len(route) == 0 {
		return ""
	}
	step := allowance
	if step > len(route) {
		step = len(route)
	}
	return route[step-1]
}

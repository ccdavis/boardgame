package game

import (
	"math/rand"
	"sort"

	"boardgame/models"
)

// Forming, working and executing amphibious plans.
//
// The AI consults its plan book at the start of a turn, spends part of its
// budget on what the plans still need, gathers the committed units toward the
// port during noncombat movement, and launches when a plan reports ready.

// candidate is a possible target for an amphibious operation.
type candidate struct {
	target   string
	staging  string
	embark   string
	dropZone string
	value    int
	defence  int
	crossing int // sea zones between the port and the drop zone
}

// maxCrossing is the longest voyage worth planning.
//
// A transport covers two sea zones a turn, so this is a two-turn crossing. It
// was originally four turns, and nothing ever arrived: a loaded convoy is the
// most vulnerable thing on the board, and five turns in the open gave every
// enemy fleet in the theatre a chance to find it. Short hops succeed; grand
// expeditions across the Pacific do not.
const maxCrossing = 4

// ProposePlan looks for somewhere worth invading that this power is not already
// planning against.
//
// A target qualifies if it is hostile land we cannot walk to -- every route
// runs through water -- and we hold a coastal territory with a sea path to it.
func (npc *NPCAIPlayer) ProposePlan(gc *GameController, player *models.Player) *AmphibiousPlan {
	g := gc.Game
	claimed := gc.Plans.Targets(player.Name)

	var options []candidate
	for name, territory := range g.Board {
		if territory.Terrain != models.Land || claimed[name] {
			continue
		}
		if territory.Owner == nil || territory.Owner.Name == player.Name {
			continue
		}
		if areAllies(territory.Owner, player) {
			continue
		}
		// Somewhere we can march to is not an amphibious problem.
		if reachableOverland(g, player, name) {
			continue
		}

		targetSeas := adjacentSeaZones(g, name)
		if len(targetSeas) == 0 {
			continue // landlocked and unreachable: nothing to plan
		}

		staging, embark, drop, crossing := bestApproach(g, player, name, targetSeas)
		if staging == "" || crossing > maxCrossing {
			continue
		}

		options = append(options, candidate{
			target:   name,
			staging:  staging,
			embark:   embark,
			dropZone: drop,
			crossing: crossing,
			value:    territoryValue(territory),
			defence:  defenderStrength(g, name),
		})
	}
	if len(options) == 0 {
		return nil
	}

	// Rank by what the prize is worth against what it costs to reach: a rich
	// target on the far side of the world loses to a decent one nearby, because
	// every extra sea zone is another turn the convoy spends exposed.
	score := func(c candidate) int {
		return c.value*4 - c.crossing*3 - c.defence
	}
	sort.Slice(options, func(i, j int) bool {
		if score(options[i]) != score(options[j]) {
			return score(options[i]) > score(options[j])
		}
		return options[i].target < options[j].target
	})

	// Take one of the better options rather than always the best, so two powers
	// in the same position do not make identical plans forever.
	pick := options[0]
	if len(options) > 1 && npc.rng != nil && npc.rng.Intn(4) == 0 {
		pick = options[1]
	}

	troops := troopsNeeded(pick.defence, npc.rng)
	return &AmphibiousPlan{
		Power:          player.Name,
		Target:         pick.target,
		Staging:        pick.staging,
		Embark:         pick.embark,
		DropZone:       pick.dropZone,
		State:          PlanForming,
		WantTroops:     troops,
		WantTransports: (troops + transportCapacity - 1) / transportCapacity,
		CreatedTurn:    g.Turn,
		LastProgress:   g.Turn,
	}
}

// transportCapacity is how many land units one transport is assumed to carry.
// The board declares the real figure in its Containers section; this is only
// used to size a plan, and being wrong costs an extra ship, not correctness.
const transportCapacity = 2

// troopsNeeded sizes a landing force against the defence, with a little
// variation so a power does not always commit exactly the same amount.
//
// This is the "gather more forces, or act quickly" decision: a lightly held
// island gets a small force soon, a strong one gets a build-up.
func troopsNeeded(defence int, rng *rand.Rand) int {
	needed := defence/2 + 2
	if rng != nil {
		needed += rng.Intn(3)
	}
	if needed < 2 {
		needed = 2
	}
	if needed > 8 {
		needed = 8 // beyond this the build-up never finishes
	}
	return needed
}

func territoryValue(territory *models.Territory) int {
	value := territory.Production
	if territory.IsVictoryCity {
		value += 6
	}
	return value
}

// reachableOverland reports whether a power can march to a territory from any
// land it holds, without crossing water.
func reachableOverland(g *models.Game, player *models.Player, targetName string) bool {
	seen := make(map[string]bool)
	var queue []string

	for _, held := range player.Territories {
		if held.Terrain == models.Land && !seen[held.Name] {
			seen[held.Name] = true
			queue = append(queue, held.Name)
		}
	}

	for len(queue) > 0 {
		current := g.Board[queue[0]]
		queue = queue[1:]
		if current == nil {
			continue
		}
		for _, next := range current.ConnectedTo {
			if next.Terrain != models.Land || seen[next.Name] {
				continue
			}
			if next.Name == targetName {
				return true
			}
			seen[next.Name] = true
			queue = append(queue, next.Name)
		}
	}
	return false
}

// bestApproach picks the port to sail from and the sea zone to land out of.
//
// A short sea route is not enough on its own: the port also has to be somewhere
// troops can actually get to. Optimising for distance alone chose islands --
// Japan planned to mount an invasion from the Solomon Islands, a single-
// territory island its army could never march to, so the port never filled and
// the plan sat forming forever.
func bestApproach(g *models.Game, player *models.Player, target string, targetSeas []string) (staging, embark, drop string, crossing int) {
	reachable := landmassReach(g, player)
	best := -1

	for _, held := range player.Territories {
		if held.Terrain != models.Land {
			continue
		}
		reach := reachable[held.Name]
		if reach <= 1 {
			continue // an isolated island: no army can march to this port
		}

		for _, port := range adjacentSeaZones(g, held.Name) {
			for _, landing := range targetSeas {
				route := seaRouteFor(g, port, landing, player)
				if len(route) == 0 {
					continue
				}
				// Favour ports an army can reach, then short crossings.
				score := reach*2 - len(route)
				if best == -1 || score > best {
					best = score
					staging, embark, drop, crossing = held.Name, port, landing, len(route)
				}
			}
		}
	}
	if best < 0 {
		return "", "", "", 0
	}
	return staging, embark, drop, crossing
}

// landmassReach measures, for each land territory a power holds, how much
// friendly territory is connected to it by land.
//
// This is what tells a mainland port from an isolated one. Japan planned an
// invasion staged from the Solomon Islands -- a single-territory island its army
// could never march to -- so the port never filled and the plan sat forming
// forever.
//
// It counts territory rather than the units standing on it, so a power can form
// a plan before it has an army and then build one: production follows the plan,
// which is the point of having plans at all.
func landmassReach(g *models.Game, player *models.Player) map[string]int {
	// Group the power's land territories into connected landmasses.
	component := make(map[string]int)
	sizeOf := make(map[int]int)
	next := 0

	for _, held := range player.Territories {
		if held.Terrain != models.Land || component[held.Name] != 0 {
			continue
		}
		next++
		queue := []*models.Territory{held}
		component[held.Name] = next

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			sizeOf[next]++

			for _, neighbour := range current.ConnectedTo {
				if neighbour.Terrain != models.Land || component[neighbour.Name] != 0 {
					continue
				}
				if neighbour.Owner != player && !areAllies(neighbour.Owner, player) {
					continue
				}
				component[neighbour.Name] = next
				queue = append(queue, neighbour)
			}
		}
	}

	reach := make(map[string]int, len(component))
	for name, id := range component {
		reach[name] = sizeOf[id]
	}
	return reach
}

// ReviewPlans updates every plan, retires the finished ones, and starts a new
// plan when there is room for another.
func (npc *NPCAIPlayer) ReviewPlans(gc *GameController, player *models.Player, transcript *GameTranscript) {
	if gc.Plans == nil {
		gc.Plans = NewPlanBook()
	}

	for _, plan := range gc.Plans.For(player.Name) {
		before, movedOn := plan.State, plan.LastProgress
		plan.Review(gc)

		// Report a change of state, and also a force that merely grew. A plan can
		// spend a dozen turns in Forming while a small power saves up for its
		// shipping; logging state alone makes that look like nothing happening.
		if plan.State == before && plan.LastProgress == movedOn {
			continue
		}
		transcript.LogAction(player.Name, plan.Describe())

		// A finished operation leaves warships in a foreign sea. Give them
		// orders rather than letting them drift out of the war.
		if plan.State == PlanSucceeded || plan.State == PlanAbandoned {
			npc.DisposeOfEscorts(gc, player, plan, transcript)
		}
	}

	// One operation at a time. Several at once split the shipping so thinly
	// that none of them ever sails.
	if len(gc.Plans.Active(player.Name)) == 0 {
		if plan := npc.ProposePlan(gc, player); plan != nil {
			gc.Plans.Add(plan)
			transcript.LogAction(player.Name, "new "+plan.Describe())
		}
	}

	// Take up whatever is available for the plans that still need it.
	for _, plan := range gc.Plans.Active(player.Name) {
		npc.assignUnits(gc, player, plan)
	}
}

// assignUnits gives a plan any uncommitted units it still needs.
func (npc *NPCAIPlayer) assignUnits(gc *GameController, player *models.Player, plan *AmphibiousPlan) {
	g := gc.Game
	units := g.Units()

	claim := func(pieceID int) bool { return !gc.Plans.Committed(player.Name, pieceID) }

	for _, held := range player.Territories {
		for _, id := range held.Pieces {
			piece, ok := g.Pieces[id]
			if !ok || !claim(id) {
				continue
			}
			caps := units.For(piece)
			switch {
			case piece.Terrain == models.Land && !caps.IsStructure && !caps.IsAA &&
				len(plan.Troops) < plan.WantTroops && piece.Movement > 0:
				plan.Troops = append(plan.Troops, id)
			case piece.Terrain == models.Water && carriesLandUnits(g, piece) &&
				len(plan.Ships) < plan.WantTransports:
				plan.Ships = append(plan.Ships, id)
			case piece.Terrain == models.Water && !carriesLandUnits(g, piece) &&
				combatValue(piece) > 0 &&
				plan.EscortStrength(g) < plan.WantEscort:
				// Escorts: something to fight with, taken up until the convoy
				// has the cover the crossing calls for.
				plan.Escorts = append(plan.Escorts, id)
			}
		}
	}
}

// PlanPurchases returns what the active plans still need to buy, so the plan
// drives production rather than production happening by habit.
func (npc *NPCAIPlayer) PlanPurchases(gc *GameController, player *models.Player) map[string]int {
	wanted := make(map[string]int)
	if gc.Plans == nil {
		return wanted
	}

	g := gc.Game
	transportName, _ := shippingNames(g)

	for _, plan := range gc.Plans.Active(player.Name) {
		if transportName != "" && len(plan.Ships) < plan.WantTransports {
			wanted[transportName] += plan.WantTransports - len(plan.Ships)
		}

		// Buy cover in proportion to what is in the way. An unguarded crossing
		// asks for nothing and the budget goes to troops instead.
		short := plan.WantEscort - plan.EscortStrength(g)
		if short <= 0 {
			continue
		}
		warship := bestWarshipFor(g, short)
		if warship == "" {
			continue
		}
		perShip := combatValue(g.GlobalPieceTemplates[warship])
		if perShip <= 0 {
			perShip = 1
		}
		wanted[warship] += (short + perShip - 1) / perShip
	}
	return wanted
}

// planPurchaseOrder puts shipping ahead of everything else on a plan's list.
//
// Plain alphabetical order spends the expeditionary purse on escorts first --
// "battleship" and "sub" sort before "transport" -- so a poor power bought cover
// for a convoy it never had the lift to assemble. Escorts protect something;
// buy the something first.
func planPurchaseOrder(g *models.Game, wanted map[string]int) []string {
	transport, _ := shippingNames(g)

	order := make([]string, 0, len(wanted))
	if wanted[transport] > 0 {
		order = append(order, transport)
	}
	for _, name := range sortedWants(wanted) {
		if name != transport {
			order = append(order, name)
		}
	}
	return order
}

// wantedCost totals what a shopping list would cost, which is the most a power
// has any reason to save.
func wantedCost(g *models.Game, wanted map[string]int) int {
	total := 0
	for name, count := range wanted {
		if template, ok := g.GlobalPieceTemplates[name]; ok {
			total += int(template.Cost) * count
		}
	}
	return total
}

// bestWarshipFor picks what to buy as cover: the best fighting value per IPC
// among the ships this board offers.
//
// Any warship will do -- a battleship, a submarine, or a carrier, whose value
// counts the aircraft it carries. What matters is that the convoy has something
// to fight with, not which silhouette it is.
func bestWarshipFor(g *models.Game, needed int) string {
	units := g.Units()

	names := make([]string, 0, len(g.GlobalPieceTemplates))
	for name := range g.GlobalPieceTemplates {
		names = append(names, name)
	}
	sort.Strings(names)

	best, bestRatio := "", 0.0
	for _, name := range names {
		template := g.GlobalPieceTemplates[name]
		if template.Terrain != models.Water || units.Of(name).IsStructure {
			continue
		}
		if carriesLandUnits(g, template) {
			continue // that is the transport, not its escort
		}
		value := combatValue(template)
		if value <= 0 || template.Cost <= 0 {
			continue
		}
		ratio := float64(value) / float64(template.Cost)
		if ratio > bestRatio {
			best, bestRatio = name, ratio
		}
	}
	return best
}

// shippingNames finds what this board calls a troop transport and a warship.
//
// Capacity alone is not enough to identify a transport. A carrier also has
// capacity, but it carries aircraft -- and taking the first ship with a hold
// meant the computer players spent their shipping budget on carriers, tried to
// load infantry into them, failed, and every invasion stalled in port. What
// makes a transport is that it can carry land units.
func shippingNames(g *models.Game) (transport, escort string) {
	units := g.Units()

	names := make([]string, 0, len(g.GlobalPieceTemplates))
	for name := range g.GlobalPieceTemplates {
		names = append(names, name)
	}
	sort.Strings(names)

	bestEscort := -1
	for _, name := range names {
		template := g.GlobalPieceTemplates[name]
		if template.Terrain != models.Water || units.Of(name).IsStructure {
			continue
		}
		if carriesLandUnits(g, template) {
			if transport == "" {
				transport = name
			}
			continue
		}
		// Cheapest warship that can actually fight.
		if template.Attack > 0 {
			if bestEscort == -1 || int(template.Cost) < bestEscort {
				bestEscort, escort = int(template.Cost), name
			}
		}
	}
	return transport, escort
}

// carriesLandUnits reports whether a ship can carry an army rather than
// aircraft.
func carriesLandUnits(g *models.Game, template *models.Piece) bool {
	if template.Capacity <= 0 {
		return false
	}
	for _, cargo := range template.CanCarry {
		if carried, ok := g.GlobalPieceTemplates[cargo]; ok && carried.Terrain == models.Land {
			return true
		}
	}
	return false
}

// sortedWants orders a purchase list so production is deterministic for a given
// seed rather than following Go's map iteration.
func sortedWants(wanted map[string]int) []string {
	names := make([]string, 0, len(wanted))
	for name := range wanted {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

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
	// The whole side's claims, not just our own: allied powers coordinate,
	// so two of them do not build invasions of the same island.
	claimed := gc.Plans.SideTargets(player.Name)
	pressure := strategicPressure(g, player)
	troopCap := maxPlanTroopsFor(pressure)

	var options []candidate
	for name, territory := range g.Board {
		if territory.Terrain != models.Land || claimed[name] {
			continue
		}
		// A target recently judged hopeless cools off before being redrawn.
		if gc.Plans.CoolingOff(player.Name, name, g.Turn) {
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

		// A fortress the largest liftable force cannot beat is not a target --
		// but the cap, and so the reach, grows with the clock.
		if defenderStrength(g, name) > hopelessDefenceFor(troopCap) {
			continue
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
	//
	// A power being outproduced halves the defence penalty: an expedition at
	// somewhat unfavourable odds today beats the same expedition at hopeless
	// odds after the enemy's factories have run for another five rounds.
	defencePenalty := func(defence int) int { return defence }
	if outproduced(pressure) {
		defencePenalty = func(defence int) int { return defence / 2 }
	}
	score := func(c candidate) int {
		return c.value*planValueWeight - c.crossing*planCrossingWeight - defencePenalty(c.defence)
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

	troops := troopsNeeded(pick.defence, npc.rng, troopCap)
	return &AmphibiousPlan{
		Power:          player.Name,
		Target:         pick.target,
		Staging:        pick.staging,
		Embark:         pick.embark,
		DropZone:       pick.dropZone,
		State:          PlanForming,
		WantTroops:     troops,
		WantTransports: (troops + transportCapacity - 1) / transportCapacity,
		InitialDefence: pick.defence,
		CreatedTurn:    g.Turn,
		LastProgress:   g.Turn,
	}
}

// transportCapacity is how many land units one transport is assumed to carry.
// The board declares the real figure in its Containers section; this is only
// used to size a plan, and being wrong costs an extra ship, not correctness.
const transportCapacity = 2

// troopsNeeded sizes a landing force against the defence, with a little
// variation so a power does not always commit exactly the same amount. The
// cap comes from the clock: an unhurried power keeps its build-ups short,
// one that must win soon assembles a real invasion.
//
// This is the "gather more forces, or act quickly" decision: a lightly held
// island gets a small force soon, a strong one gets a build-up.
func troopsNeeded(defence int, rng *rand.Rand, troopCap int) int {
	needed := defence/2 + 2
	if rng != nil {
		needed += rng.Intn(3)
	}
	if needed < 2 {
		needed = 2
	}
	if needed > troopCap {
		needed = troopCap // beyond this the build-up never finishes
	}
	return needed
}

// Plan scoring weights: what a point of production is worth against a sea
// zone of exposure, and what a victory city adds to a target's value.
const (
	planValueWeight    = 4
	planCrossingWeight = 3
	planVCBonus        = 6
)

func territoryValue(territory *models.Territory) int {
	value := territory.Production
	if territory.IsVictoryCity {
		value += planVCBonus
	}
	return value
}

// reachableOverland reports whether a power can march to a territory from
// land it can actually march THROUGH -- its own and its allies'. The target
// counts as overland-reachable when it borders that friendly landmass, where
// an ordinary attack can already reach it.
//
// The walk used to cross ANY land, enemy and neutral alike: from UK-held
// Egypt there was a "land path" to Western Europe through the whole
// Axis-held continent, so the UK never once planned a landing in Europe --
// every European target was dismissed as "not an amphibious problem". Fifty
// observed games: the Allies captured Axis victory cities six times, all
// but one of them Pacific islands.
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
				return true // it borders ground we can march across
			}
			// March only across our own side's soil; enemy or neutral
			// country in the way is exactly what makes a target an
			// amphibious problem.
			if next.Owner != player && !areAllies(next.Owner, player) {
				continue
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

	// Loose lips: every live operation runs a small risk each turn of its
	// details reaching the other side. A leaked operation stays in force --
	// cancelling it would tell the enemy their intelligence was good -- but
	// its reports are no longer redacted, and enemy powers may prepare.
	for _, plan := range gc.Plans.Active(player.Name) {
		if plan.Revealed || npc.rng == nil {
			continue
		}
		if npc.rng.Float64() < operationLeakChance {
			plan.Revealed = true
			transcript.LogAction(player.Name,
				"Intelligence leak: the enemy has learned of "+plan.Describe())
		}
	}

	// A plan's details are secret from the other side until they leak.
	logPlan := func(plan *AmphibiousPlan, text string) {
		if plan.Revealed {
			transcript.LogAction(player.Name, text)
		} else {
			transcript.LogSecretAction(player.Name, text)
		}
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
		logPlan(plan, plan.Describe())

		// A finished operation leaves warships in a foreign sea. Give them
		// orders rather than letting them drift out of the war.
		if plan.State == PlanSucceeded || plan.State == PlanAbandoned {
			npc.DisposeOfEscorts(gc, player, plan, transcript)
		}
		if plan.State == PlanAbandoned && plan.HopelessTarget {
			gc.Plans.RecordHopeless(player.Name, plan.Target, gc.Game.Turn)
		}
	}

	// With time to spare, one operation at a time: several at once split the
	// shipping so thinly that none of them ever sails. With the clock against
	// us the arithmetic reverses -- a second (or, desperate, a third) front
	// forces the enemy to defend everywhere at once, and the urgent purse is
	// open wide enough to float them all.
	wantPlans := concurrentPlansFor(strategicPressure(gc.Game, player))
	for len(gc.Plans.Active(player.Name)) < wantPlans {
		plan := npc.ProposePlan(gc, player)
		if plan == nil {
			break // nothing else worth invading
		}
		gc.Plans.Add(plan)
		transcript.LogSecretAction(player.Name, "new "+plan.Describe())
	}

	// Take up whatever is available for the plans that still need it.
	for _, plan := range gc.Plans.Active(player.Name) {
		npc.assignUnits(gc, player, plan)
	}
}

// assignUnits gives a plan any uncommitted units it still needs.
//
// Troops are taken only from the staging port's own landmass. Recruiting from
// anywhere the power held committed island garrisons to armies they could
// never march to join; the plan then counted them as progress and waited out
// its stall limit on troops that were never coming.
func (npc *NPCAIPlayer) assignUnits(gc *GameController, player *models.Player, plan *AmphibiousPlan) {
	g := gc.Game
	units := g.Units()
	reachesPort := marchableTo(g, player, plan.Staging)

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
				len(plan.Troops) < plan.WantTroops && piece.Movement > 0 &&
				reachesPort[held.Name]:
				plan.Troops = append(plan.Troops, id)
			case piece.Terrain == models.Water && carriesLandUnits(g, piece) &&
				len(plan.Ships) < plan.WantTransports:
				plan.Ships = append(plan.Ships, id)
			case piece.Terrain == models.Water && !carriesLandUnits(g, piece) &&
				combatValue(piece) > 0 &&
				(plan.EscortStrength(g) < plan.WantEscort ||
					(units.For(piece).CanBombard && !plan.hasBombardier(g))):
				// Escorts: something to fight with, taken up until the convoy
				// has the cover the crossing calls for -- and one ship that can
				// shell the beach, even when the cover is already sufficient.
				plan.Escorts = append(plan.Escorts, id)
			}
		}
	}
}

// marchableTo returns the friendly land territories from which an army can
// walk to the given territory without crossing water.
func marchableTo(g *models.Game, player *models.Player, to string) map[string]bool {
	start, ok := g.Board[to]
	if !ok || start.Terrain != models.Land {
		return nil
	}

	reach := map[string]bool{to: true}
	queue := []*models.Territory{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbour := range current.ConnectedTo {
			if neighbour.Terrain != models.Land || reach[neighbour.Name] {
				continue
			}
			if neighbour.Owner != player && !areAllies(neighbour.Owner, player) {
				continue
			}
			reach[neighbour.Name] = true
			queue = append(queue, neighbour)
		}
	}
	return reach
}

// hasCoastalProduction reports whether any of a power's factories borders the
// sea -- the precondition for building a navy at all.
func hasCoastalProduction(g *models.Game, player *models.Player) bool {
	units := g.Units()
	for _, territory := range player.Territories {
		hasFactory := false
		for _, id := range territory.Pieces {
			if units.For(g.Pieces[id]).IsStructure {
				hasFactory = true
				break
			}
		}
		if hasFactory && len(adjacentSeaZones(g, territory.Name)) > 0 {
			return true
		}
	}
	return false
}

// PlanPurchases returns what the active plans still need to buy, so the plan
// drives production rather than production happening by habit.
func (npc *NPCAIPlayer) PlanPurchases(gc *GameController, player *models.Player) map[string]int {
	wanted := make(map[string]int)
	if gc.Plans == nil {
		return wanted
	}

	g := gc.Game

	// A power with no coastal factory cannot launch a ship: buying one puts it
	// in the mobilisation queue forever. The USSR -- one landlocked factory in
	// Moscow -- bought sixteen transports this way, and its plans must make do
	// with whatever shipping it already has afloat.
	if !hasCoastalProduction(g, player) {
		return wanted
	}

	transportName, _ := shippingNames(g)

	for _, plan := range gc.Plans.Active(player.Name) {
		if transportName != "" && len(plan.Ships) < plan.WantTransports {
			wanted[transportName] += plan.WantTransports - len(plan.Ships)
		}

		// A landing wants one ship that can shell the beach. Escorts are
		// bought by fighting value per IPC, which picks submarines -- so once
		// the starting battleships sank, late-game landings went in without
		// naval gunfire. Lift comes first: the gun is only wanted once the
		// transports are on hand. And only with time to spare: a power under
		// pressure strikes with what it has rather than waiting out the price
		// of a battleship -- a landing this turn can matter more than naval
		// gunfire next month.
		if bombardier := bombardierName(g); bombardier != "" &&
			!outproduced(strategicPressure(g, player)) &&
			len(plan.Ships) >= plan.WantTransports && !plan.hasBombardier(g) {
			wanted[bombardier]++
		}

		// Buy cover in proportion to what is in the way. An unguarded crossing
		// asks for nothing and the budget goes to troops instead.
		short := plan.WantEscort - plan.EscortStrength(g)
		if short <= 0 {
			continue
		}
		warship := bestWarship(g)
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

// bombardierName is the cheapest ship on this board that can shell a beach.
func bombardierName(g *models.Game) string {
	units := g.Units()
	best, bestCost := "", 0
	for _, name := range sortedTemplateNames(g) {
		template := g.GlobalPieceTemplates[name]
		if template.Terrain != models.Water || !units.Of(name).CanBombard {
			continue
		}
		if best == "" || int(template.Cost) < bestCost {
			best, bestCost = name, int(template.Cost)
		}
	}
	return best
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

// bestWarship picks what to buy as cover: the best fighting value per IPC
// among the ships this board offers.
//
// Any warship will do -- a battleship, a submarine, or a carrier, whose value
// counts the aircraft it carries. What matters is that the convoy has something
// to fight with, not which silhouette it is.
func bestWarship(g *models.Game) string {
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

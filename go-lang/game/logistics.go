package game

import (
	"fmt"
	"sort"

	"boardgame/models"
)

// Moving the army to where the war is.
//
// Six observed games all died the same way: production piled up in capitals
// (Germany kept 135 of its 178 units in Berlin) while the fronts held five or
// six, because reinforcement only ever *sourced* from "safe" territories --
// defined so strictly that a capital bordering an ally, a neutral or someone
// else's nominal sea zone never qualified -- and was capped at a handful of
// moves a turn against a factory output twice that. The war froze by round
// six and every game ran to the turn cap as the same 8-6 stalemate.
//
// The replacement is a quartermaster's rule, not a general's: keep what the
// defence plans say to keep, and march everything else toward the fighting.
// Fronts are ranked by deficit -- the enemy strength next door minus our own
// strength on the spot -- and reinforced until at least equal. Surplus beyond
// that still walks forward, and surplus on a landmass with no fighting is
// ferried by whatever transports are not already spoken for.

// front is one of our territories with the enemy next door.
type front struct {
	territory *models.Territory
	enemy     int // attack strength adjacent
	ours      int // defence strength present
	deficit   int // enemy - ours; positive means outnumbered
	distances map[string]int // marching distance from every friendly territory
}

// findFronts lists the player's land territories that border hostile land or
// hostile armies, worst-outnumbered first.
func findFronts(g *models.Game, player *models.Player) []*front {
	var fronts []*front
	for _, territory := range sortedTerritories(player) {
		if territory.Terrain != models.Land {
			continue
		}
		enemy := 0
		for _, neighbour := range territory.ConnectedTo {
			if neighbour.Terrain != models.Land {
				continue
			}
			if neighbour.Owner == player || areAllies(neighbour.Owner, player) {
				continue
			}
			// Unclaimed neutrals are not a threat; occupied ones are.
			if neighbour.Owner != nil && neighbour.Owner.Name == "Neutral" && !enemyPresent(g, neighbour, player) {
				continue
			}
			enemy += attackStrength(g, neighbour)
		}
		if enemy == 0 {
			continue
		}
		f := &front{
			territory: territory,
			enemy:     enemy,
			ours:      defenceStrength(g, territory, player),
			distances: marchDistances(g, player, territory.Name),
		}
		f.deficit = f.enemy - f.ours
		fronts = append(fronts, f)
	}
	sort.Slice(fronts, func(i, j int) bool {
		if fronts[i].deficit != fronts[j].deficit {
			return fronts[i].deficit > fronts[j].deficit
		}
		return fronts[i].territory.Name < fronts[j].territory.Name
	})
	return fronts
}

// defenceStrength is the defensive value of a player's units standing in a
// territory, structures excluded.
func defenceStrength(g *models.Game, territory *models.Territory, player *models.Player) int {
	units := g.Units()
	strength := 0
	for _, id := range territory.Pieces {
		piece, ok := g.Pieces[id]
		if !ok || piece.Owner != player {
			continue
		}
		if units.For(piece).IsStructure {
			continue
		}
		strength += int(piece.Defend)
	}
	return strength
}

// marchDistances maps every friendly land territory to its overland marching
// distance from the given territory.
func marchDistances(g *models.Game, player *models.Player, to string) map[string]int {
	start, ok := g.Board[to]
	if !ok || start.Terrain != models.Land {
		return nil
	}
	dist := map[string]int{to: 0}
	queue := []*models.Territory{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, neighbour := range current.ConnectedTo {
			if neighbour.Terrain != models.Land {
				continue
			}
			if _, seen := dist[neighbour.Name]; seen {
				continue
			}
			if neighbour.Owner != player && !areAllies(neighbour.Owner, player) {
				continue
			}
			dist[neighbour.Name] = dist[current.Name] + 1
			queue = append(queue, neighbour)
		}
	}
	return dist
}

// surplusIn lists a territory's exportable units: mobile land units that are
// not committed to any plan (garrisons stay), beyond what the local front
// needs to stay equal with its neighbours.
func (npc *NPCAIPlayer) surplusIn(gc *GameController, player *models.Player, territory *models.Territory, keepStrength int) []*models.Piece {
	g := gc.Game
	units := g.Units()

	var free []*models.Piece
	for _, id := range territory.Pieces {
		piece, ok := g.Pieces[id]
		if !ok || piece.Owner != player {
			continue
		}
		if piece.Terrain != models.Land || piece.Movement <= 0 {
			continue
		}
		caps := units.For(piece)
		if caps.IsStructure || caps.IsAA {
			continue
		}
		if gc.Plans.Committed(player.Name, piece.ID) {
			continue
		}
		free = append(free, piece)
	}

	// A front keeps enough of its free units to stay equal; only the rest is
	// surplus. Weakest defenders are exported first, so the line holds with
	// what defends best.
	if keepStrength > 0 {
		sort.SliceStable(free, func(i, j int) bool { return free[i].Defend < free[j].Defend })
		kept := 0
		for len(free) > 0 && kept < keepStrength {
			last := free[len(free)-1]
			kept += int(last.Defend)
			free = free[:len(free)-1]
		}
	}
	return free
}

// DisperseToFronts marches uncommitted surplus toward the fighting.
//
// Outnumbered fronts are reinforced first, until at least equal with the enemy
// next door; whatever surplus remains still walks toward the nearest front
// rather than accumulating at the factory that built it.
func (npc *NPCAIPlayer) DisperseToFronts(gc *GameController, player *models.Player, transcript *GameTranscript) int {
	g := gc.Game
	fronts := findFronts(g, player)
	if len(fronts) == 0 {
		return 0
	}

	// Collect surplus once. A front territory keeps enough to stay equal;
	// everywhere else exports every free unit.
	type source struct {
		territory *models.Territory
		units     []*models.Piece
	}
	frontNeeds := make(map[string]int, len(fronts))
	for _, f := range fronts {
		frontNeeds[f.territory.Name] = f.enemy
	}
	var sources []*source
	for _, territory := range sortedTerritories(player) {
		if territory.Terrain != models.Land {
			continue
		}
		units := npc.surplusIn(gc, player, territory, frontNeeds[territory.Name])
		if len(units) > 0 {
			sources = append(sources, &source{territory, units})
		}
	}
	if len(sources) == 0 {
		return 0
	}

	moved := 0
	march := func(piece *models.Piece, from *models.Territory, f *front) bool {
		if from.Name == f.territory.Name {
			return false
		}
		step := nextStepTowards(g, piece, from.Name, f.territory.Name, player, models.Land)
		if step == "" {
			return false
		}
		if err := gc.PlanMove(piece.ID, from.Name, step); err != nil {
			return false
		}
		transcript.LogMove(player.Name, piece.Name, from.Name, step, "noncombat")
		moved++
		return true
	}

	// Pass one: fill deficits, nearest willing source first, until each
	// outnumbered front expects at least equality. Units en route count toward
	// the front they are marching for, so a long column is not double-ordered.
	for _, f := range fronts {
		for f.deficit > 0 {
			var best *source
			bestDist := 0
			for _, s := range sources {
				if len(s.units) == 0 {
					continue
				}
				d, reachable := f.distances[s.territory.Name]
				if !reachable || d == 0 {
					continue
				}
				if best == nil || d < bestDist {
					best, bestDist = s, d
				}
			}
			if best == nil {
				break // nobody left who can march there
			}
			piece := best.units[len(best.units)-1]
			best.units = best.units[:len(best.units)-1]
			if march(piece, best.territory, f) {
				f.deficit -= int(piece.Defend)
			}
		}
	}

	// Pass two: remaining surplus walks toward its nearest front anyway.
	// Standing in the capital defends nothing the garrison is not already
	// defending.
	for _, s := range sources {
		for _, piece := range s.units {
			var nearest *front
			nearestDist := 0
			for _, f := range fronts {
				d, reachable := f.distances[s.territory.Name]
				if !reachable || d == 0 {
					continue
				}
				if nearest == nil || d < nearestDist {
					nearest, nearestDist = f, d
				}
			}
			if nearest == nil {
				continue // an island garrison; the ferries deal with it
			}
			march(piece, s.territory, nearest)
		}
	}
	return moved
}

// FerrySurplus shuttles stranded surplus toward the war by sea.
//
// An island power's army cannot march to its front: Britain and Japan each
// ended probe games with ~170 units at home for want of sea lift. Transports
// not committed to a plan run a standing shuttle -- load surplus where it is
// stranded, sail for a friendly coast that can reach a front overland, unload,
// come back. The shuttle is stateless: each turn every idle transport decides
// afresh from where the cargo and the fronts actually are.
func (npc *NPCAIPlayer) FerrySurplus(gc *GameController, player *models.Player, transcript *GameTranscript) int {
	g := gc.Game
	fronts := findFronts(g, player)
	if len(fronts) == 0 {
		return 0
	}

	// Territories that can reach a front on foot, and those that cannot.
	reachesFront := make(map[string]bool)
	for _, f := range fronts {
		for name := range f.distances {
			reachesFront[name] = true
		}
	}

	// Stranded surplus: free units on landmasses that reach no front.
	stranded := make(map[string][]*models.Piece)
	for _, territory := range sortedTerritories(player) {
		if territory.Terrain != models.Land || reachesFront[territory.Name] {
			continue
		}
		if len(adjacentSeaZones(g, territory.Name)) == 0 {
			continue
		}
		if units := npc.surplusIn(gc, player, territory, 0); len(units) > 0 {
			stranded[territory.Name] = units
		}
	}

	// Delivery coasts: friendly coastal territories that do reach a front.
	var deliveries []string
	for name := range reachesFront {
		if territory := g.Board[name]; territory != nil &&
			(territory.Owner == player || areAllies(territory.Owner, player)) &&
			len(adjacentSeaZones(g, name)) > 0 {
			deliveries = append(deliveries, name)
		}
	}
	sort.Strings(deliveries)
	if len(deliveries) == 0 {
		return 0
	}

	moved := 0
	for _, transportID := range npc.idleTransports(gc, player) {
		transport := g.Pieces[transportID]
		at := territoryOf(g, transportID)
		if at == nil || transport == nil {
			continue
		}

		if len(transport.Holding) > 0 {
			// Loaded: unload onto an adjacent delivery coast, or sail toward
			// the nearest one.
			unloaded := false
			for _, name := range deliveries {
				if !areConnected(at, g.Board[name]) {
					continue
				}
				for _, cargo := range append([]int{}, transport.Holding...) {
					if gc.UnloadUnit(transportID, cargo, name) == nil {
						moved++
					}
				}
				transcript.LogAction(player.Name, fmt.Sprintf(
					"ferry unloaded reinforcements at %s", name))
				unloaded = true
				break
			}
			if unloaded {
				continue
			}
			if dest := nearestSeaZoneBy(g, player, at.Name, deliveries); dest != "" {
				if step := nextStepTowards(g, transport, at.Name, dest, player, models.Water); step != "" {
					if gc.PlanMove(transportID, at.Name, step) == nil {
						transcript.LogMove(player.Name, transport.Name, at.Name, step, "noncombat")
						moved++
					}
				}
			}
			continue
		}

		// Empty: load stranded surplus if we are beside some, otherwise sail
		// toward the nearest stranded pile.
		loadedAny := false
		for name, units := range stranded {
			if !areConnected(at, g.Board[name]) {
				continue
			}
			for len(units) > 0 && len(transport.Holding) < int(transport.Capacity) {
				piece := units[len(units)-1]
				units = units[:len(units)-1]
				if gc.LoadUnit(transportID, piece.ID) == nil {
					loadedAny = true
					moved++
				}
			}
			stranded[name] = units
			break
		}
		if loadedAny {
			continue
		}
		var piles []string
		for name := range stranded {
			piles = append(piles, name)
		}
		sort.Strings(piles)
		if dest := nearestSeaZoneBy(g, player, at.Name, piles); dest != "" && dest != at.Name {
			if step := nextStepTowards(g, transport, at.Name, dest, player, models.Water); step != "" {
				if gc.PlanMove(transportID, at.Name, step) == nil {
					transcript.LogMove(player.Name, transport.Name, at.Name, step, "noncombat")
					moved++
				}
			}
		}
	}
	return moved
}

// idleTransports lists the player's troop transports not committed to a plan.
func (npc *NPCAIPlayer) idleTransports(gc *GameController, player *models.Player) []int {
	g := gc.Game
	var out []int
	for _, id := range sortedIntKeys(g.Pieces) {
		piece := g.Pieces[id]
		if piece.Owner != player || !carriesLandUnits(g, piece) {
			continue
		}
		if gc.Plans.Committed(player.Name, id) {
			continue
		}
		if territoryOf(g, id) == nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

// nearestSeaZoneBy finds the closest sea zone (by sea travel from `from`)
// adjacent to any of the named land territories. Returns "" if none is
// reachable.
func nearestSeaZoneBy(g *models.Game, player *models.Player, from string, lands []string) string {
	wanted := make(map[string]bool)
	for _, name := range lands {
		for _, zone := range adjacentSeaZones(g, name) {
			wanted[zone] = true
		}
	}
	if len(wanted) == 0 {
		return ""
	}
	if wanted[from] {
		return from
	}

	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		territory := g.Board[current]
		if territory == nil {
			continue
		}
		for _, neighbour := range territory.ConnectedTo {
			if neighbour.Terrain != models.Water || seen[neighbour.Name] {
				continue
			}
			if occupiedByEnemy(g, neighbour, player) {
				continue
			}
			if wanted[neighbour.Name] {
				return neighbour.Name
			}
			seen[neighbour.Name] = true
			queue = append(queue, neighbour.Name)
		}
	}
	return ""
}

func sortedIntKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Ints(out)
	return out
}

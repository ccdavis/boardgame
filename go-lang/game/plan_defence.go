package game

import (
	"fmt"
	"sort"

	"boardgame/models"
)

// Garrisoning what a power cannot afford to lose.
//
// Offence is easy to plan for because it has an obvious object. Defence has no
// object, so a stateless AI never does it: there is no turn on which garrisoning
// a factory looks better than attacking something. A defence plan gives the
// quiet work a goal and a finishing line -- a target strength for a place worth
// holding -- so it competes for production on equal terms and, once met, stops
// consuming and lets the surplus go to the front.

// DefencePlan is an intent to hold a territory to a given strength.
type DefencePlan struct {
	ID    int
	Power string

	// Codename is the operation's name, drawn from the side's vendored list.
	Codename string

	Territory string

	// WantStrength is the defensive value the garrison should reach, from what
	// the place is worth, what threatens it, and how cautious the power is.
	WantStrength int

	// Garrison holds the units assigned to stay here.
	Garrison []int

	// WantAA is whether this place should have anti-aircraft cover. Factories
	// and victory cities attract bombers.
	WantAA bool

	Satisfied    bool
	CreatedTurn  int
	LastProgress int
}

// Describe renders a defence plan for a transcript.
func (p *DefencePlan) Describe(g *models.Game) string {
	state := "building"
	if p.Satisfied {
		state = "held"
	}
	name := p.Codename
	if name == "" {
		name = "UNNAMED"
	}
	return fmt.Sprintf("Operation %s (defence %d): hold %s to strength %d (%s; have %d)",
		name, p.ID, p.Territory, p.WantStrength, state, p.GarrisonStrength(g))
}

// GarrisonStrength is the defensive value currently stationed here.
func (p *DefencePlan) GarrisonStrength(g *models.Game) int {
	territory := g.Board[p.Territory]
	if territory == nil {
		return 0
	}
	units := g.Units()

	strength := 0
	for _, id := range p.Garrison {
		piece, ok := g.Pieces[id]
		if !ok || !contains(territory.Pieces, id) {
			continue // destroyed, or wandered off
		}
		if units.For(piece).IsStructure {
			continue
		}
		strength += int(piece.Defend)
	}
	return strength
}

// Review keeps a defence plan honest: it drops units that have died or left,
// re-reads the threat, and reports whether the place is held.
func (p *DefencePlan) Review(gc *GameController) bool {
	g := gc.Game

	territory, ok := g.Board[p.Territory]
	if !ok || territory.Owner == nil || territory.Owner.Name != p.Power {
		return false // lost it, or it was never ours; the plan is moot
	}

	p.Garrison = survivors(g, p.Power, p.Garrison)
	p.WantStrength = garrisonTarget(g, territory, PostureFor(p.Power))
	p.WantAA = worthAntiAircraft(g, territory)

	was := p.Satisfied
	p.Satisfied = p.GarrisonStrength(g) >= p.WantStrength
	if p.Satisfied != was {
		p.LastProgress = g.Turn
	}
	return true
}

// garrisonTarget decides how strongly a place should be held.
//
// Value first -- a victory city or a factory is worth more than open country --
// then the threat next door, then the power's own caution. A defensive power
// wants a deeper garrison for the same ground than an aggressive one.
func garrisonTarget(g *models.Game, territory *models.Territory, posture Posture) int {
	if territory.Owner == nil {
		return 0
	}

	value := territory.Production
	if territory.IsVictoryCity {
		value += 6
	}
	if hasProduction(g, territory) {
		value += 4
	}

	// What could reach it next turn.
	threat := 0
	for _, neighbour := range territory.ConnectedTo {
		if neighbour.Owner == nil || neighbour.Owner == territory.Owner {
			continue
		}
		if areAllies(neighbour.Owner, territory.Owner) {
			continue
		}
		threat += attackStrength(g, neighbour)
	}

	target := int(float64(value)*posture.Garrison) + threat/2
	if target < 2 {
		target = 2
	}
	if target > maxGarrison {
		target = maxGarrison
	}
	return target
}

// maxGarrison bounds a single garrison, so one frightening neighbour cannot
// absorb an entire war economy.
const maxGarrison = 20

// attackStrength rates what a territory could attack with.
func attackStrength(g *models.Game, territory *models.Territory) int {
	units := g.Units()
	strength := 0
	for _, id := range territory.Pieces {
		piece, ok := g.Pieces[id]
		if !ok || piece.Movement == 0 {
			continue
		}
		if units.For(piece).IsStructure {
			continue
		}
		strength += int(piece.Attack)
	}
	return strength
}

func hasProduction(g *models.Game, territory *models.Territory) bool {
	units := g.Units()
	for _, id := range territory.Pieces {
		if piece, ok := g.Pieces[id]; ok && units.For(piece).IsStructure {
			return true
		}
	}
	return false
}

// worthAntiAircraft reports whether a place is worth covering against bombers:
// the factories that build the army and the cities that decide the game.
func worthAntiAircraft(g *models.Game, territory *models.Territory) bool {
	return territory.IsVictoryCity || hasProduction(g, territory)
}

// hasAntiAircraft reports whether a territory already has cover.
func hasAntiAircraft(g *models.Game, territory *models.Territory) bool {
	units := g.Units()
	for _, id := range territory.Pieces {
		if piece, ok := g.Pieces[id]; ok && units.For(piece).IsAA {
			return true
		}
	}
	return false
}

// ReviewDefences keeps a power's garrisons current and opens plans for the
// places that warrant one.
func (npc *NPCAIPlayer) ReviewDefences(gc *GameController, player *models.Player, transcript *GameTranscript) {
	if gc.Plans == nil {
		gc.Plans = NewPlanBook()
	}

	// Intelligence: revealed enemy landings make their targets worth holding,
	// and the counterforce is sized against the invasion actually being
	// prepared. The garrison plan is itself an ordinary (secret) operation.
	threats := gc.Plans.RevealedThreatsAgainst(gc.Game, player)

	kept := make([]*DefencePlan, 0, len(gc.Plans.defences[player.Name]))
	held := make(map[string]bool)
	for _, plan := range gc.Plans.defences[player.Name] {
		if !plan.Review(gc) {
			continue // territory lost; forget the plan
		}
		if extra := threats[plan.Territory]; plan.WantStrength < extra {
			plan.WantStrength = extra
			plan.Satisfied = plan.GarrisonStrength(gc.Game) >= plan.WantStrength
		}
		kept = append(kept, plan)
		held[plan.Territory] = true
	}
	gc.Plans.defences[player.Name] = kept

	// Open plans for anywhere worth holding that has none.
	for _, territory := range sortedTerritories(player) {
		if held[territory.Name] || territory.Terrain != models.Land {
			continue
		}
		if !worthDefending(gc.Game, territory) && threats[territory.Name] == 0 {
			continue
		}
		plan := gc.Plans.AddDefence(&DefencePlan{
			Power:        player.Name,
			Territory:    territory.Name,
			CreatedTurn:  gc.Game.Turn,
			LastProgress: gc.Game.Turn,
		})
		plan.Review(gc)
		if extra := threats[territory.Name]; plan.WantStrength < extra {
			plan.WantStrength = extra
			plan.Satisfied = plan.GarrisonStrength(gc.Game) >= plan.WantStrength
		}
		transcript.LogSecretAction(player.Name, "new "+plan.Describe(gc.Game))
	}

	for _, plan := range gc.Plans.Defences(player.Name) {
		npc.manGarrison(gc, player, plan)
	}
}

// worthDefending picks the places a plan is opened for: production centres,
// victory cities, and anywhere valuable with an enemy next door.
func worthDefending(g *models.Game, territory *models.Territory) bool {
	if territory.IsVictoryCity || hasProduction(g, territory) {
		return true
	}
	if territory.Production < 3 {
		return false
	}
	for _, neighbour := range territory.ConnectedTo {
		if neighbour.Owner == nil || neighbour.Owner == territory.Owner {
			continue
		}
		if !areAllies(neighbour.Owner, territory.Owner) && attackStrength(g, neighbour) > 0 {
			return true
		}
	}
	return false
}

// manGarrison assigns units standing in a territory to its garrison.
//
// Only units already there: a defence plan does not march troops across the
// board, it declares that what is here stays here. Movement toward the front is
// the general logic's business.
func (npc *NPCAIPlayer) manGarrison(gc *GameController, player *models.Player, plan *DefencePlan) {
	g := gc.Game
	territory := g.Board[plan.Territory]
	if territory == nil {
		return
	}
	units := g.Units()

	for _, id := range territory.Pieces {
		if plan.GarrisonStrength(g) >= plan.WantStrength {
			return
		}
		piece, ok := g.Pieces[id]
		if !ok || contains(plan.Garrison, id) {
			continue
		}
		if gc.Plans.Committed(player.Name, id) {
			continue // already spoken for by an operation
		}
		caps := units.For(piece)
		if caps.IsStructure {
			continue
		}
		if piece.Owner == nil || piece.Owner.Name != player.Name {
			continue
		}
		plan.Garrison = append(plan.Garrison, id)
	}
}

// DefencePurchases returns what the garrisons still need.
//
// Anti-aircraft first where a factory or city has none -- it is cheap and
// nothing else does its job -- then whatever defends best per IPC.
func (npc *NPCAIPlayer) DefencePurchases(gc *GameController, player *models.Player) map[string]int {
	wanted := make(map[string]int)
	if gc.Plans == nil {
		return wanted
	}
	g := gc.Game

	aaName := antiAircraftName(g)
	defender := bestDefenderName(g)

	for _, plan := range gc.Plans.Defences(player.Name) {
		territory := g.Board[plan.Territory]
		if territory == nil {
			continue
		}
		if plan.WantAA && aaName != "" && !hasAntiAircraft(g, territory) {
			wanted[aaName]++
		}
		short := plan.WantStrength - plan.GarrisonStrength(g)
		if short <= 0 || defender == "" {
			continue
		}
		per := int(g.GlobalPieceTemplates[defender].Defend)
		if per <= 0 {
			per = 1
		}
		wanted[defender] += (short + per - 1) / per
	}
	return wanted
}

// antiAircraftName finds what this board calls its anti-aircraft unit.
func antiAircraftName(g *models.Game) string {
	units := g.Units()
	for _, name := range sortedTemplateNames(g) {
		if units.Of(name).IsAA {
			return name
		}
	}
	return ""
}

// bestDefenderName picks the best defensive value per IPC among land units.
func bestDefenderName(g *models.Game) string {
	units := g.Units()

	best, bestRatio := "", 0.0
	for _, name := range sortedTemplateNames(g) {
		template := g.GlobalPieceTemplates[name]
		if template.Terrain != models.Land || template.Cost <= 0 {
			continue
		}
		caps := units.Of(name)
		if caps.IsStructure || caps.IsAA {
			continue
		}
		ratio := float64(template.Defend) / float64(template.Cost)
		if ratio > bestRatio {
			best, bestRatio = name, ratio
		}
	}
	return best
}

func sortedTemplateNames(g *models.Game) []string {
	names := make([]string, 0, len(g.GlobalPieceTemplates))
	for name := range g.GlobalPieceTemplates {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedTerritories(player *models.Player) []*models.Territory {
	out := append([]*models.Territory{}, player.Territories...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

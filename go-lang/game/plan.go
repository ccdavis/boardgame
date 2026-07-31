package game

import (
	"fmt"
	"sort"

	"boardgame/models"
)

// Multi-turn planning for the computer players.
//
// Amphibious operations cannot be decided one turn at a time. Taking an island
// means having shipping, having troops at a port, getting both to the same sea
// zone, and only then attacking -- and the units that will do it must be built
// several turns before they are used. A stateless AI re-derives its intent
// every turn and so never accumulates anything: it is why the computer players
// never loaded a transport, and why island territories could not be attacked at
// all.
//
// A plan is a goal that outlives a turn. It names a target, a port to gather
// at, and the sea route between them; it holds handles to the units committed
// to it; and it advances through states as those units come together. Units are
// referenced by piece ID, so a plan notices its own losses: if everything
// committed to it is sunk, the plan starts over rather than believing it still
// has an army.
//
// The rules decide what happens in a turn. Under Axis & Allies an assault
// loads, sails and lands inside one combat-move phase, so the multi-turn part
// is the build-up, not the landing.

// PlanState is how far along a plan is.
type PlanState int

const (
	// PlanForming: buying shipping and massing troops at the port.
	PlanForming PlanState = iota
	// PlanEmbarked: troops are aboard and the convoy is sailing. A transport
	// moves two sea zones a turn and a crossing can be seven, so this state
	// lasts as many turns as the voyage takes.
	PlanEmbarked
	// PlanReady: the convoy is in the sea zone beside the target and lands on
	// the next combat move.
	PlanReady
	// PlanSucceeded: the target was taken.
	PlanSucceeded
	// PlanAbandoned: the target stopped being worth taking, or the plan failed
	// too many times to keep trying.
	PlanAbandoned
)

func (s PlanState) String() string {
	switch s {
	case PlanForming:
		return "forming"
	case PlanEmbarked:
		return "at sea"
	case PlanReady:
		return "ready"
	case PlanSucceeded:
		return "succeeded"
	case PlanAbandoned:
		return "abandoned"
	default:
		return "unknown"
	}
}

// AmphibiousPlan is an intent to take a territory that can only be reached by
// sea, together with the force being assembled to do it.
type AmphibiousPlan struct {
	ID    int
	Power string

	Target   string // hostile land territory to take
	Staging  string // friendly coastal land territory where troops gather
	Embark   string // sea zone next to Staging, where shipping waits
	DropZone string // sea zone next to Target, from which troops land

	State PlanState

	// WantTroops is how large a landing force this plan is waiting for. It is
	// set from the defence at the target plus a margin, so a lightly held island
	// is taken quickly and a strong one is built up against.
	WantTroops     int
	WantTransports int

	// Units committed to this plan, by piece ID. A plan that loses all of them
	// has lost its army and must rebuild.
	Troops  []int
	Ships   []int
	Escorts []int

	// Route is the sea path from Embark to DropZone, recomputed every turn
	// because an enemy fleet can close it.
	Route []string

	// pendingLanding holds the transports that sailed this turn and still have
	// troops aboard, so the landing happens after the convoy has actually moved.
	pendingLanding []int

	// Reason records why a plan ended, so a transcript says what went wrong
	// rather than only that something did.
	Reason string

	// bestProgress is the furthest this plan has got, used to tell a slow
	// build-up from a stalled one.
	bestProgress int

	CreatedTurn int
	Restarts    int
	// LastProgress is the turn something actually changed. A plan that stops
	// making progress is abandoned rather than tying up units forever.
	LastProgress int
}

// maxPlanRestarts is how many times a plan may lose its whole force before the
// target is written off as beyond reach.
const maxPlanRestarts = 3

// planStallLimit is how many turns a plan may make no progress before it is
// abandoned, so a doomed operation eventually releases its units.
const planStallLimit = 12

// PlanBook holds the standing plans of every power.
//
// It lives on the controller rather than on the AI, because an NPCAIPlayer is
// built fresh for each turn in some code paths -- notably the web server, which
// constructs one per request. State kept on the AI would be discarded between
// turns, which is precisely the failure this is meant to fix.
type PlanBook struct {
	plans  map[string][]*AmphibiousPlan
	nextID int
}

// NewPlanBook creates an empty plan book.
func NewPlanBook() *PlanBook {
	return &PlanBook{plans: make(map[string][]*AmphibiousPlan), nextID: 1}
}

// For returns a power's plans.
func (pb *PlanBook) For(power string) []*AmphibiousPlan {
	if pb == nil {
		return nil
	}
	return pb.plans[power]
}

// Active returns a power's plans that are still being worked on.
func (pb *PlanBook) Active(power string) []*AmphibiousPlan {
	var active []*AmphibiousPlan
	for _, plan := range pb.For(power) {
		switch plan.State {
		case PlanForming, PlanEmbarked, PlanReady:
			active = append(active, plan)
		}
	}
	return active
}

// Add records a new plan and gives it an identity.
func (pb *PlanBook) Add(plan *AmphibiousPlan) *AmphibiousPlan {
	plan.ID = pb.nextID
	pb.nextID++
	pb.plans[plan.Power] = append(pb.plans[plan.Power], plan)
	return plan
}

// Targets returns the territories a power is already planning against, so two
// plans do not chase the same place.
func (pb *PlanBook) Targets(power string) map[string]bool {
	claimed := make(map[string]bool)
	for _, plan := range pb.Active(power) {
		claimed[plan.Target] = true
	}
	return claimed
}

// Committed reports whether a piece is already assigned to some plan, so
// ordinary movement does not wander off with an invasion force.
func (pb *PlanBook) Committed(power string, pieceID int) bool {
	for _, plan := range pb.Active(power) {
		for _, id := range plan.allPieces() {
			if id == pieceID {
				return true
			}
		}
	}
	return false
}

func (p *AmphibiousPlan) allPieces() []int {
	out := make([]int, 0, len(p.Troops)+len(p.Ships)+len(p.Escorts))
	out = append(out, p.Troops...)
	out = append(out, p.Ships...)
	out = append(out, p.Escorts...)
	return out
}

// Describe renders a plan for a transcript.
func (p *AmphibiousPlan) Describe() string {
	text := fmt.Sprintf("plan %d: take %s from %s via %s (%s; %d/%d troops, %d/%d transports, %d escorts)",
		p.ID, p.Target, p.Staging, p.DropZone, p.State,
		len(p.Troops), p.WantTroops, len(p.Ships), p.WantTransports, len(p.Escorts))
	if p.Reason != "" {
		text += " -- " + p.Reason
	}
	return text
}

// Review brings a plan up to date with the board: it drops units that no longer
// exist, notices that the target has already fallen, recomputes the sea route,
// and restarts or abandons a plan that has lost its force.
//
// Returns true if the plan is still worth working on.
func (p *AmphibiousPlan) Review(gc *GameController) bool {
	g := gc.Game

	if p.State == PlanSucceeded || p.State == PlanAbandoned {
		return false
	}

	// Somebody may have taken the target already -- possibly us, by land.
	if target, ok := g.Board[p.Target]; ok {
		if target.Owner != nil && target.Owner.Name == p.Power {
			p.State = PlanSucceeded
			return false
		}
	} else {
		p.abandon("target territory no longer exists")
		return false
	}

	// A port we no longer hold cannot mount an invasion.
	if staging, ok := g.Board[p.Staging]; !ok || staging.Owner == nil || staging.Owner.Name != p.Power {
		p.abandon("lost the staging port " + p.Staging)
		return false
	}

	hadForce := len(p.allPieces()) > 0
	p.Troops = survivors(g, p.Power, p.Troops)
	p.Ships = survivors(g, p.Power, p.Ships)
	p.Escorts = survivors(g, p.Power, p.Escorts)

	// Losing the entire committed force means starting again rather than
	// pretending an army still exists.
	if hadForce && len(p.allPieces()) == 0 {
		p.Restarts++
		if p.Restarts > maxPlanRestarts {
			p.abandon("force destroyed too many times")
			return false
		}
		p.State = PlanForming
		p.LastProgress = g.Turn
	}

	// Progress is the force actually coming together, not merely changing
	// state. Measuring only state transitions declared a plan stalled while it
	// was steadily marching troops to the port, and killed six plans out of
	// seven before any of them could sail.
	if score := p.progressScore(g); score > p.bestProgress {
		p.bestProgress = score
		p.LastProgress = g.Turn
	}

	if g.Turn-p.LastProgress > planStallLimit {
		p.abandon("no progress for many turns")
		return false
	}

	// Both the port and the route are re-chosen every turn, because an enemy
	// fleet can occupy the sea zone a convoy was going to gather in -- entering
	// that is combat, not a quiet assembly -- and can close the crossing behind
	// it. Recomputing is what lets a plan route around interference instead of
	// stalling against it.
	if p.State == PlanForming {
		if clear := clearEmbarkZone(g, p); clear != "" {
			p.Embark = clear
		}
	}
	p.Route = seaRouteFor(g, p.Embark, p.DropZone, g.Players[p.Power])

	p.advanceState(g)
	return true
}

// advanceState moves a plan along according to where its force actually is.
func (p *AmphibiousPlan) advanceState(g *models.Game) {
	drop := g.Board[p.DropZone]

	// Any loaded transport means the operation is under way.
	sailing, atDrop := 0, 0
	for _, shipID := range p.Ships {
		ship, ok := g.Pieces[shipID]
		if !ok || len(ship.Holding) == 0 {
			continue
		}
		sailing++
		if drop != nil && contains(drop.Pieces, shipID) {
			atDrop++
		}
	}

	switch {
	case atDrop > 0:
		// Beside the target: land on the next combat move.
		if p.State != PlanReady {
			p.State = PlanReady
			p.LastProgress = g.Turn
		}
	case sailing > 0:
		if p.State != PlanEmbarked {
			p.State = PlanEmbarked
			p.LastProgress = g.Turn
		}
	case p.forceAssembled(g) && len(p.Route) > 0:
		// Assembled at the port but not yet aboard; loading happens in the
		// noncombat phase.
		p.State = PlanForming
	default:
		p.State = PlanForming
	}
}

// progressScore measures how far along the operation is, so a build-up that is
// still making headway is not mistaken for a stalled one.
func (p *AmphibiousPlan) progressScore(g *models.Game) int {
	score := len(p.Troops) + len(p.Ships)*2

	if staging := g.Board[p.Staging]; staging != nil {
		for _, id := range p.Troops {
			if contains(staging.Pieces, id) {
				score += 2 // at the port is worth more than merely assigned
			}
		}
	}
	if embark := g.Board[p.Embark]; embark != nil {
		for _, id := range p.Ships {
			if contains(embark.Pieces, id) {
				score += 3
			}
		}
	}
	// Loaded and under way counts for a great deal more than sitting in port.
	for _, id := range p.Ships {
		if ship, ok := g.Pieces[id]; ok && len(ship.Holding) > 0 {
			score += 10
		}
	}
	if drop := g.Board[p.DropZone]; drop != nil {
		for _, id := range p.Ships {
			if contains(drop.Pieces, id) {
				score += 20
			}
		}
	}
	return score
}

func (p *AmphibiousPlan) abandon(reason string) {
	p.State = PlanAbandoned
	p.Reason = reason
}

// forceAssembled reports whether enough of the force is in place to sail.
//
// Deliberately not "everything the plan asked for". A single straggler that
// cannot reach the port -- a starting transport on the far side of the map --
// held entire operations in harbour indefinitely. A landing that goes with most
// of its strength beats one that never goes at all, which is also the "act
// quickly or gather more" trade-off: a plan sails once it can lift a worthwhile
// force, and the size it waits for came from the defence at the target.
func (p *AmphibiousPlan) forceAssembled(g *models.Game) bool {
	staging := g.Board[p.Staging]
	embark := g.Board[p.Embark]
	if staging == nil || embark == nil {
		return false
	}

	troopsAtPort := 0
	for _, id := range p.Troops {
		if contains(staging.Pieces, id) {
			troopsAtPort++
		}
	}

	lift := 0
	for _, id := range p.Ships {
		if !contains(embark.Pieces, id) {
			continue
		}
		if ship, ok := g.Pieces[id]; ok {
			lift += int(ship.Capacity)
		}
	}

	// Enough troops to be worth landing, and enough hulls to carry them.
	minimum := p.WantTroops / 2
	if minimum < 2 {
		minimum = 2
	}
	if troopsAtPort < minimum || lift < 2 {
		return false
	}

	carried := troopsAtPort
	if carried > lift {
		carried = lift
	}
	return carried >= minimum
}

// survivors keeps the piece IDs that still exist and still belong to us.
func survivors(g *models.Game, power string, ids []int) []int {
	kept := make([]int, 0, len(ids))
	for _, id := range ids {
		piece, ok := g.Pieces[id]
		if !ok || piece.Owner == nil || piece.Owner.Name != power {
			continue
		}
		kept = append(kept, id)
	}
	return kept
}

func contains(haystack []int, needle int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// seaRoute finds a path between two sea zones through water only.
//
// Recomputed every turn: this is the "recalculate the path in case of
// obstacles" part, and it is why a plan can go back to forming when a fleet
// blocks the strait it was counting on.
func seaRoute(g *models.Game, from, to string) []string {
	return seaRouteFor(g, from, to, nil)
}

// seaRouteFor finds a sea path, optionally avoiding zones held by enemy ships.
//
// A convoy sails during noncombat movement, and entering a sea zone occupied by
// an enemy fleet is a combat move -- so a route computed without regard to
// enemy shipping is a route the convoy cannot actually take. Germany planned
// crossings out of the Baltic straight through the Royal Navy sitting in the
// North Sea, and its transports simply never moved.
func seaRouteFor(g *models.Game, from, to string, power *models.Player) []string {
	start, ok := g.Board[from]
	if !ok {
		return nil
	}
	goal, ok := g.Board[to]
	if !ok {
		return nil
	}
	if start == goal {
		return []string{from}
	}

	type step struct {
		territory *models.Territory
		path      []string
	}
	seen := map[string]bool{from: true}
	queue := []step{{start, []string{from}}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, next := range current.territory.ConnectedTo {
			if seen[next.Name] || next.Terrain != models.Water {
				continue
			}
			if power != nil && next.Name != to && occupiedByEnemy(g, next, power) {
				continue // an enemy fleet closes this water to a quiet passage
			}
			seen[next.Name] = true
			path := append(append([]string{}, current.path...), next.Name)
			if next == goal {
				return path
			}
			queue = append(queue, step{next, path})
		}
	}
	return nil
}

// clearEmbarkZone picks a sea zone next to the staging port that is free of
// enemy ships and still has a crossing to the drop zone.
//
// Returns "" if none qualifies, in which case the plan keeps the zone it had
// and waits -- the enemy fleet may move on.
func clearEmbarkZone(g *models.Game, p *AmphibiousPlan) string {
	power := g.Players[p.Power]
	if power == nil {
		return ""
	}

	best, bestRoute := "", -1
	for _, candidate := range adjacentSeaZones(g, p.Staging) {
		zone := g.Board[candidate]
		if zone == nil || occupiedByEnemy(g, zone, power) {
			continue
		}
		route := seaRouteFor(g, candidate, p.DropZone, power)
		if len(route) == 0 {
			continue
		}
		if bestRoute == -1 || len(route) < bestRoute {
			best, bestRoute = candidate, len(route)
		}
	}
	return best
}

// occupiedByEnemy reports whether a territory holds units hostile to a power.
func occupiedByEnemy(g *models.Game, territory *models.Territory, power *models.Player) bool {
	for _, id := range territory.Pieces {
		piece, ok := g.Pieces[id]
		if !ok || piece.Owner == nil {
			continue
		}
		if piece.Owner != power && !areAllies(piece.Owner, power) {
			return true
		}
	}
	return false
}

// adjacentSeaZones returns the water territories touching a land territory.
func adjacentSeaZones(g *models.Game, landName string) []string {
	territory, ok := g.Board[landName]
	if !ok {
		return nil
	}
	var seas []string
	for _, neighbour := range territory.ConnectedTo {
		if neighbour.Terrain == models.Water {
			seas = append(seas, neighbour.Name)
		}
	}
	sort.Strings(seas)
	return seas
}

// defenderStrength is a rough measure of what is holding a territory, used to
// decide how large a landing force to gather.
func defenderStrength(g *models.Game, territoryName string) int {
	territory, ok := g.Board[territoryName]
	if !ok {
		return 0
	}
	units := g.Units()
	strength := 0
	for _, id := range territory.Pieces {
		piece, ok := g.Pieces[id]
		if !ok || units.For(piece).IsStructure {
			continue
		}
		strength += int(piece.Defend)
	}
	return strength
}

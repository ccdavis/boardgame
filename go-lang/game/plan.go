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

	// Codename is the operation's name, drawn from the side's vendored list.
	Codename string

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

	// WantEscort is the fighting strength the convoy should sail with, set from
	// what is actually in the way. A crossing nobody is guarding needs almost
	// nothing; one covered by a fleet needs enough to win the action, or the
	// transports are simply sunk.
	WantEscort int

	// Units committed to this plan, by piece ID. A plan that loses all of them
	// has lost its army and must rebuild.
	Troops  []int
	Ships   []int
	Escorts []int

	// Route is the sea path from Embark to DropZone, recomputed every turn
	// because an enemy fleet can close it.
	Route []string

	// Contested is how many zones on that route are held by an enemy fleet.
	Contested int

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
	plans    map[string][]*AmphibiousPlan
	defences map[string][]*DefencePlan
	naval    map[string][]*NavalPlan
	reserve  map[string]int
	nextID   int

	// sideOf answers which side a power fights for, so a new operation can be
	// christened from that side's codename list. Set by the controller; a
	// book built bare (tests) names everything from the default list.
	sideOf func(power string) string
}

// NewPlanBook creates an empty plan book.
func NewPlanBook() *PlanBook {
	return &PlanBook{
		plans:    make(map[string][]*AmphibiousPlan),
		defences: make(map[string][]*DefencePlan),
		naval:    make(map[string][]*NavalPlan),
		reserve:  make(map[string]int),
		nextID:   1,
	}
}

// Reserve is the money a power is holding back for expeditionary work it cannot
// yet afford.
//
// A per-turn share of production is not enough on its own. Italy's offence share
// of an eleven-IPC income is one IPC and a transport costs eight, so a plan that
// needed shipping waited out its stall limit and was abandoned -- every game,
// with the money going to infantry in the meantime. Saving the share instead of
// surrendering it lets a small power accumulate a ship over several turns.
func (pb *PlanBook) Reserve(power string) int {
	if pb == nil {
		return 0
	}
	return pb.reserve[power]
}

// SetReserve records what a power is holding back for its next ship.
func (pb *PlanBook) SetReserve(power string, amount int) {
	if pb == nil {
		return
	}
	if pb.reserve == nil {
		pb.reserve = make(map[string]int)
	}
	if amount < 0 {
		amount = 0
	}
	pb.reserve[power] = amount
}

// Defences returns a power's garrison plans.
func (pb *PlanBook) Defences(power string) []*DefencePlan {
	if pb == nil {
		return nil
	}
	return pb.defences[power]
}

// christen picks an operation's codename from its side's list.
func (pb *PlanBook) christen(power string, id int) string {
	side := ""
	if pb.sideOf != nil {
		side = pb.sideOf(power)
	}
	return operationName(side, id)
}

// AddDefence records a new garrison plan.
func (pb *PlanBook) AddDefence(plan *DefencePlan) *DefencePlan {
	plan.ID = pb.nextID
	pb.nextID++
	plan.Codename = pb.christen(plan.Power, plan.ID)
	pb.defences[plan.Power] = append(pb.defences[plan.Power], plan)
	return plan
}

// Naval returns a power's squadrons.
func (pb *PlanBook) Naval(power string) []*NavalPlan {
	if pb == nil {
		return nil
	}
	return pb.naval[power]
}

// AddNaval records a new squadron.
func (pb *PlanBook) AddNaval(plan *NavalPlan) *NavalPlan {
	plan.ID = pb.nextID
	pb.nextID++
	plan.Codename = pb.christen(plan.Power, plan.ID)
	pb.naval[plan.Power] = append(pb.naval[plan.Power], plan)
	return plan
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
	plan.Codename = pb.christen(plan.Power, plan.ID)
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
// ordinary movement does not wander off with an invasion force, a garrison, or
// a squadron under orders.
func (pb *PlanBook) Committed(power string, pieceID int) bool {
	if pb == nil {
		return false
	}
	for _, plan := range pb.Active(power) {
		for _, id := range plan.allPieces() {
			if id == pieceID {
				return true
			}
		}
	}
	for _, plan := range pb.Defences(power) {
		for _, id := range plan.Garrison {
			if id == pieceID {
				return true
			}
		}
	}
	for _, plan := range pb.Naval(power) {
		for _, id := range plan.Ships {
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
	text := fmt.Sprintf("Operation %s (plan %d): take %s from %s via %s (%s; %d/%d troops, %d/%d transports, %d escorts)",
		p.title(), p.ID, p.Target, p.Staging, p.DropZone, p.State,
		len(p.Troops), p.WantTroops, len(p.Ships), p.WantTransports, len(p.Escorts))
	if p.Reason != "" {
		text += " -- " + p.Reason
	}
	return text
}

func (p *AmphibiousPlan) title() string {
	if p.Codename == "" {
		return "UNNAMED"
	}
	return p.Codename
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

	// Somebody may have taken the target already -- possibly us, by land, or an
	// ally by any route. An ally's conquest ends the plan too: pressing on
	// would land troops against a friendly garrison, and attacking an ally is
	// exactly what the rules forbid.
	if target, ok := g.Board[p.Target]; ok {
		if target.Owner != nil && target.Owner.Name == p.Power {
			p.State = PlanSucceeded
			return false
		}
		if power := g.Players[p.Power]; power != nil && areAllies(target.Owner, power) {
			p.abandon("an ally holds " + p.Target)
			return false
		}
	} else {
		p.abandon("target territory no longer exists")
		return false
	}

	// A port we no longer hold cannot mount an invasion -- but that only
	// matters while the invasion is still mounting. A convoy already at sea
	// carries everything it needs; abandoning it because the province behind
	// it fell threw away a fully loaded operation mid-crossing (seen in game
	// transcripts: eight troops and three transports scuttled on the news that
	// Manchuria was lost).
	if p.State == PlanForming {
		if staging, ok := g.Board[p.Staging]; !ok || staging.Owner == nil || staging.Owner.Name != p.Power {
			p.abandon("lost the staging port " + p.Staging)
			return false
		}
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
		if zone := chooseEmbarkZone(g, p); zone != "" {
			p.Embark = zone
		}
	}
	power := g.Players[p.Power]
	route, contested := seaRouteCost(g, p.Embark, p.DropZone, power)
	p.Route = route
	p.Contested = contested
	p.WantEscort = escortNeeded(g, p, power)

	// Re-read the defence while the force assembles. WantTroops was set once
	// at proposal, so a plan drawn against a thinly held coast in round one
	// sailed ten rounds later with eight troops against what had become a
	// 159-unit fortress -- twelve such landings at Eastern US across six
	// games, every one annihilated, each abandonment starting the next. A
	// defence the largest liftable force cannot beat ends the plan; a defence
	// that merely grew raises the force to match while still liftable. Both
	// limits scale with the clock: a power that must win soon lifts more and
	// judges fewer targets hopeless.
	if p.State == PlanForming || p.State == PlanEmbarked {
		troopCap := maxPlanTroopsFor(strategicPressure(g, power))
		defence := defenderStrength(g, p.Target)
		if defence > hopelessDefenceFor(troopCap) {
			p.abandon(fmt.Sprintf("%s is too strongly held (defence %d)", p.Target, defence))
			return false
		}
		if want := troopsNeeded(defence, nil, troopCap); want > p.WantTroops {
			p.WantTroops = want
			p.WantTransports = (want + transportCapacity - 1) / transportCapacity
		}
	}

	p.advanceState(g)
	return true
}

// hopelessDefenceFor is the defensive strength beyond which no landing the
// given troop cap can lift could expect to win, whatever the escort. The
// troops attack at roughly one pip each; a defence several times the cap is
// not a target, it is a deterrent.
func hopelessDefenceFor(troopCap int) int {
	return hopelessDefenceMultiple * troopCap
}

const hopelessDefenceMultiple = 3

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
	default:
		// Nothing loaded: still forming, whether or not the force is at the
		// port -- loading happens in the noncombat phase.
		p.State = PlanForming
	}
}

// escortNeeded sizes the covering force from the opposition actually on the
// route and over the landing.
//
// This is deliberately proportionate. Sending a battle fleet to escort a
// crossing nobody is watching wastes the production that should be buying
// troops; sending bare transports into guarded water loses them. Where there is
// no enemy navy in the way the requirement falls to almost nothing.
func escortNeeded(g *models.Game, p *AmphibiousPlan, power *models.Player) int {
	if power == nil {
		return 0
	}

	opposition := 0
	for _, name := range p.Route {
		opposition += enemyNavalStrength(g, g.Board[name], power)
	}
	// Whatever covers the landing itself matters most: the convoy has to sit
	// there while the troops go ashore.
	opposition += enemyNavalStrength(g, g.Board[p.DropZone], power) * 2

	if opposition == 0 {
		return 0 // an unguarded crossing needs no fleet
	}
	// Enough to expect to win rather than merely trade, but bounded: past a
	// point the answer is a different target, not a bigger fleet. Without a
	// ceiling a heavily patrolled crossing demanded a navy that consumed the
	// whole war economy and left no army to land.
	needed := opposition*3/2 + 2
	if needed > maxEscortStrength {
		needed = maxEscortStrength
	}
	return needed
}

// maxEscortStrength caps what one operation will wait for. Roughly a handful of
// warships; beyond that the crossing is not the problem, the choice of target is.
const maxEscortStrength = 24

// EscortStrength is what the plan currently has to fight with.
func (p *AmphibiousPlan) EscortStrength(g *models.Game) int {
	return friendlyNavalStrength(g, p.Escorts)
}

// hasBombardier reports whether any committed escort can shell the beach.
func (p *AmphibiousPlan) hasBombardier(g *models.Game) bool {
	units := g.Units()
	for _, id := range p.Escorts {
		if piece, ok := g.Pieces[id]; ok && units.For(piece).CanBombard {
			return true
		}
	}
	return false
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
	if carried < minimum {
		return false
	}

	// Do not sail into a guarded crossing with nothing to fight back with.
	// Where nobody is watching the water this costs nothing, because the
	// requirement is zero.
	return p.EscortStrength(g) >= p.WantEscort
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

// seaRouteFor finds a sea path, optionally avoiding zones held by enemy ships.
//
// A convoy sails during noncombat movement, and entering a sea zone occupied by
// an enemy fleet is a combat move -- so a route computed without regard to
// enemy shipping is a route the convoy cannot actually take. Germany planned
// crossings out of the Baltic straight through the Royal Navy sitting in the
// North Sea, and its transports simply never moved.
func seaRouteFor(g *models.Game, from, to string, power *models.Player) []string {
	route, _ := seaRouteCost(g, from, to, power)
	return route
}

// contestedPenalty is how much a sea zone held by an enemy fleet adds to the
// cost of a route.
//
// It is a preference, not a prohibition. Treating enemy shipping as impassable
// meant a single destroyer parked off a coast made a landing there impossible
// forever, which is not how a navy works: you either go round, or you bring
// something to fight with. The penalty is large enough that a clear route
// several zones longer still wins, and small enough that a defended crossing
// remains on the table when there is no alternative.
const contestedPenalty = 6

// seaRouteCost finds the cheapest sea path and reports how much of it is
// contested, so a plan knows whether it must fight its way through.
//
// Cost is one per open sea zone and contestedPenalty per zone held by an enemy
// fleet. The second return is the number of contested zones on the chosen route.
func seaRouteCost(g *models.Game, from, to string, power *models.Player) ([]string, int) {
	start, ok := g.Board[from]
	if !ok {
		return nil, 0
	}
	goal, ok := g.Board[to]
	if !ok {
		return nil, 0
	}
	if start == goal {
		return []string{from}, 0
	}

	// Small graph, so a simple settled-set search is clearer than a heap.
	dist := map[string]int{from: 0}
	prev := map[string]string{}
	settled := map[string]bool{}

	for {
		current, best := "", -1
		for name, d := range dist {
			if !settled[name] && (best == -1 || d < best) {
				current, best = name, d
			}
		}
		if current == "" {
			break
		}
		if current == to {
			break
		}
		settled[current] = true

		territory := g.Board[current]
		if territory == nil {
			continue
		}
		for _, next := range territory.ConnectedTo {
			if next.Terrain != models.Water && next.Name != to {
				continue
			}
			step := 1
			if power != nil && occupiedByEnemy(g, next, power) {
				step += contestedPenalty
			}
			if known, seen := dist[next.Name]; !seen || best+step < known {
				dist[next.Name] = best + step
				prev[next.Name] = current
			}
		}
	}

	if _, reached := dist[to]; !reached {
		return nil, 0
	}

	route := []string{to}
	for at := prev[to]; at != "" && at != from; at = prev[at] {
		route = append([]string{at}, route...)
	}

	contested := 0
	if power != nil {
		for _, name := range route {
			if zone := g.Board[name]; zone != nil && occupiedByEnemy(g, zone, power) {
				contested++
			}
		}
	}
	return route, contested
}

// occupiedByEnemy reports whether a territory holds units hostile to a power.
func occupiedByEnemy(g *models.Game, territory *models.Territory, power *models.Player) bool {
	return enemyNavalStrength(g, territory, power) > 0 || enemyPresent(g, territory, power)
}

// enemyPresent reports any hostile unit at all.
func enemyPresent(g *models.Game, territory *models.Territory, power *models.Player) bool {
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

// enemyNavalStrength measures the hostile fighting power in a sea zone.
//
// Used to size an escort: what matters is not whether an enemy is present but
// how much of a fight it can put up.
func enemyNavalStrength(g *models.Game, territory *models.Territory, power *models.Player) int {
	if territory == nil {
		return 0
	}
	units := g.Units()
	strength := 0
	for _, id := range territory.Pieces {
		piece, ok := g.Pieces[id]
		if !ok || piece.Owner == nil {
			continue
		}
		if piece.Owner == power || areAllies(piece.Owner, power) {
			continue
		}
		if units.For(piece).IsStructure {
			continue
		}
		strength += combatValue(piece)
	}
	return strength
}

// combatValue rates a unit's usefulness in a naval action. Aircraft aboard a
// carrier count, which is why a carrier is a credible escort.
func combatValue(piece *models.Piece) int {
	return int(piece.Attack) + int(piece.Defend) + 2*len(piece.Holding)
}

// friendlyNavalStrength measures what a power has to fight with in a zone.
func friendlyNavalStrength(g *models.Game, ids []int) int {
	strength := 0
	for _, id := range ids {
		if piece, ok := g.Pieces[id]; ok {
			strength += combatValue(piece)
		}
	}
	return strength
}

// chooseEmbarkZone picks the sea zone next to the staging port to gather in.
//
// Clear water is strongly preferred, but a contested zone is allowed rather
// than leaving the plan with nowhere to assemble -- the route cost already
// makes an unguarded approach the first choice.
func chooseEmbarkZone(g *models.Game, p *AmphibiousPlan) string {
	power := g.Players[p.Power]
	if power == nil {
		return ""
	}

	best, bestCost := "", -1
	for _, candidate := range adjacentSeaZones(g, p.Staging) {
		zone := g.Board[candidate]
		if zone == nil {
			continue
		}
		route, _ := seaRouteCost(g, candidate, p.DropZone, power)
		if len(route) == 0 {
			continue
		}
		cost := len(route)
		if occupiedByEnemy(g, zone, power) {
			cost += contestedPenalty
		}
		if bestCost == -1 || cost < bestCost {
			best, bestCost = candidate, cost
		}
	}
	return best
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
// decide how large a landing force to gather. A neutral counts the garrison
// it would mobilise when invaded, so a landing on an "empty" neutral is sized
// against the army that will actually meet it on the beach.
func defenderStrength(g *models.Game, territoryName string) int {
	territory, ok := g.Board[territoryName]
	if !ok {
		return 0
	}
	units := g.Units()
	strength := 0
	for _, piece := range expectedDefenders(g, territory) {
		if units.For(piece).IsStructure {
			continue
		}
		strength += int(piece.Defend)
	}
	return strength
}

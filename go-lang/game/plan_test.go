package game

import (
	"testing"

	"boardgame/models"
)

// invasionBoard is a small sea-and-land board:
//
//	Home (Germany) -- Home Sea -- Mid Sea -- Island Sea -- Island (UK)
//
// Home and Inland are one landmass; Island can only be reached by sea.
func invasionBoard(t *testing.T) (*models.Game, *GameController) {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "UK"}
	for _, name := range g.PlayerOrder {
		player := g.GetOrCreatePlayer(name)
		player.TakesTurns = true
		player.IPCs = 60
	}
	g.Players["Germany"].Side = "Axis"
	g.Players["UK"].Side = "Allies"

	g.AddTerritory("Home", models.Land, "Germany", 8)
	g.AddTerritory("Inland", models.Land, "Germany", 3)
	g.AddTerritory("Island", models.Land, "UK", 6)
	g.AddTerritory("Home Sea", models.Water, "Germany", 0)
	g.AddTerritory("Mid Sea", models.Water, "Germany", 0)
	g.AddTerritory("Island Sea", models.Water, "UK", 0)

	g.ConnectTerritories("Home", "Inland")
	g.ConnectTerritories("Home", "Home Sea")
	g.ConnectTerritories("Home Sea", "Mid Sea")
	g.ConnectTerritories("Mid Sea", "Island Sea")
	g.ConnectTerritories("Island Sea", "Island")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 10)
	g.GlobalPieceTemplates["transport"].Capacity = 2
	g.GlobalPieceTemplates["transport"].CanCarry = []string{"infantry"}
	g.AddPieceTemplate("destroyer", models.Water, 2, 2, 2, 8)
	g.AddPieceTemplate("carrier", models.Water, 2, 0, 1, 24)
	g.GlobalPieceTemplates["carrier"].Capacity = 2
	g.GlobalPieceTemplates["carrier"].CanCarry = []string{"fighter"}

	controller := NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	return g, controller
}

// A carrier has capacity but carries aircraft. Picking the first ship with a
// hold made the computer players buy carriers to invade with, load nothing into
// them, and stall in port forever.
func TestShippingNames_PrefersTroopTransportOverCarrier(t *testing.T) {
	g, _ := invasionBoard(t)

	transport, escort := shippingNames(g)
	if transport != "transport" {
		t.Errorf("chose %q as the troop transport, want transport", transport)
	}
	if escort != "destroyer" {
		t.Errorf("chose %q as the escort, want destroyer", escort)
	}
}

// A plan should be proposed against land that cannot be walked to.
func TestProposePlan_TargetsWhatCannotBeWalkedTo(t *testing.T) {
	g, controller := invasionBoard(t)
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)

	plan := npc.ProposePlan(controller, g.Players["Germany"])
	if plan == nil {
		t.Fatal("no plan proposed for an island held by the enemy")
	}
	if plan.Target != "Island" {
		t.Errorf("target %q, want Island", plan.Target)
	}
	if plan.Staging != "Home" {
		t.Errorf("staging %q, want Home -- the only coastal territory held", plan.Staging)
	}
	if plan.DropZone != "Island Sea" {
		t.Errorf("drop zone %q, want Island Sea", plan.DropZone)
	}
}

// Somewhere reachable on foot is not an amphibious problem.
func TestProposePlan_IgnoresTargetsReachableOverland(t *testing.T) {
	g, controller := invasionBoard(t)
	// Hand the island to Germany so nothing is left but a land neighbour.
	models.ChangeOwnership(g.Board["Island"], g.Players["Germany"])
	models.ChangeOwnership(g.Board["Inland"], g.Players["UK"])

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	if plan := npc.ProposePlan(controller, g.Players["Germany"]); plan != nil {
		t.Errorf("proposed an amphibious plan against %q, which can be walked to", plan.Target)
	}
}

// The whole point of the plan book: state that outlives a turn, and outlives
// the AI object, since a fresh NPCAIPlayer is built per turn in some paths.
func TestPlanBook_SurvivesANewAIInstance(t *testing.T) {
	g, controller := invasionBoard(t)

	first := NewSeededNPCAIPlayer("Germany", "normal", 1)
	first.ReviewPlans(controller, g.Players["Germany"], NewGameTranscript("t"))
	if len(controller.Plans.Active("Germany")) == 0 {
		t.Fatal("no plan was recorded")
	}
	planID := controller.Plans.Active("Germany")[0].ID

	// A completely new AI, as the web server builds for every request.
	second := NewSeededNPCAIPlayer("Germany", "normal", 99)
	second.ReviewPlans(controller, g.Players["Germany"], NewGameTranscript("t"))

	active := controller.Plans.Active("Germany")
	if len(active) != 1 || active[0].ID != planID {
		t.Errorf("the standing plan was lost when the AI was rebuilt: %v", active)
	}
}

// Losing the whole committed force restarts the plan rather than leaving it
// believing it still has an army.
func TestPlan_RestartsWhenItsForceIsDestroyed(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Home", "infantry", 3); err != nil {
		t.Fatalf("placing troops: %v", err)
	}
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))

	plan := controller.Plans.Active("Germany")[0]
	if len(plan.Troops) == 0 {
		t.Fatal("no troops were committed to the plan")
	}

	// Sink the lot.
	for _, id := range plan.allPieces() {
		delete(g.Pieces, id)
	}
	plan.Review(controller)

	if plan.Restarts != 1 {
		t.Errorf("restarts = %d, want 1", plan.Restarts)
	}
	if plan.State != PlanForming {
		t.Errorf("state = %v, want forming after losing everything", plan.State)
	}
	if len(plan.allPieces()) != 0 {
		t.Error("destroyed units are still committed to the plan")
	}
}

// A plan gives up rather than tying units to a hopeless target forever.
func TestPlan_AbandonsAfterRepeatedLosses(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	plan := controller.Plans.Active("Germany")[0]

	for i := 0; i <= maxPlanRestarts; i++ {
		if err := g.PlacePieces("Home", "infantry", 1); err != nil {
			t.Fatalf("placing troops: %v", err)
		}
		npc.assignUnits(controller, player, plan)
		for _, id := range plan.allPieces() {
			delete(g.Pieces, id)
		}
		plan.Review(controller)
	}

	if plan.State != PlanAbandoned {
		t.Errorf("state = %v, want abandoned after %d losses", plan.State, maxPlanRestarts+1)
	}
	if plan.Reason == "" {
		t.Error("an abandoned plan should record why")
	}
}

// Taking the target by other means completes the plan.
func TestPlan_SucceedsWhenTheTargetIsTaken(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	plan := controller.Plans.Active("Germany")[0]

	models.ChangeOwnership(g.Board["Island"], player)
	plan.Review(controller)

	if plan.State != PlanSucceeded {
		t.Errorf("state = %v, want succeeded once the target is ours", plan.State)
	}
}

// Enemy shipping makes a passage expensive, not impossible.
//
// Treating it as impassable meant one destroyer parked off a coast forbade any
// landing there for the rest of the game. A convoy should prefer open water and
// go round -- but where there is no way round, it should still find the route
// and be told the crossing is contested, so it can bring something to fight
// with.
func TestSeaRoute_PrefersOpenWaterButStillFindsAContestedOne(t *testing.T) {
	g, _ := invasionBoard(t)
	germany := g.Players["Germany"]

	route, contested := seaRouteCost(g, "Home Sea", "Island Sea", germany)
	if len(route) == 0 {
		t.Fatal("expected a route before any blockade")
	}
	if contested != 0 {
		t.Errorf("open water reported %d contested zones", contested)
	}

	// Park a UK destroyer in the only intervening sea zone.
	if err := g.PlacePieces("Mid Sea", "destroyer", 1); err != nil {
		t.Fatalf("placing blockade: %v", err)
	}
	for _, id := range g.Board["Mid Sea"].Pieces {
		g.Pieces[id].Owner = g.Players["UK"]
	}

	route, contested = seaRouteCost(g, "Home Sea", "Island Sea", germany)
	if len(route) == 0 {
		t.Fatal("a guarded sea zone should not make the crossing impossible")
	}
	if contested == 0 {
		t.Error("the route runs through a guarded zone but was not reported contested")
	}
}

// Where a way round exists, take it.
func TestSeaRoute_GoesAroundAGuardedZone(t *testing.T) {
	g, _ := invasionBoard(t)
	germany := g.Players["Germany"]

	// An alternative passage: Home Sea -- Far Sea -- Island Sea.
	g.AddTerritory("Far Sea", models.Water, "Germany", 0)
	g.ConnectTerritories("Home Sea", "Far Sea")
	g.ConnectTerritories("Far Sea", "Island Sea")

	if err := g.PlacePieces("Mid Sea", "destroyer", 1); err != nil {
		t.Fatalf("placing blockade: %v", err)
	}
	for _, id := range g.Board["Mid Sea"].Pieces {
		g.Pieces[id].Owner = g.Players["UK"]
	}

	route, contested := seaRouteCost(g, "Home Sea", "Island Sea", germany)
	if contested != 0 {
		t.Errorf("took a contested route %v when a clear one existed", route)
	}
	for _, name := range route {
		if name == "Mid Sea" {
			t.Errorf("route %v sails through the blockade instead of round it", route)
		}
	}
}

// The covering force is sized to what is actually in the way: nothing where
// nobody is watching, and enough to win where somebody is.
func TestEscortNeeded_ScalesWithOpposition(t *testing.T) {
	g, controller := invasionBoard(t)
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, g.Players["Germany"], NewGameTranscript("t"))
	plan := controller.Plans.Active("Germany")[0]

	if plan.WantEscort != 0 {
		t.Errorf("an unguarded crossing asked for %d escort strength, want 0", plan.WantEscort)
	}

	// Cover the landing with a battleship-grade force.
	if err := g.PlacePieces(plan.DropZone, "destroyer", 2); err != nil {
		t.Fatalf("placing defenders: %v", err)
	}
	for _, id := range g.Board[plan.DropZone].Pieces {
		g.Pieces[id].Owner = g.Players["UK"]
	}
	plan.Review(controller)

	if plan.WantEscort <= 0 {
		t.Error("a guarded landing should call for a covering force")
	}
}

// A carrier is a credible escort because its aircraft do the fighting.
func TestCombatValue_CountsCarriedAircraft(t *testing.T) {
	empty := &models.Piece{Name: "carrier", Attack: 0, Defend: 1}
	loaded := &models.Piece{Name: "carrier", Attack: 0, Defend: 1, Holding: []int{1, 2}}

	if combatValue(loaded) <= combatValue(empty) {
		t.Error("a carrier with aircraft aboard should be worth more than an empty one")
	}
}

// Units reserved by a plan are held back from ordinary movement, or the general
// logic walks the invasion force off the quayside every turn.
func TestPlanBook_CommittedUnitsAreReserved(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Home", "infantry", 3); err != nil {
		t.Fatalf("placing troops: %v", err)
	}
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))

	plan := controller.Plans.Active("Germany")[0]
	if len(plan.Troops) == 0 {
		t.Fatal("no troops committed")
	}
	committed := plan.Troops[0]

	if !controller.Plans.Committed("Germany", committed) {
		t.Error("a committed unit is not reported as reserved")
	}
	// The quartermaster's surplus must not include the committed unit.
	for _, piece := range npc.surplusIn(controller, player, g.Board["Home"], 0) {
		if piece.ID == committed {
			t.Error("a committed unit was offered to general movement")
		}
	}
}

// A power too poor to buy a ship in one turn must save until it can.
//
// The expeditionary share of a small income is smaller than a transport, so
// spending it or losing it each turn means the shipping is never bought and the
// plan is abandoned for want of progress. Italy did this in every game.
func TestPurchase_SavesTowardsShippingItCannotAffordYet(t *testing.T) {
	g, controller := poorInvaderBoard(t)
	player := g.Players["Italy"]

	npc := NewSeededNPCAIPlayer("Italy", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	if len(controller.Plans.Active("Italy")) == 0 {
		t.Fatal("no plan to buy shipping for")
	}

	// Ten turns of a ten-IPC income. One turn's expeditionary share is two IPCs
	// against a transport at eight, so this only works if the share is saved
	// instead of falling through to the infantry the general buildup would buy.
	const income = 10
	for turn := 1; turn <= 10; turn++ {
		player.IPCs += income
		if err := npc.PurchasePhase(controller, NewGameTranscript("t")); err != nil {
			t.Fatalf("turn %d purchase phase: %v", turn, err)
		}
		if purchasedCount(g, "Italy", "transport") > 0 {
			return
		}
		if saved := controller.Plans.Reserve("Italy"); player.IPCs < saved {
			t.Fatalf("turn %d: held back %d IPCs but only %d remain in the treasury",
				turn, saved, player.IPCs)
		}
	}
	t.Errorf("ten turns of income never bought a transport: treasury %d, reserve %d, bought %d infantry",
		player.IPCs, controller.Plans.Reserve("Italy"), purchasedCount(g, "Italy", "infantry"))
}

// With no plan wanting anything, nothing is held back -- the money belongs to
// the army rather than to a purse for an operation that does not exist.
func TestPurchase_HoldsNothingBackWithoutAPlan(t *testing.T) {
	g, controller := poorInvaderBoard(t)
	// Hand the island over so there is nothing left to invade.
	models.ChangeOwnership(g.Board["Island"], g.Players["Italy"])
	player := g.Players["Italy"]
	player.IPCs = 30

	npc := NewSeededNPCAIPlayer("Italy", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	if err := npc.PurchasePhase(controller, NewGameTranscript("t")); err != nil {
		t.Fatalf("purchase phase: %v", err)
	}
	if saved := controller.Plans.Reserve("Italy"); saved != 0 {
		t.Errorf("held back %d IPCs with no plan asking for anything", saved)
	}
}

// poorInvaderBoard is an invasion that a small income has to save up for.
//
// Italy's posture puts a fifth of production into expeditions, and infantry
// costs one IPC, so the general buildup can consume every last IPC left to it.
// That is the situation the war chest exists for: without it the treasury never
// grows and the transport is never affordable.
func poorInvaderBoard(t *testing.T) (*models.Game, *GameController) {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Italy", "UK"}
	for _, name := range g.PlayerOrder {
		player := g.GetOrCreatePlayer(name)
		player.TakesTurns = true
	}
	g.Players["Italy"].Side = "Axis"
	g.Players["UK"].Side = "Allies"

	g.AddTerritory("Home", models.Land, "Italy", 2)
	g.AddTerritory("Inland", models.Land, "Italy", 1)
	g.AddTerritory("Island", models.Land, "UK", 6)
	g.AddTerritory("Home Sea", models.Water, "Italy", 0)
	g.AddTerritory("Island Sea", models.Water, "UK", 0)

	g.ConnectTerritories("Home", "Inland")
	g.ConnectTerritories("Home", "Home Sea")
	g.ConnectTerritories("Home Sea", "Island Sea")
	g.ConnectTerritories("Island Sea", "Island")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 1)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	g.GlobalPieceTemplates["transport"].Capacity = 2
	g.GlobalPieceTemplates["transport"].CanCarry = []string{"infantry"}
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)

	// A coastal yard: a power with no way to launch a ship rightly refuses to
	// buy one, and this test is about affording one, not launching one.
	if err := g.PlacePieces("Home", "factory", 1); err != nil {
		t.Fatalf("placing factory: %v", err)
	}
	if err := g.PlacePieces("Home", "infantry", 8); err != nil {
		t.Fatalf("placing troops: %v", err)
	}

	controller := NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	return g, controller
}

func purchasedCount(g *models.Game, power, unit string) int {
	count := 0
	for _, pending := range g.PurchasedUnits[power] {
		if pending.Type == unit {
			count++
		}
	}
	return count
}

// A plan whose target an ally has captured is over, not a new enemy.
//
// Review only retired a plan when its own power held the target, so if an
// ally got there first the plan pressed on -- and the landing created a
// battle against the ally, which the rules flatly forbid.
func TestPlan_RetiresWhenAnAllyTakesTheTarget(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	// A second Axis power that will capture the island first.
	japan := g.GetOrCreatePlayer("Japan")
	japan.Side = "Axis"
	japan.TakesTurns = true

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	active := controller.Plans.Active("Germany")
	if len(active) == 0 {
		t.Fatal("no plan formed")
	}
	plan := active[0]

	models.ChangeOwnership(g.Board[plan.Target], japan)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))

	if got := controller.Plans.Active("Germany"); len(got) > 0 && got[0].ID == plan.ID {
		t.Errorf("plan %d still active against %s, which an ally now holds", plan.ID, plan.Target)
	}
	if plan.State == PlanSucceeded {
		t.Error("an ally's conquest is not this plan's success")
	}
}

// A convoy already at sea is not scuttled by news from home.
//
// Losing the staging port used to abandon the plan in any state, which threw
// away fully loaded operations mid-crossing. The port only matters while the
// force is still assembling there.
func TestPlan_SurvivesLosingThePortOnceAtSea(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Home", "infantry", 2); err != nil {
		t.Fatalf("placing troops: %v", err)
	}
	if err := g.PlacePieces("Mid Sea", "transport", 1); err != nil {
		t.Fatalf("placing transport: %v", err)
	}
	transportID := g.Board["Mid Sea"].Pieces[0]
	g.Pieces[transportID].Owner = player

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	plan := controller.Plans.Active("Germany")[0]

	// Put a troop aboard by hand and let the plan notice it is embarked.
	troopID := plan.Troops[0]
	g.Board["Home"].Pieces = removeID(g.Board["Home"].Pieces, troopID)
	g.Pieces[transportID].Holding = append(g.Pieces[transportID].Holding, troopID)
	plan.Ships = []int{transportID}
	plan.Review(controller)
	if plan.State != PlanEmbarked {
		t.Fatalf("state = %v, want at sea", plan.State)
	}

	// The home port falls while the convoy is mid-crossing.
	models.ChangeOwnership(g.Board["Home"], g.Players["UK"])
	if !plan.Review(controller) {
		t.Fatalf("plan was retired: %v (%s)", plan.State, plan.Reason)
	}
	if plan.State == PlanAbandoned {
		t.Errorf("a convoy at sea was abandoned because %s", plan.Reason)
	}

	// But a plan still forming, whose port falls, is rightly abandoned.
	forming := &AmphibiousPlan{
		Power: "Germany", Target: "Island", Staging: "Home",
		Embark: "Home Sea", DropZone: "Island Sea", State: PlanForming,
	}
	controller.Plans.Add(forming)
	forming.Review(controller)
	if forming.State != PlanAbandoned {
		t.Errorf("a forming plan kept a staging port the enemy holds (state %v)", forming.State)
	}
}

func removeID(ids []int, drop int) []int {
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}

// Troops come from the staging port's landmass, not from garrisons that can
// never march to it.
func TestAssignUnits_TakesTroopsOnlyFromTheStagingLandmass(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	// A German-held island off on its own, with a garrison.
	g.AddTerritory("Outpost", models.Land, "Germany", 1)
	g.AddTerritory("Outpost Sea", models.Water, "Neutral", 0)
	g.ConnectTerritories("Outpost", "Outpost Sea")
	g.ConnectTerritories("Outpost Sea", "Home Sea")
	if err := g.PlacePieces("Outpost", "infantry", 3); err != nil {
		t.Fatalf("garrisoning outpost: %v", err)
	}
	if err := g.PlacePieces("Home", "infantry", 2); err != nil {
		t.Fatalf("placing troops: %v", err)
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	plans := controller.Plans.Active("Germany")
	if len(plans) == 0 {
		t.Fatal("no plan formed")
	}
	plan := plans[0]

	outpost := g.Board["Outpost"]
	for _, id := range plan.Troops {
		for _, garrisoned := range outpost.Pieces {
			if id == garrisoned {
				t.Fatalf("plan staged at %s committed a troop from Outpost, an island its army cannot leave",
					plan.Staging)
			}
		}
	}
}

// A plan re-reads the defence while its force assembles: a target that has
// grown into a fortress ends the plan, and a defence that merely grew raises
// the wanted force to match.
//
// WantTroops used to be fixed at proposal, so a plan drawn against a thin
// coast in round one sailed ten rounds later with eight troops against what
// had become a 159-unit fortress -- twelve such landings in six games, every
// one annihilated.
func TestPlan_ResizesAgainstAGrowingDefence(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Island", "infantry", 2); err != nil {
		t.Fatalf("garrisoning island: %v", err)
	}
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	plan := controller.Plans.Active("Germany")[0]
	firstWant := plan.WantTroops

	// The defence doubles: the plan wants more troops, but stays alive.
	g.PlacePieces("Island", "infantry", 3)
	plan.Review(controller)
	if plan.State == PlanAbandoned {
		t.Fatalf("a beatable defence ended the plan: %s", plan.Reason)
	}
	if plan.WantTroops <= firstWant {
		t.Errorf("defence grew but WantTroops stayed at %d", plan.WantTroops)
	}

	// The defence becomes hopeless. A fresh plan spends its reconnaissance
	// turns watching before it will pass that judgement, so move the clock
	// past the watch window first.
	g.PlacePieces("Island", "infantry", 20)
	g.Turn += planReconTurns
	if plan.Review(controller) {
		t.Error("a plan against a fortress no landing can beat is still being worked")
	}
	if plan.State != PlanAbandoned {
		t.Errorf("state = %v, want abandoned", plan.State)
	}
	if !plan.HopelessTarget {
		t.Error("a hopeless abandonment should be marked for the cooling-off book")
	}
}

// A fortress is not even proposed against.
func TestProposePlan_SkipsHopelessTargets(t *testing.T) {
	g, controller := invasionBoard(t)
	if err := g.PlacePieces("Island", "infantry", 20); err != nil { // defence 40
		t.Fatalf("garrisoning: %v", err)
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	if plan := npc.ProposePlan(controller, g.Players["Germany"]); plan != nil {
		t.Errorf("proposed %q against defence %d", plan.Target, defenderStrength(g, plan.Target))
	}
}

// A landing wants one ship that can shell the beach -- but lift comes first,
// and one gun is enough.
func TestPlanPurchases_WantsOneBombardier(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)
	if err := g.PlacePieces("Home", "factory", 1); err != nil {
		t.Fatalf("factory: %v", err)
	}
	if err := g.PlacePieces("Home", "infantry", 4); err != nil {
		t.Fatalf("troops: %v", err)
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(controller, player, NewGameTranscript("t"))
	plans := controller.Plans.Active("Germany")
	if len(plans) == 0 {
		t.Fatal("no plan formed")
	}
	plan := plans[0]

	// Short of transports: the shopping list must not ask for the gun yet.
	if wants := npc.PlanPurchases(controller, player); wants["battleship"] > 0 {
		t.Errorf("wants a battleship before the lift exists: %v", wants)
	}

	// Transports on hand: now exactly one bombardier is wanted.
	for len(plan.Ships) < plan.WantTransports {
		if err := g.PlacePieces("Home Sea", "transport", 1); err != nil {
			t.Fatalf("transport: %v", err)
		}
		id := g.Board["Home Sea"].Pieces[len(g.Board["Home Sea"].Pieces)-1]
		g.Pieces[id].Owner = player
		plan.Ships = append(plan.Ships, id)
	}
	wants := npc.PlanPurchases(controller, player)
	if wants["battleship"] != 1 {
		t.Errorf("wants %d battleships with lift on hand, want exactly 1", wants["battleship"])
	}

	// A battleship already escorting: no second gun.
	if err := g.PlacePieces("Home Sea", "battleship", 1); err != nil {
		t.Fatalf("battleship: %v", err)
	}
	bb := g.Board["Home Sea"].Pieces[len(g.Board["Home Sea"].Pieces)-1]
	g.Pieces[bb].Owner = player
	plan.Escorts = append(plan.Escorts, bb)
	if wants := npc.PlanPurchases(controller, player); wants["battleship"] != 0 {
		t.Errorf("wants %d more battleships with one already on escort", wants["battleship"])
	}
}

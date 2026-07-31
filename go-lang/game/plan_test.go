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

// A route is only useful if the convoy can actually take it, so enemy fleets
// close a passage rather than being sailed through.
func TestSeaRoute_AvoidsEnemyFleets(t *testing.T) {
	g, _ := invasionBoard(t)
	germany := g.Players["Germany"]

	if open := seaRouteFor(g, "Home Sea", "Island Sea", germany); len(open) == 0 {
		t.Fatal("expected an open route before any blockade")
	}

	// Park a UK destroyer in the only intervening sea zone.
	if err := g.PlacePieces("Mid Sea", "destroyer", 1); err != nil {
		t.Fatalf("placing blockade: %v", err)
	}
	for _, id := range g.Board["Mid Sea"].Pieces {
		g.Pieces[id].Owner = g.Players["UK"]
	}

	if blocked := seaRouteFor(g, "Home Sea", "Island Sea", germany); len(blocked) != 0 {
		t.Errorf("route %v runs through an enemy fleet", blocked)
	}
	// Ignoring ownership, the passage still exists.
	if ignoring := seaRoute(g, "Home Sea", "Island Sea"); len(ignoring) == 0 {
		t.Error("the geographic route should still be found when enemies are ignored")
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
	free := npc.uncommittedPieces(controller, player, "Home")
	for _, piece := range free {
		if piece.ID == committed {
			t.Error("a committed unit was offered to general movement")
		}
	}
}

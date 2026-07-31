package game

import (
	"testing"

	"boardgame/models"
)

// A factory and a victory city are the places a power cannot afford to lose,
// and are what a defence plan is opened for.
func TestReviewDefences_OpensPlansForWhatMatters(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]

	g.Board["Home"].IsVictoryCity = true
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)
	if err := g.PlacePieces("Inland", "factory", 1); err != nil {
		t.Fatalf("placing factory: %v", err)
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewDefences(controller, player, NewGameTranscript("t"))

	held := map[string]bool{}
	for _, plan := range controller.Plans.Defences("Germany") {
		held[plan.Territory] = true
		if plan.WantStrength <= 0 {
			t.Errorf("%s was given a garrison target of %d", plan.Territory, plan.WantStrength)
		}
	}
	if !held["Home"] {
		t.Error("no plan for the victory city")
	}
	if !held["Inland"] {
		t.Error("no plan for the territory with a factory")
	}
}

// A cautious power garrisons the same ground more heavily than an aggressive
// one. This is the posture dial reaching the actual decision.
func TestGarrisonTarget_FollowsPosture(t *testing.T) {
	g, _ := invasionBoard(t)
	territory := g.Board["Home"]
	territory.IsVictoryCity = true

	aggressive := garrisonTarget(g, territory, PostureFor("Germany"))
	defensive := garrisonTarget(g, territory, PostureFor("Italy"))

	if defensive <= aggressive {
		t.Errorf("defensive posture asked for %d, aggressive for %d; expected the "+
			"cautious power to want a deeper garrison", defensive, aggressive)
	}
}

// A threatening neighbour raises the bar.
func TestGarrisonTarget_RisesWithThreat(t *testing.T) {
	g, _ := invasionBoard(t)
	posture := PostureFor("UK")

	quiet := garrisonTarget(g, g.Board["Home"], posture)

	// Put an enemy army next door.
	if err := g.PlacePieces("Inland", "infantry", 4); err != nil {
		t.Fatalf("placing threat: %v", err)
	}
	models.ChangeOwnership(g.Board["Inland"], g.Players["UK"])
	for _, id := range g.Board["Inland"].Pieces {
		g.Pieces[id].Owner = g.Players["UK"]
	}

	threatened := garrisonTarget(g, g.Board["Home"], posture)
	if threatened <= quiet {
		t.Errorf("garrison target did not rise with a hostile neighbour: %d then %d",
			quiet, threatened)
	}
}

// Once a garrison is up to strength the plan stops asking for units, so the
// surplus can go to the front.
func TestDefencePlan_StopsBuyingOnceSatisfied(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]
	g.Board["Home"].IsVictoryCity = true

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewDefences(controller, player, NewGameTranscript("t"))

	var plan *DefencePlan
	for _, candidate := range controller.Plans.Defences("Germany") {
		if candidate.Territory == "Home" {
			plan = candidate
		}
	}
	if plan == nil {
		t.Fatal("no plan for Home")
	}

	if wanted := npc.DefencePurchases(controller, player); len(wanted) == 0 {
		t.Error("an empty garrison should be asking for units")
	}

	// Fill the garrison well past its target.
	if err := g.PlacePieces("Home", "infantry", plan.WantStrength+4); err != nil {
		t.Fatalf("placing garrison: %v", err)
	}
	npc.manGarrison(controller, player, plan)
	plan.Review(controller)

	if !plan.Satisfied {
		t.Fatalf("garrison of %d against target %d not satisfied",
			plan.GarrisonStrength(g), plan.WantStrength)
	}
	for unit, count := range npc.DefencePurchases(controller, player) {
		if unit != antiAircraftName(g) && count > 0 {
			t.Errorf("a satisfied garrison is still asking for %d %s", count, unit)
		}
	}
}

// Garrisoned units are reserved, or the general logic marches the defenders off
// to the front the turn after they arrive.
func TestDefencePlan_GarrisonIsReserved(t *testing.T) {
	g, controller := invasionBoard(t)
	player := g.Players["Germany"]
	g.Board["Home"].IsVictoryCity = true

	if err := g.PlacePieces("Home", "infantry", 3); err != nil {
		t.Fatalf("placing troops: %v", err)
	}
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewDefences(controller, player, NewGameTranscript("t"))

	var garrisoned int
	for _, plan := range controller.Plans.Defences("Germany") {
		if plan.Territory == "Home" && len(plan.Garrison) > 0 {
			garrisoned = plan.Garrison[0]
		}
	}
	if garrisoned == 0 {
		t.Fatal("nothing was assigned to the garrison")
	}
	if !controller.Plans.Committed("Germany", garrisoned) {
		t.Error("a garrisoned unit is not reserved against ordinary movement")
	}
}

// Anti-aircraft is wanted over factories and cities, not over empty ground.
func TestDefencePlan_AntiAircraftOnlyWhereItMatters(t *testing.T) {
	g, _ := invasionBoard(t)

	if worthAntiAircraft(g, g.Board["Inland"]) {
		t.Error("plain territory should not call for anti-aircraft")
	}
	g.Board["Inland"].IsVictoryCity = true
	if !worthAntiAircraft(g, g.Board["Inland"]) {
		t.Error("a victory city should call for anti-aircraft")
	}
}

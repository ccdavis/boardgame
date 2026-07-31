package game

import (
	"testing"

	"boardgame/models"
)

// finishedOperation sets up a completed landing with surviving warships.
func finishedOperation(t *testing.T) (*models.Game, *GameController, *AmphibiousPlan) {
	t.Helper()

	g, controller := invasionBoard(t)
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)
	g.AddPieceTemplate("sub", models.Water, 2, 2, 2, 8)

	plan := controller.Plans.Add(&AmphibiousPlan{
		Power:    "Germany",
		Target:   "Island",
		Staging:  "Home",
		Embark:   "Home Sea",
		DropZone: "Island Sea",
		State:    PlanForming,
	})
	return g, controller, plan
}

func idsIn(g *models.Game, territory string) []int {
	return append([]int{}, g.Board[territory].Pieces...)
}

// Warships that survive a landing get orders. Left alone they drift, because
// nothing in the general logic has an opinion about an idle fleet -- so a
// surviving escort was simply lost to the war effort.
func TestDisposeOfEscorts_GivesSurvivorsOrders(t *testing.T) {
	g, controller, plan := finishedOperation(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Island Sea", "battleship", 1); err != nil {
		t.Fatalf("placing escort: %v", err)
	}
	plan.Escorts = idsIn(g, "Island Sea")
	for _, id := range plan.Escorts {
		g.Pieces[id].Owner = player
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.DisposeOfEscorts(controller, player, plan, NewGameTranscript("t"))

	squadrons := controller.Plans.Naval("Germany")
	if len(squadrons) == 0 {
		t.Fatal("surviving warships were given no orders")
	}
	total := 0
	for _, squadron := range squadrons {
		total += len(squadron.Ships)
	}
	if total != len(plan.Escorts) {
		t.Errorf("%d ships survived but %d were given orders", len(plan.Escorts), total)
	}
}

// While the beachhead is still contested, ships that can help stay; the rest go
// home rather than loitering.
func TestDisposeOfEscorts_SplitsBetweenStationAndHome(t *testing.T) {
	g, controller, plan := finishedOperation(t)
	player := g.Players["Germany"]

	// A carrier with aircraft aboard is useful on station; a bare transport is
	// not, and should be sent home.
	if err := g.PlacePieces("Island Sea", "carrier", 1); err != nil {
		t.Fatalf("placing carrier: %v", err)
	}
	if err := g.PlacePieces("Island Sea", "transport", 1); err != nil {
		t.Fatalf("placing transport: %v", err)
	}
	ships := idsIn(g, "Island Sea")
	for _, id := range ships {
		g.Pieces[id].Owner = player
	}
	// Put aircraft aboard the carrier so it is worth keeping on station.
	for _, id := range ships {
		if g.Pieces[id].Name == "carrier" {
			g.Pieces[id].Holding = []int{999}
			plan.Escorts = append(plan.Escorts, id)
		} else {
			plan.Ships = append(plan.Ships, id)
		}
	}

	// The island is still in enemy hands, so there is something to cover.
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.DisposeOfEscorts(controller, player, plan, NewGameTranscript("t"))

	var patrols, returns int
	for _, squadron := range controller.Plans.Naval("Germany") {
		switch squadron.Mission {
		case NavalPatrol:
			patrols += len(squadron.Ships)
			if squadron.Supporting != "Island" {
				t.Errorf("patrol is covering %q, want Island", squadron.Supporting)
			}
		case NavalReturn:
			returns += len(squadron.Ships)
		}
	}
	if patrols == 0 {
		t.Error("the carrier should have stayed to cover the landing")
	}
	if returns == 0 {
		t.Error("the empty transport should have been sent home")
	}
}

// Once the target is taken there is nothing left to cover, so a patrol turns
// for home and becomes available again.
func TestNavalPlan_PatrolGoesHomeWhenTheJobIsDone(t *testing.T) {
	g, controller, plan := finishedOperation(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Island Sea", "battleship", 1); err != nil {
		t.Fatalf("placing escort: %v", err)
	}
	plan.Escorts = idsIn(g, "Island Sea")
	for _, id := range plan.Escorts {
		g.Pieces[id].Owner = player
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.DisposeOfEscorts(controller, player, plan, NewGameTranscript("t"))

	// Take the island, so the patrol has nothing left to support.
	models.ChangeOwnership(g.Board["Island"], player)
	npc.ReviewNaval(controller, player, NewGameTranscript("t"))

	for _, squadron := range controller.Plans.Naval("Germany") {
		if squadron.Mission == NavalPatrol {
			t.Error("a patrol is still covering a target that has already fallen")
		}
	}
}

// A squadron that loses every ship stops being a plan.
func TestNavalPlan_SunkSquadronIsForgotten(t *testing.T) {
	g, controller, plan := finishedOperation(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Island Sea", "battleship", 1); err != nil {
		t.Fatalf("placing escort: %v", err)
	}
	plan.Escorts = idsIn(g, "Island Sea")
	for _, id := range plan.Escorts {
		g.Pieces[id].Owner = player
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.DisposeOfEscorts(controller, player, plan, NewGameTranscript("t"))
	if len(controller.Plans.Naval("Germany")) == 0 {
		t.Fatal("no squadron was created")
	}

	for _, id := range plan.Escorts {
		delete(g.Pieces, id)
	}
	npc.ReviewNaval(controller, player, NewGameTranscript("t"))

	if len(controller.Plans.Naval("Germany")) != 0 {
		t.Error("a squadron with no ships left is still on the books")
	}
}

// A squadron under orders is reserved, so general movement does not sail it off
// somewhere else.
func TestNavalPlan_ShipsUnderOrdersAreReserved(t *testing.T) {
	g, controller, plan := finishedOperation(t)
	player := g.Players["Germany"]

	if err := g.PlacePieces("Island Sea", "battleship", 1); err != nil {
		t.Fatalf("placing escort: %v", err)
	}
	plan.Escorts = idsIn(g, "Island Sea")
	for _, id := range plan.Escorts {
		g.Pieces[id].Owner = player
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.DisposeOfEscorts(controller, player, plan, NewGameTranscript("t"))

	if !controller.Plans.Committed("Germany", plan.Escorts[0]) {
		t.Error("a ship under orders is not reserved against ordinary movement")
	}
}

// A returning squadron heads for water beside its own coast, where it can pick
// up the next landing force and covers home waters meanwhile.
func TestNearestFriendlyPort_FindsHomeWaters(t *testing.T) {
	g, _ := invasionBoard(t)
	player := g.Players["Germany"]

	port := nearestFriendlyPort(g, player, "Island Sea")
	if port != "Home Sea" {
		t.Errorf("nearest friendly port from Island Sea is %q, want Home Sea", port)
	}
}

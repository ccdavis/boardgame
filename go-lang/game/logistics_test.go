package game

import (
	"testing"

	"boardgame/models"
)

// A little continent: Capital -- Middle -- Front | Enemyland.
// The capital hoards; the front faces a larger enemy force.
func logisticsBoard(t *testing.T) (*models.Game, *GameController) {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "USSR"}
	germany := g.GetOrCreatePlayer("Germany")
	ussr := g.GetOrCreatePlayer("USSR")
	germany.Side, ussr.Side = "Axis", "Allies"
	germany.TakesTurns, ussr.TakesTurns = true, true

	g.AddTerritory("Capital", models.Land, "Germany", 8)
	g.AddTerritory("Middle", models.Land, "Germany", 2)
	g.AddTerritory("Front", models.Land, "Germany", 2)
	g.AddTerritory("Enemyland", models.Land, "USSR", 4)
	g.ConnectTerritories("Capital", "Middle")
	g.ConnectTerritories("Middle", "Front")
	g.ConnectTerritories("Front", "Enemyland")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("armor", models.Land, 2, 3, 2, 5)

	gc := NewGameController(g)
	if err := gc.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.NoncombatMovePhase
	return g, gc
}

// Surplus in the rear marches toward an outnumbered front instead of piling up
// at the factory. This is the fix for the frozen war: Germany kept 135 of 178
// units in Berlin because reinforcement only sourced from "safe" territories,
// a class its capital could never belong to.
func TestDisperse_SurplusMarchesTowardTheFront(t *testing.T) {
	g, gc := logisticsBoard(t)
	germany := g.Players["Germany"]

	g.PlacePieces("Capital", "infantry", 10)
	g.PlacePieces("Front", "infantry", 1)
	g.PlacePieces("Enemyland", "armor", 4) // attack 12 vs our defence 2

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	moved := npc.DisperseToFronts(gc, germany, NewGameTranscript("t"))
	if moved == 0 {
		t.Fatal("no surplus marched despite an outnumbered front")
	}
	if err := gc.ExecuteNoncombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	left := 0
	for _, id := range g.Board["Capital"].Pieces {
		if g.Pieces[id].Owner == germany {
			left++
		}
	}
	if left == 10 {
		t.Error("the capital still hoards all ten units")
	}
	if n := len(g.Board["Middle"].Pieces); n == 0 {
		t.Error("nobody is on the road to the front")
	}
}

// A front with more strength than the enemy next door exports only its excess:
// equalize at least, then send the rest onward.
func TestDisperse_FrontKeepsEnoughToStayEqual(t *testing.T) {
	g, gc := logisticsBoard(t)
	germany := g.Players["Germany"]

	// The front holds 8 infantry (defence 16) against enemy attack 6. A second
	// outnumbered front needs help.
	g.AddTerritory("Second Front", models.Land, "Germany", 2)
	g.AddTerritory("Enemyland Two", models.Land, "USSR", 3)
	g.ConnectTerritories("Middle", "Second Front")
	g.ConnectTerritories("Second Front", "Enemyland Two")

	g.PlacePieces("Front", "infantry", 8)
	g.PlacePieces("Enemyland", "armor", 2)     // attack 6 at Front
	g.PlacePieces("Enemyland Two", "armor", 4) // attack 12 at Second Front

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.DisperseToFronts(gc, germany, NewGameTranscript("t"))
	if err := gc.ExecuteNoncombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	kept := defenceStrength(g, g.Board["Front"], germany)
	if kept < 6 {
		t.Errorf("Front kept defence %d against enemy attack 6; it exported below equality", kept)
	}
	if kept >= 16 {
		t.Error("Front exported nothing despite holding well above equality")
	}
}

// Units a defence plan has claimed stay put whatever the fronts need: the
// garrison is the floor the quartermaster must not touch.
func TestDisperse_GarrisonsAreNotExported(t *testing.T) {
	g, gc := logisticsBoard(t)
	germany := g.Players["Germany"]

	g.PlacePieces("Capital", "infantry", 6)
	g.PlacePieces("Front", "infantry", 1)
	g.PlacePieces("Enemyland", "armor", 5)

	// A defence plan claims four of the capital's six.
	plan := gc.Plans.AddDefence(&DefencePlan{
		Power: "Germany", Territory: "Capital", WantStrength: 8,
	})
	for _, id := range g.Board["Capital"].Pieces[:4] {
		plan.Garrison = append(plan.Garrison, id)
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.DisperseToFronts(gc, germany, NewGameTranscript("t"))
	if err := gc.ExecuteNoncombatMoves(); err != nil {
		t.Fatalf("executing: %v", err)
	}

	remaining := make(map[int]bool)
	for _, id := range g.Board["Capital"].Pieces {
		remaining[id] = true
	}
	for _, id := range plan.Garrison {
		if !remaining[id] {
			t.Errorf("garrison piece %d was marched away from its post", id)
		}
	}
}

// Surplus stranded on an island with no front is ferried toward a coast that
// can reach one.
func TestFerry_ShuttlesStrandedSurplus(t *testing.T) {
	g, gc := logisticsBoard(t)
	germany := g.Players["Germany"]

	// An island with troops and no enemies, a sea lane, and an idle transport.
	g.AddTerritory("Island", models.Land, "Germany", 3)
	g.AddTerritory("Island Sea", models.Water, "Neutral", 0)
	g.AddTerritory("Coast Sea", models.Water, "Neutral", 0)
	g.ConnectTerritories("Island", "Island Sea")
	g.ConnectTerritories("Island Sea", "Coast Sea")
	g.ConnectTerritories("Coast Sea", "Capital")

	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	g.GlobalPieceTemplates["transport"].Capacity = 2
	g.GlobalPieceTemplates["transport"].CanCarry = []string{"infantry", "armor"}

	g.PlacePieces("Island", "infantry", 4)
	g.PlacePieces("Island Sea", "transport", 1)
	transportID := g.Board["Island Sea"].Pieces[0]
	g.Pieces[transportID].Owner = germany

	g.PlacePieces("Front", "infantry", 1)
	g.PlacePieces("Enemyland", "armor", 3)

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	moved := npc.FerrySurplus(gc, germany, NewGameTranscript("t"))
	if moved == 0 {
		t.Fatal("the ferry did nothing with stranded surplus and an idle transport")
	}
	if len(g.Pieces[transportID].Holding) == 0 {
		t.Error("the transport loaded nothing from the island")
	}
}

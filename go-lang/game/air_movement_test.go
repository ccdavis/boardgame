package game

import (
	"testing"

	"boardgame/models"
)

// airBoard is a strip for exercising aircraft movement:
//
//	Base (Germany) -- Occupied (UK, garrisoned) -- Far (Germany)
//	Base -- Coast Sea (empty water)
func airBoard(t *testing.T) (*models.Game, *GameController) {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "UK"}
	for _, name := range g.PlayerOrder {
		p := g.GetOrCreatePlayer(name)
		p.TakesTurns = true
	}
	g.Players["Germany"].Side = "Axis"
	g.Players["UK"].Side = "Allies"

	g.AddTerritory("Base", models.Land, "Germany", 3)
	g.AddTerritory("Occupied", models.Land, "UK", 2)
	g.AddTerritory("Far", models.Land, "Germany", 2)
	g.AddTerritory("Coast Sea", models.Water, "Neutral", 0)

	g.ConnectTerritories("Base", "Occupied")
	g.ConnectTerritories("Occupied", "Far")
	g.ConnectTerritories("Base", "Coast Sea")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 12)
	g.AddPieceTemplate("carrier", models.Water, 2, 0, 1, 24)
	g.GlobalPieceTemplates["carrier"].Capacity = 2
	g.GlobalPieceTemplates["carrier"].CanCarry = []string{"fighter"}

	g.PlacePieces("Base", "fighter", 1)
	g.PlacePieces("Occupied", "infantry", 2)

	gc := NewGameController(g)
	if err := gc.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	return g, gc
}

func fighterAt(t *testing.T, g *models.Game, territory string) int {
	t.Helper()
	for _, id := range g.Board[territory].Pieces {
		if g.Pieces[id].Name == "fighter" {
			return id
		}
	}
	t.Fatalf("no fighter in %s", territory)
	return -1
}

// Aircraft overfly enemy armies. The ground waypoint rules -- enemy territory
// and enemy units block the path -- grounded every fighter: it could not reach
// any friendly territory past a hostile one.
func TestAir_FliesOverEnemyHeldGround(t *testing.T) {
	g, gc := airBoard(t)
	g.CurrentPhase = models.NoncombatMovePhase
	id := fighterAt(t, g, "Base")

	if err := gc.PlanMove(id, "Base", "Far"); err != nil {
		t.Fatalf("fighter cannot overfly an enemy garrison: %v", err)
	}
}

// A fighter cannot end a noncombat move on open water.
//
// Nothing used to check this: air could "move anywhere", so a fighter would
// settle in a sea zone and live there for the rest of the game.
func TestAir_CannotLandOnOpenWater(t *testing.T) {
	g, gc := airBoard(t)
	g.CurrentPhase = models.NoncombatMovePhase
	id := fighterAt(t, g, "Base")

	if err := gc.PlanMove(id, "Base", "Coast Sea"); err == nil {
		t.Fatal("a fighter was allowed to end its move on open water with no carrier")
	}
}

// A friendly carrier with room turns that same sea zone into a landing spot.
func TestAir_LandsOnAFriendlyCarrierWithRoom(t *testing.T) {
	g, gc := airBoard(t)
	g.CurrentPhase = models.NoncombatMovePhase
	id := fighterAt(t, g, "Base")

	g.PlacePieces("Coast Sea", "carrier", 1)
	// Sea zones are owned by nobody in practice; the carrier's flag is what
	// matters. PlacePieces assigns the territory owner, so set it explicitly.
	for _, pid := range g.Board["Coast Sea"].Pieces {
		if g.Pieces[pid].Name == "carrier" {
			g.Pieces[pid].Owner = g.Players["Germany"]
		}
	}

	if err := gc.PlanMove(id, "Base", "Coast Sea"); err != nil {
		t.Fatalf("fighter refused a friendly carrier with room: %v", err)
	}
}

// A full carrier is no landing spot.
func TestAir_RefusesAFullCarrier(t *testing.T) {
	g, gc := airBoard(t)
	g.CurrentPhase = models.NoncombatMovePhase
	id := fighterAt(t, g, "Base")

	g.PlacePieces("Coast Sea", "carrier", 1)
	var carrierID int
	for _, pid := range g.Board["Coast Sea"].Pieces {
		if g.Pieces[pid].Name == "carrier" {
			carrierID = pid
			g.Pieces[pid].Owner = g.Players["Germany"]
		}
	}
	// Fill both slots.
	g.Pieces[carrierID].Holding = []int{9001, 9002}

	if err := gc.PlanMove(id, "Base", "Coast Sea"); err == nil {
		t.Fatal("a fighter was allowed to land on a carrier with no room")
	}
}

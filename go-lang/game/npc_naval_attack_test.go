package game

import (
	"testing"

	"boardgame/models"
)

// A fleet in open ocean must find and attack the enemy fleet next door. Three
// separate faults used to prevent this ever happening: target discovery and
// attacker sourcing both looked only at owned territory (fleets own nothing),
// and canAttackNeutral refused every Neutral-flagged sea zone outright.
func TestNPCAttacksEnemyFleetInOpenOcean(t *testing.T) {
	g := models.NewGame()

	g.AddTerritory("Home Port", models.Land, "Germany", 4)
	g.AddTerritory("Near Sea", models.Water, "Neutral", 0)
	// Far Sea carries a UK starting marker, which also brings the UK player
	// into existence -- players are created by territory ownership.
	g.AddTerritory("Far Sea", models.Water, "UK", 0)
	g.ConnectTerritories("Home Port", "Near Sea")
	g.ConnectTerritories("Near Sea", "Home Port")
	g.ConnectTerritories("Near Sea", "Far Sea")
	g.ConnectTerritories("Far Sea", "Near Sea")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("destroyer", models.Water, 2, 2, 2, 8)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 0, 8)

	// Germany: a strong squadron in open water plus a home garrison. UK: a
	// defenceless transport one zone over -- odds any doctrine accepts.
	g.PlacePieces("Home Port", "infantry", 3)
	g.PlacePieces("Near Sea", "destroyer", 3)
	g.PlacePieces("Far Sea", "transport", 1)

	germany := g.Players["Germany"]
	uk := g.Players["UK"]
	for _, id := range g.Board["Near Sea"].Pieces {
		g.Pieces[id].Owner = germany
	}
	for _, id := range g.Board["Far Sea"].Pieces {
		g.Pieces[id].Owner = uk
	}

	g.PlayerOrder = []string{"Germany", "UK"}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	germany.Side = "Axis"
	uk.Side = "Allies"

	gc := NewGameController(g)
	npc := NewNPCAIPlayer("Germany", DefaultDifficulty)
	transcript := NewGameTranscript("naval test")

	if err := npc.CombatMovePhase(gc, transcript); err != nil {
		t.Fatalf("NPC combat move phase: %v", err)
	}

	if _, ok := gc.PendingBattles["Far Sea"]; !ok {
		for _, entry := range transcript.Entries {
			t.Logf("transcript: %s", entry.Action)
		}
		t.Fatal("no battle in Far Sea: the NPC never attacked the enemy fleet")
	}
}

package webserver

import (
	"net/http"
	"testing"

	"boardgame/game"
	"boardgame/models"
)

func TestGameplayAPI_ExcessPurchasesAreRefundedAfterDeployment(t *testing.T) {
	g := createIntegrationTestGame()
	g.Board["Berlin"].Production = 1
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 15)
	if err := g.PlacePieces("Berlin", "factory", 1); err != nil {
		t.Fatal(err)
	}
	controller := game.NewGameController(g)
	if err := controller.StartGame(); err != nil {
		t.Fatal(err)
	}
	server := NewServer(0)
	session, err := server.sessionManager.CreateSession(controller, "Germany")
	if err != nil {
		t.Fatal(err)
	}
	client := &simClient{t: t, server: server, sessionID: session.ID}
	if code, result := client.post("purchase", map[string]any{"unitType": "infantry", "quantity": -10}); code != http.StatusBadRequest {
		t.Fatalf("negative purchase returned %d: %v", code, result)
	}
	if g.Players["Germany"].IPCs != 40 {
		t.Fatal("negative purchase changed treasury")
	}
	if code, result := client.post("purchase", map[string]any{"unitType": "infantry", "quantity": 2}); code != http.StatusOK {
		t.Fatalf("purchase returned %d: %v", code, result)
	}
	g.CurrentPhase = models.MobilizePhase
	if code, _ := client.post("advance-phase", nil); code != http.StatusConflict {
		t.Fatal("allowed skipping units that can still be placed")
	}
	if code, result := client.post("mobilize", map[string]any{"unitType": "infantry", "quantity": 1, "territory": "Berlin"}); code != http.StatusOK {
		t.Fatalf("deployment returned %d: %v", code, result)
	}
	if code, result := client.post("advance-phase", nil); code != http.StatusOK {
		t.Fatalf("excess purchase trapped turn: %d %v", code, result)
	}
	if g.CurrentPhase != models.CollectIncomePhase || g.Players["Germany"].IPCs != 37 || len(g.PurchasedUnits["Germany"]) != 0 {
		t.Fatal("phase did not advance with the exact excess purchase refunded")
	}
}

func TestGameplayAPI_UnloadedTransportIsNotOfferedForBoarding(t *testing.T) {
	session := powerSession(t, "UK", models.NoncombatMovePhase)
	ship, cargo := loadedTransport(t, session, "North Sea", "Britain", 2)
	for _, id := range cargo {
		if err := session.Controller.UnloadUnit(ship, id, "Britain"); err != nil {
			t.Fatal(err)
		}
	}
	g := session.Controller.Game
	// The real board starts with two transports here; leave only the one
	// whose unloading is under test, so the other cannot offer a valid trip.
	zone := g.Board["North Sea"]
	var remaining []int
	for _, id := range zone.Pieces {
		if id != ship && g.Pieces[id].Name == "transport" {
			delete(g.Pieces, id)
			continue
		}
		remaining = append(remaining, id)
	}
	zone.Pieces = remaining
	if err := g.PlacePieces("Britain", "infantry", 1); err != nil {
		t.Fatal(err)
	}
	dests, _ := transportOptions(session, []int{g.NextPieceID - 1}, "Britain")
	for _, dest := range dests {
		if dest.Name == "North Sea" && dest.IsBoard {
			t.Fatal("offered an unloaded transport for another trip")
		}
	}
	reachable, err := reachableForPiece(session, ship, "North Sea")
	if err != nil {
		t.Fatal(err)
	}
	if len(reachable) != 0 {
		t.Fatal("unloaded transport was offered another sailing destination")
	}
}

func TestGameplayAPI_BattleRosterExcludesRaidingBombers(t *testing.T) {
	g := createIntegrationTestGame()
	g.AddPieceTemplate("bomber", models.Air, 6, 4, 1, 15)
	if err := g.PlacePieces("Moscow", "infantry", 1); err != nil {
		t.Fatal(err)
	}
	if err := g.PlacePieces("Moscow", "bomber", 1); err != nil {
		t.Fatal(err)
	}
	bomber := g.NextPieceID - 1
	g.Pieces[bomber].Owner = g.Players["Germany"]
	battle := game.NewBattle("Moscow", game.LandBattle, "Germany", "USSR")
	dto := ToPendingBattleDTO(g, battle)
	if len(dto.Defenders) != 1 || dto.Defenders[0].Type != "infantry" || len(dto.Attackers) != 0 {
		t.Fatalf("raider appeared in a ground-combat roster: %+v", dto)
	}
}

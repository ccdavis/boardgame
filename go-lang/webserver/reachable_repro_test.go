package webserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"boardgame/models"
)

// newGameForTestAs creates a session for a chosen human power.
func newGameForTestAs(t *testing.T, server *Server, power string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"gdfPath":    "../../aaa.gdf",
		"playerName": power,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/game/new", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	server.handleCreateGame(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("creating game: status %d, body %s", rec.Code, rec.Body.String())
	}

	var response struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	return response.SessionID
}

// Reproduction of a reported game: Italy holds Egypt with two units, the UK
// holds Kenya and Congo next door with units, and the player could not open
// the movement dialog / attack either neighbour. The dialog opens only when
// the territory DTO reports friendly units, and the destinations come from
// get-reachable — so both are checked here against the real board.
func TestReachable_EgyptCanAttackKenyaAndCongo(t *testing.T) {
	server := NewServer(0)

	sessionID := newGameForTestAs(t, server, "Italy")
	session, err := server.sessionManager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("getting session: %v", err)
	}
	g := session.Controller.Game

	italy := g.Players["Italy"]

	// Hand Egypt to Italy: clear the UK garrison, then garrison it with Italy.
	egypt := g.Board["Egypt"]
	for _, id := range egypt.Pieces {
		delete(g.Pieces, id)
	}
	egypt.Pieces = nil
	models.ChangeOwnership(egypt, italy)
	if err := g.PlacePieces("Egypt", "infantry", 1); err != nil {
		t.Fatalf("placing infantry: %v", err)
	}
	if err := g.PlacePieces("Egypt", "armor", 1); err != nil {
		t.Fatalf("placing armor: %v", err)
	}

	// The UK moves units into Kenya and Congo, as happened in the game.
	if err := g.PlacePieces("Kenya", "infantry", 2); err != nil {
		t.Fatalf("placing UK infantry: %v", err)
	}
	if err := g.PlacePieces("Congo", "infantry", 1); err != nil {
		t.Fatalf("placing UK infantry: %v", err)
	}

	g.CurrentPower = "Italy"
	g.CurrentPhase = models.CombatMovePhase

	// The map's eligibility glow needs friendly units reported for Egypt.
	dto := ToTerritoryDTO(egypt, g, "Italy")
	if dto.FriendlyUnits != 2 {
		t.Errorf("Egypt friendlyUnits = %d, want 2", dto.FriendlyUnits)
	}

	// Each unit individually, and the pair together, must be offered Kenya
	// and Congo as attack destinations.
	for _, id := range egypt.Pieces {
		reachable, err := reachableForPiece(session, id, "Egypt")
		if err != nil {
			t.Fatalf("reachable for piece %d (%s): %v", id, g.Pieces[id].Name, err)
		}
		for _, want := range []string{"Kenya", "Congo"} {
			dest, ok := reachable[want]
			if !ok {
				t.Errorf("%s in Egypt is not offered %s", g.Pieces[id].Name, want)
				continue
			}
			if !dest.IsAttack {
				t.Errorf("%s -> %s not flagged as an attack", g.Pieces[id].Name, want)
			}
		}
	}
}

// Reproduction of the "stranded troops" report. An American infantry starts
// in Alaska, whose only land neighbour is Western Canada -- British ground.
// The map lit Alaska up in the combat phase, the picker opened, and the
// destination request came back empty: the pathfinder refused every
// allied-owned destination for a combat move. Allied ground must be offered
// exactly as our own is, and never as an attack.
func TestReachable_AlliedNeighbourIsOfferedInCombatMove(t *testing.T) {
	server := NewServer(0)
	sessionID := newGameForTestAs(t, server, "USA")
	session, err := server.sessionManager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("getting session: %v", err)
	}
	g := session.Controller.Game
	g.CurrentPower = "USA"
	g.CurrentPhase = models.CombatMovePhase

	alaska := g.Board["Alaska"]
	if len(alaska.Pieces) == 0 {
		t.Fatal("the board should start an American unit in Alaska")
	}
	infantry := alaska.Pieces[0]

	reachable, err := reachableForPiece(session, infantry, "Alaska")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	dest, ok := reachable["Western Canada"]
	if !ok {
		t.Fatalf("Western Canada (allied) is not offered from Alaska in the combat phase; got %v", reachable)
	}
	if dest.IsAttack {
		t.Error("a move onto an ally's ground is flagged as an attack")
	}

	// And the engine agrees: the move books, executes, and neither fights
	// nor captures anything.
	if err := session.Controller.PlanMove(infantry, "Alaska", "Western Canada"); err != nil {
		t.Fatalf("PlanMove into allied Western Canada: %v", err)
	}
	if attacks := session.Controller.GetPlannedAttacks(); len(attacks) != 0 {
		t.Errorf("planned attacks should be empty, got %v", attacks)
	}
	if err := session.Controller.ExecuteCombatMoves(); err != nil {
		t.Fatalf("ExecuteCombatMoves: %v", err)
	}
	if len(session.Controller.PendingBattles) != 0 {
		t.Errorf("a battle was staged against an ally: %v", session.Controller.PendingBattles)
	}
	if owner := g.Board["Western Canada"].Owner.Name; owner != "UK" {
		t.Errorf("Western Canada changed hands to %s", owner)
	}
	if g.Pieces[infantry].Owner.Name != "USA" {
		t.Errorf("the infantry changed allegiance to %s", g.Pieces[infantry].Owner.Name)
	}
}

// Sea zones carry a nominal owner in the board file. Judging a combat move by
// it barred a fleet from attacking into any zone nominally an ally's -- which
// for the UK and USA is most of the ocean. What makes a sea zone a fight is
// the enemy fleet in it.
func TestReachable_FleetMayAttackIntoAlliedNominalSeaZone(t *testing.T) {
	server := NewServer(0)
	sessionID := newGameForTestAs(t, server, "UK")
	session, err := server.sessionManager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("getting session: %v", err)
	}
	g := session.Controller.Game
	g.CurrentPower = "UK"
	g.CurrentPhase = models.CombatMovePhase

	// Alaskan Pacific is nominally American; put a Japanese submarine in it
	// and a British battleship next door in the Canadian Pacific.
	target := g.Board["Alaskan Pacific"]
	if target.Owner.Name != "USA" {
		t.Fatalf("test assumes Alaskan Pacific is nominally USA, got %s", target.Owner.Name)
	}
	if err := g.PlacePieces("Alaskan Pacific", "sub", 1); err != nil {
		t.Fatal(err)
	}
	g.Pieces[g.NextPieceID-1].Owner = g.Players["Japan"]
	if err := g.PlacePieces("Canadian Pacific", "battleship", 1); err != nil {
		t.Fatal(err)
	}
	battleship := g.NextPieceID - 1
	g.Pieces[battleship].Owner = g.Players["UK"]

	reachable, err := reachableForPiece(session, battleship, "Canadian Pacific")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	dest, ok := reachable["Alaskan Pacific"]
	if !ok {
		t.Fatalf("the battleship is not offered the enemy-held Alaskan Pacific; got %v", reachable)
	}
	if !dest.IsAttack {
		t.Error("sailing into an enemy submarine is not flagged as an attack")
	}
	if err := session.Controller.PlanMove(battleship, "Canadian Pacific", "Alaskan Pacific"); err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if err := session.Controller.ExecuteCombatMoves(); err != nil {
		t.Fatalf("ExecuteCombatMoves: %v", err)
	}
	if _, fight := session.Controller.PendingBattles["Alaskan Pacific"]; !fight {
		t.Error("no sea battle was staged against the submarine")
	}
}

// The glow must mean "something here can move". A territory holding only an
// AA gun (0 movement), or a stack whose every unit has already been given
// orders, must not light up and invite a click that opens an empty picker.
func TestTerritories_GlowOnlyWhereSomethingCanMove(t *testing.T) {
	server := NewServer(0)
	sessionID := newGameForTestAs(t, server, "Germany")
	session, err := server.sessionManager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("getting session: %v", err)
	}
	g := session.Controller.Game
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.NoncombatMovePhase

	// Norway: strip it to one AA gun.
	norway := g.Board["Norway"]
	for _, id := range norway.Pieces {
		delete(g.Pieces, id)
	}
	norway.Pieces = nil
	if err := g.PlacePieces("Norway", "AAA", 1); err != nil {
		t.Fatal(err)
	}
	// An AA gun is towed in the noncombat phase only.
	if n := movableUnits(session, norway); n != 1 {
		t.Errorf("Norway with only an AA gun reports %d movable units in noncombat, want 1", n)
	}
	g.CurrentPhase = models.CombatMovePhase
	if n := movableUnits(session, norway); n != 0 {
		t.Errorf("Norway with only an AA gun reports %d movable units in the combat phase, want 0", n)
	}
	g.CurrentPhase = models.NoncombatMovePhase

	// Finland: one infantry, movable until it is given orders.
	finland := g.Board["Finland"]
	for _, id := range finland.Pieces {
		delete(g.Pieces, id)
	}
	finland.Pieces = nil
	if err := g.PlacePieces("Finland", "infantry", 1); err != nil {
		t.Fatal(err)
	}
	infantry := finland.Pieces[0]
	if n := movableUnits(session, finland); n != 1 {
		t.Fatalf("Finland with one fresh infantry reports %d movable units", n)
	}
	if err := session.Controller.PlanMove(infantry, "Finland", "Norway"); err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if n := movableUnits(session, finland); n != 0 {
		t.Errorf("Finland reports %d movable units after its only unit was given orders", n)
	}
	if err := session.Controller.CancelMove(infantry); err != nil {
		t.Fatal(err)
	}
	// A unit that spent its whole allowance this turn is done too.
	session.Controller.MoveTracker.Spend(infantry, 1)
	if n := movableUnits(session, finland); n != 0 {
		t.Errorf("Finland reports %d movable units though its infantry has no movement left", n)
	}
}

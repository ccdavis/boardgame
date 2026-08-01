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

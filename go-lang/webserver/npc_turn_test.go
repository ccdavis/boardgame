package webserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The browser's whole game loop: the human plays a turn, then asks the server
// to run each NPC turn until play comes back around.
//
// The action gate used to reject execute-npc-turn with "Not your turn" -- which
// is the one action that only makes sense when it is NOT the human's turn -- so
// a web game deadlocked permanently after the human's first turn.
func TestNPCTurnRunnableFromBrowser(t *testing.T) {
	server := NewServer(0)
	sessionID := newGameForTest(t, server) // human plays Germany

	post := func(action string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost,
			"/api/game/"+sessionID+"/action/"+action, bytes.NewReader([]byte("{}")))
		rec := httptest.NewRecorder()
		server.handleGameRoutes(rec, req)
		return rec
	}

	// The human walks through all six phases of their turn.
	for i := 0; i < 6; i++ {
		rec := post("advance-phase")
		if rec.Code != http.StatusOK {
			t.Fatalf("advance %d: status %d, body %s", i, rec.Code, rec.Body.String())
		}
	}

	session, err := server.sessionManager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("getting session: %v", err)
	}
	if session.Controller.Game.CurrentPower == "Germany" {
		t.Fatal("turn did not pass to an NPC power")
	}

	// Now each NPC turn must be runnable from the browser until the human is up
	// again. One iteration per playing power is more than enough.
	for i := 0; i < 8; i++ {
		if session.Controller.Game.CurrentPower == "Germany" {
			return // play came back around: the loop works
		}
		rec := post("execute-npc-turn")
		if rec.Code != http.StatusOK {
			var body map[string]any
			json.Unmarshal(rec.Body.Bytes(), &body)
			t.Fatalf("execute-npc-turn for %s: status %d, body %v",
				session.Controller.Game.CurrentPower, rec.Code, body)
		}
	}
	t.Fatalf("play never returned to the human; stuck at %s",
		session.Controller.Game.CurrentPower)
}

// Actions other than execute-npc-turn stay gated on it being the human's turn.
func TestHumanActionsRejectedDuringNPCTurn(t *testing.T) {
	server := NewServer(0)
	sessionID := newGameForTest(t, server)

	post := func(action string, payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost,
			"/api/game/"+sessionID+"/action/"+action, bytes.NewReader([]byte(payload)))
		rec := httptest.NewRecorder()
		server.handleGameRoutes(rec, req)
		return rec
	}

	for i := 0; i < 6; i++ {
		if rec := post("advance-phase", "{}"); rec.Code != http.StatusOK {
			t.Fatalf("advance %d: status %d, body %s", i, rec.Code, rec.Body.String())
		}
	}

	rec := post("purchase", `{"unitType":"infantry","quantity":1}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("purchase during an NPC turn: status %d, want 400", rec.Code)
	}
}

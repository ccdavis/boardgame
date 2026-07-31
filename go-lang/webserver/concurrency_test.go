package webserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// newGameForTest creates a session through the real handler and returns its ID.
func newGameForTest(t *testing.T, server *Server) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"gdfPath":    "../../aaa.gdf",
		"playerName": "Germany",
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

// TestConcurrentRequestsOnOneGame is the test that makes the session locking
// worth having. Run it with -race.
//
// Every handler reaches through the session into one shared GameController and
// mutates it. The client polls game state on a timer while the player clicks, so
// two requests hitting the same game at once is the normal case. Before the
// session was locked for the duration of a request, the mutex guarded only the
// last-accessed timestamp and this raced the entire object graph.
func TestConcurrentRequestsOnOneGame(t *testing.T) {
	server := NewServer(0)
	sessionID := newGameForTest(t, server)

	// Give the human power money to spend, otherwise every purchase is refused
	// for want of IPCs and the test never performs a write -- reads alone cannot
	// race, so it would pass whether or not the session is locked.
	session, err := server.sessionManager.GetSession(sessionID)
	if err != nil {
		t.Fatalf("getting session: %v", err)
	}
	session.Controller.Game.Players["Germany"].IPCs = 100000

	const workers = 32
	const iterations = 60

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				var req *http.Request
				switch worker % 4 {
				case 0:
					req = httptest.NewRequest(http.MethodGet, "/api/game/"+sessionID, nil)
				case 1:
					req = httptest.NewRequest(http.MethodGet, "/api/game/"+sessionID+"/territories", nil)
				case 2:
					req = httptest.NewRequest(http.MethodGet, "/api/game/"+sessionID+"/available-actions", nil)
				default:
					body, _ := json.Marshal(map[string]interface{}{
						"unitType": "infantry", "quantity": 1,
					})
					req = httptest.NewRequest(http.MethodPost,
						"/api/game/"+sessionID+"/action/purchase", bytes.NewReader(body))
				}

				rec := httptest.NewRecorder()
				server.handleGameRoutes(rec, req)

				// Any well-formed answer is fine, including a refusal. What must
				// not happen is a data race or a panic.
				if rec.Code >= 500 {
					t.Errorf("worker %d: server error %d: %s", worker, rec.Code, rec.Body.String())
					return
				}
			}
		}(worker)
	}
	wg.Wait()
}

// The layout is fetched once per game but the state is polled continuously, so
// these two run against each other constantly in practice.
func TestConcurrentLayoutAndStateRequests(t *testing.T) {
	server := NewServer(0)
	sessionID := newGameForTest(t, server)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			path := "/api/game/" + sessionID + "/layout"
			if i%2 == 0 {
				path = "/api/game/" + sessionID
			}
			rec := httptest.NewRecorder()
			server.handleGameRoutes(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Errorf("GET %s: status %d", path, rec.Code)
			}
		}(i)
	}
	wg.Wait()
}

package webserver

import (
	"net/http"

	"boardgame/layout"
	"boardgame/models"
)

// loadLayoutFor loads and validates the map geometry paired with a .gdf.
//
// The check runs at game creation rather than in the browser, for two reasons:
// the .gdf path is chosen at runtime, so the client cannot know which layout
// belongs to it; and the layout sits next to the .gdf, outside the static
// directory the file server can reach. Validating here means a board can never
// render against coordinates belonging to a different board -- the failure that
// made the previous hand-authored coordinate table useless.
//
// Only the key sets are checked. Full geometry validation is expensive and
// belongs in `layoutcheck` and the layout package's tests, not on a request.
func loadLayoutFor(gdfPath string, game *models.Game) (*layout.Layout, []byte, error) {
	l, raw, err := layout.LoadFor(gdfPath)
	if err != nil {
		return nil, nil, err
	}
	if err := l.ValidateKeys(game); err != nil {
		return nil, nil, err
	}
	return l, raw, nil
}

// handleLayout serves GET /api/game/:sessionId/layout.
//
// The stored bytes are echoed verbatim: the geometry is static for the life of
// a game, and re-marshalling it on every request would be pure waste. Geometry
// is deliberately kept out of the game-state response, which the client polls.
func (s *Server) handleLayout(w http.ResponseWriter, r *http.Request, session *GameSession) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(session.LayoutRaw) == 0 {
		s.sendError(w, "No map layout is loaded for this game", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write(session.LayoutRaw)
}

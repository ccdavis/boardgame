package webserver

import (
	"boardgame/game"
	"boardgame/models"
	"boardgame/parser"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

// Server represents the web server
type Server struct {
	sessionManager *SessionManager
	port           int
	mux            *http.ServeMux
}

// NewServer creates a new web server
func NewServer(port int) *Server {
	return &Server{
		sessionManager: NewSessionManager(),
		port:           port,
		mux:            http.NewServeMux(),
	}
}

// Start starts the web server
func (s *Server) Start() error {
	// Setup routes on our own ServeMux (not the global one)
	s.mux.HandleFunc("/api/game/new", s.corsMiddleware(s.handleCreateGame))
	s.mux.HandleFunc("/api/game/", s.corsMiddleware(s.handleGameRoutes))

	// Serve static files for frontend
	// Check multiple possible locations for static files
	staticDir := "./webserver/static"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// Try from webserver package directory (during tests)
		staticDir = "./static"
		if _, err := os.Stat(staticDir); os.IsNotExist(err) {
			log.Printf("Warning: static directory not found, static files will not be served")
		}
	}
	s.mux.Handle("/", http.FileServer(http.Dir(staticDir)))

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting web server on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// handleCreateGame handles POST /api/game/new
func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GDFPath    string `json:"gdfPath"`
		PlayerName string `json:"playerName"`

		// VictoryCities toggles the victory-city win condition for this game.
		// The cities are always in the board data; whether holding them ends
		// the game is chosen at play time. Absent means "as the board says".
		VictoryCities *bool `json:"victoryCities,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Parse the game file
	p, err := parser.NewParser(req.GDFPath)
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to load game file: %v", err), http.StatusBadRequest)
		return
	}

	gameModel, err := p.Parse()
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to parse game: %v", err), http.StatusBadRequest)
		return
	}

	if req.VictoryCities != nil {
		gameModel.VictoryCitiesEnabled = *req.VictoryCities
	}

	// Create game controller
	controller := game.NewGameController(gameModel)

	// Mark selected player as human, others as NPC
	humanPlayer := req.PlayerName
	found := false
	for _, playerName := range gameModel.PlayerOrder {
		player := gameModel.Players[playerName]
		if playerName == humanPlayer {
			player.NPC = false
			found = true
		} else {
			player.NPC = true
		}
	}

	if !found {
		s.sendError(w, fmt.Sprintf("Player %s not found in game", humanPlayer), http.StatusBadRequest)
		return
	}

	// Start the game
	if err := controller.StartGame(); err != nil {
		s.sendError(w, fmt.Sprintf("Failed to start game: %v", err), http.StatusInternalServerError)
		return
	}

	// Load the map geometry that pairs with this board and check it describes
	// the same territories. Failing here, loudly, is the point: a board rendered
	// against another board's coordinates is exactly the bug this replaced.
	mapLayout, layoutRaw, err := loadLayoutFor(req.GDFPath, gameModel)
	if err != nil {
		s.sendError(w, fmt.Sprintf("Map layout problem: %v", err), http.StatusBadRequest)
		return
	}

	// Create session
	session, err := s.sessionManager.CreateSession(controller, humanPlayer)
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)
		return
	}
	session.Layout = mapLayout
	session.LayoutRaw = layoutRaw

	// Return session info
	response := map[string]interface{}{
		"sessionId": session.ID,
		"game":      ToGameStateDTO(gameModel, humanPlayer),
	}

	s.sendJSON(w, response, http.StatusCreated)
}

// handleGameRoutes routes game-specific requests
func (s *Server) handleGameRoutes(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from path: /api/game/:sessionId/...
	path := strings.TrimPrefix(r.URL.Path, "/api/game/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 {
		s.sendError(w, "Invalid path", http.StatusBadRequest)
		return
	}

	sessionID := parts[0]

	// Get session
	session, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusNotFound)
		return
	}

	// Serialise everything that touches this game.
	//
	// Handlers reach through the session into one shared controller and mutate
	// it; the client polls state on a timer while the player clicks, so
	// concurrent requests on one game are the normal case, not an edge case.
	// Locking once here rather than in thirty handlers means a request either
	// owns the game or waits for it.
	session.Lock()
	defer session.Unlock()

	// Route based on remaining path
	if len(parts) == 1 {
		// /api/game/:sessionId
		switch r.Method {
		case "GET":
			s.handleGetGameState(w, r, session)
		case "DELETE":
			s.handleDeleteGame(w, r, session)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// Route to sub-handlers
	switch parts[1] {
	case "layout":
		s.handleLayout(w, r, session)
	case "territories":
		s.handleTerritories(w, r, session, parts[2:])
	case "territory":
		s.handleTerritory(w, r, session, parts[2:])
	case "available-actions":
		s.handleAvailableActions(w, r, session)
	case "action":
		s.handleAction(w, r, session, parts[2:])
	default:
		s.sendError(w, "Unknown endpoint", http.StatusNotFound)
	}
}

// handleGetGameState handles GET /api/game/:sessionId
func (s *Server) handleGetGameState(w http.ResponseWriter, r *http.Request, session *GameSession) {
	state := ToGameStateDTO(session.Controller.Game, session.HumanPlayer)
	if winner, won, _ := session.Controller.CheckVictoryCondition(); won {
		state.GameOver = true
		state.Winner = winner
	}
	s.sendJSON(w, state, http.StatusOK)
}

// handleDeleteGame handles DELETE /api/game/:sessionId
func (s *Server) handleDeleteGame(w http.ResponseWriter, r *http.Request, session *GameSession) {
	if err := s.sessionManager.DeleteSession(session.ID); err != nil {
		s.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleTerritories handles GET /api/game/:sessionId/territories
func (s *Server) handleTerritories(w http.ResponseWriter, r *http.Request, session *GameSession, parts []string) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	names := make([]string, 0, len(session.Controller.Game.Board))
	for name := range session.Controller.Game.Board {
		names = append(names, name)
	}
	sort.Strings(names)

	territories := make([]TerritoryDTO, 0, len(names))
	for _, name := range names {
		territories = append(territories, ToTerritoryDTO(session.Controller.Game.Board[name]))
	}

	response := map[string]interface{}{
		"territories": territories,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleTerritory handles GET /api/game/:sessionId/territory/:name
func (s *Server) handleTerritory(w http.ResponseWriter, r *http.Request, session *GameSession, parts []string) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if len(parts) == 0 {
		s.sendError(w, "Territory name required", http.StatusBadRequest)
		return
	}

	territoryName := strings.Join(parts, "/") // Handle multi-word names

	territory, exists := session.Controller.Game.Board[territoryName]
	if !exists {
		s.sendError(w, fmt.Sprintf("Territory %s not found", territoryName), http.StatusNotFound)
		return
	}

	// Build detailed territory info
	dto := TerritoryDetailDTO{
		TerritoryDTO: ToTerritoryDTO(territory),
		Units:        make([]UnitDTO, 0),
		ConnectedTerritories: make([]ConnectedTerritoryDTO, 0),
	}

	// Add units
	plannedMoves := session.Controller.GetPlannedMoves()
	movedPieceIDs := make(map[int]bool)
	for _, move := range plannedMoves {
		movedPieceIDs[move.PieceID] = true
	}

	for _, pieceID := range territory.Pieces {
		piece := session.Controller.Game.Pieces[pieceID]
		canMove := !movedPieceIDs[pieceID]
		dto.Units = append(dto.Units, ToUnitDTO(pieceID, piece, canMove))
	}

	// Add connected territories with context
	currentPhase := session.Controller.Game.CurrentPhase
	player := session.Controller.Game.Players[session.HumanPlayer]

	for _, conn := range territory.ConnectedTo {
		connDTO := ConnectedTerritoryDTO{
			Name:      conn.Name,
			Owner:     conn.Owner.Name,
			UnitCount: len(conn.Pieces),
			CanAttack: false,
			CanMoveTo: false,
		}

		// Determine if can attack or move to
		if currentPhase == models.CombatMovePhase {
			if conn.Owner.Name != player.Name {
				connDTO.CanAttack = true
			}
			connDTO.CanMoveTo = true
		} else if currentPhase == models.NoncombatMovePhase {
			// Friendly ground, open water, or a neutral this side may
			// peacefully activate. The old check said any "Neutral"-owned
			// territory was enterable, which invited the player into strict
			// neutrals the server would then refuse.
			switch {
			case conn.Owner.Name == player.Name || conn.Terrain == models.Water:
				connDTO.CanMoveTo = true
			case game.CanActivateNeutral(conn, player):
				connDTO.CanMoveTo = true
			}
		}

		dto.ConnectedTerritories = append(dto.ConnectedTerritories, connDTO)
	}

	s.sendJSON(w, dto, http.StatusOK)
}

// handleAvailableActions handles GET /api/game/:sessionId/available-actions
func (s *Server) handleAvailableActions(w http.ResponseWriter, r *http.Request, session *GameSession) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	g := session.Controller.Game
	player := g.Players[session.HumanPlayer]

	response := map[string]interface{}{
		"phase":       g.CurrentPhase.String(),
		"isHumanTurn": session.isHumanTurnLocked(),
	}

	// Add phase-specific actions
	switch g.CurrentPhase {
	case models.PurchasePhase:
		response["actions"] = map[string]interface{}{
			"purchase": map[string]interface{}{
				"availableUnits": ToAvailableUnitDTOs(g.GlobalPieceTemplates, player.IPCs),
				"currentIPCs":    player.IPCs,
			},
		}

	case models.CombatMovePhase, models.NoncombatMovePhase:
		moves := session.Controller.GetPlannedMoves()
		moveDTOs := make([]MoveDTO, len(moves))
		for i, move := range moves {
			moveDTOs[i] = ToMoveDTO(move)
		}

		response["actions"] = map[string]interface{}{
			"plannedMoves": moveDTOs,
		}

		if g.CurrentPhase == models.CombatMovePhase {
			response["plannedAttacks"] = session.Controller.GetPlannedAttacks()
		}

	case models.ConductCombatPhase:
		battles := make([]string, 0, len(session.Controller.PendingBattles))
		for territory := range session.Controller.PendingBattles {
			battles = append(battles, territory)
		}
		response["actions"] = map[string]interface{}{
			"pendingBattles": battles,
		}

	case models.MobilizePhase:
		purchased := GroupPurchasedUnits(g.PurchasedUnits[player.Name])
		for i := range purchased {
			purchased[i].Targets = session.Controller.PlacementTargets(player, purchased[i].Type)
		}
		response["actions"] = map[string]interface{}{
			"purchasedUnits": purchased,
		}

	case models.CollectIncomePhase:
		income, _ := session.Controller.CalculateIncome(player.Name)
		response["actions"] = map[string]interface{}{
			"income": income,
		}
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleAction routes action requests
func (s *Server) handleAction(w http.ResponseWriter, r *http.Request, session *GameSession, parts []string) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if len(parts) == 0 {
		s.sendError(w, "Action type required", http.StatusBadRequest)
		return
	}

	actionType := parts[0]

	// Every action belongs to the human player except running an NPC's turn,
	// which by definition happens when it is NOT the human's turn. Gating it
	// with the rest deadlocked the whole game: after the human's first turn the
	// browser could neither act (not its turn) nor let the NPC act (same gate).
	if actionType != "execute-npc-turn" && !session.isHumanTurnLocked() {
		s.sendError(w, "Not your turn", http.StatusBadRequest)
		return
	}

	switch actionType {
	case "purchase":
		s.handlePurchaseAction(w, r, session)
	case "plan-move":
		s.handlePlanMoveAction(w, r, session)
	case "cancel-move":
		s.handleCancelMoveAction(w, r, session)
	case "advance-phase":
		s.handleAdvancePhaseAction(w, r, session)
	case "mobilize":
		s.handleMobilizeAction(w, r, session)
	case "resolve-battle":
		s.handleResolveBattleAction(w, r, session)
	case "auto-resolve-battles":
		s.handleAutoResolveBattlesAction(w, r, session)
	case "get-reachable":
		s.handleGetReachableAction(w, r, session)
	case "execute-npc-turn":
		s.handleExecuteNPCTurn(w, r, session)
	default:
		s.sendError(w, fmt.Sprintf("Unknown action: %s", actionType), http.StatusBadRequest)
	}
}

// Helper functions

func (s *Server) sendJSON(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) sendError(w http.ResponseWriter, message string, statusCode int) {
	s.sendJSON(w, map[string]string{"error": message}, statusCode)
}

package webserver

import (
	"boardgame/game"
	"boardgame/models"
	"boardgame/parser"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

	// The frontend is embedded in the binary (see static_assets.go), so it is
	// served identically no matter where the server is started from.
	s.mux.Handle("/", staticHandler())

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

	names := sortedNames(session.Controller.Game)

	territories := make([]TerritoryDTO, 0, len(names))
	for _, name := range names {
		dto := ToTerritoryDTO(session.Controller.Game.Board[name],
			session.Controller.Game, session.HumanPlayer)
		dto.FriendlyUnits = movableUnits(session, session.Controller.Game.Board[name])
		territories = append(territories, dto)
	}

	response := map[string]interface{}{
		"territories": territories,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// movableUnits counts the human player's units in a territory that the unit
// picker would actually offer right now: pieces with movement left this turn
// and no move already planned, plus cargo aboard the player's ships here that
// is not yet booked ashore. This is what decides whether the territory glows
// during a movement phase. Counting every piece instead lit up territories
// whose garrison had spent its whole allowance attacking, and the click then
// opened an empty picker -- which reads as "my troops are stuck here".
//
// Outside the movement phases every piece that could ever move counts, so the
// number means the same thing whichever phase the browser asks in.
func movableUnits(session *GameSession, territory *models.Territory) int {
	g := session.Controller.Game
	units := g.Units()
	tracker := session.Controller.MoveTracker
	inMovementPhase := g.CurrentPhase == models.CombatMovePhase ||
		g.CurrentPhase == models.NoncombatMovePhase

	committed := make(map[int]bool)
	if inMovementPhase {
		for _, move := range session.Controller.GetPlannedMoves() {
			committed[move.PieceID] = true
		}
		for _, landing := range session.Controller.GetPlannedLandings() {
			for _, cargoID := range landing.CargoIDs {
				committed[cargoID] = true
			}
		}
	}

	mine := func(piece *models.Piece) bool {
		return piece != nil && piece.Owner != nil && piece.Owner.Name == session.HumanPlayer
	}

	count := 0
	for _, pieceID := range territory.Pieces {
		piece := g.Pieces[pieceID]
		if !mine(piece) || units.For(piece).IsStructure {
			continue
		}
		// Cargo can always be sent ashore, whatever the ship has done.
		for _, cargoID := range piece.Holding {
			if mine(g.Pieces[cargoID]) && !committed[cargoID] {
				count++
			}
		}
		if piece.Movement == 0 || committed[pieceID] {
			continue
		}
		if inMovementPhase && tracker.Remaining(pieceID, int(piece.Movement)) == 0 {
			continue
		}
		if g.CurrentPhase == models.CombatMovePhase && units.For(piece).IsAA {
			continue // towed in the noncombat phase only
		}
		count++
	}
	return count
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
		TerritoryDTO: ToTerritoryDTO(territory, session.Controller.Game, session.HumanPlayer),
		Units:        make([]UnitDTO, 0),
		ConnectedTerritories: make([]ConnectedTerritoryDTO, 0),
	}
	dto.FriendlyUnits = movableUnits(session, territory)

	// Add units. "Can move" here means "has no commitment yet": pieces with a
	// planned move and cargo booked for a landing are both spoken for, and the
	// unit picker must not offer them a second time.
	plannedMoves := session.Controller.GetPlannedMoves()
	movedPieceIDs := make(map[int]bool)
	for _, move := range plannedMoves {
		movedPieceIDs[move.PieceID] = true
	}
	for _, landing := range session.Controller.GetPlannedLandings() {
		for _, cargoID := range landing.CargoIDs {
			movedPieceIDs[cargoID] = true
		}
	}

	units := session.Controller.Game.Units()
	phase := session.Controller.Game.CurrentPhase
	for _, pieceID := range territory.Pieces {
		piece := session.Controller.Game.Pieces[pieceID]
		// A piece is offered for movement when it has no commitment AND some
		// allowance left this turn -- a unit that spent everything attacking
		// is done until next turn, and the picker must say so.
		whyNot := ""
		switch {
		case units.For(piece).IsStructure || piece.Movement == 0:
			whyNot = "cannot move"
		case movedPieceIDs[pieceID]:
			whyNot = "already moving"
		case session.Controller.MoveTracker.Remaining(pieceID, int(piece.Movement)) == 0:
			if phase == models.NoncombatMovePhase {
				whyNot = "already fought this turn"
			} else {
				whyNot = "no movement left"
			}
		case phase == models.CombatMovePhase && units.For(piece).IsAA:
			whyNot = "moves in noncombat only"
		}
		unitDTO := ToUnitDTO(pieceID, piece, whyNot == "")
		unitDTO.WhyNot = whyNot
		dto.Units = append(dto.Units, unitDTO)

		// Cargo lives in the transport's hold, not the territory's piece list;
		// list it here or the browser can never see or unload it.
		for _, cargoID := range piece.Holding {
			cargo, ok := session.Controller.Game.Pieces[cargoID]
			if !ok {
				continue
			}
			cargoDTO := ToUnitDTO(cargoID, cargo, !movedPieceIDs[cargoID])
			if movedPieceIDs[cargoID] {
				cargoDTO.WhyNot = "booked for a landing"
			}
			cargoDTO.Aboard = pieceID
			dto.Units = append(dto.Units, cargoDTO)
		}
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
			// An attack needs somebody to fight or enemy ground to take;
			// an ally's territory, or a sea zone merely carrying an ally's
			// name, is neither.
			allied := conn.Owner != nil && player.Side != "" && conn.Owner.Side == player.Side
			if hostileForces(session.Controller.Game, conn, player) ||
				(conn.Terrain != models.Water && conn.Owner != player && !allied) {
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
		// Booked amphibious landings ride in the same list, flagged, so the
		// browser draws their arrows and offers them for review alongside
		// ordinary moves.
		moveDTOs = append(moveDTOs, plannedLandingDTOs(session)...)

		response["actions"] = map[string]interface{}{
			"plannedMoves": moveDTOs,
		}

		if g.CurrentPhase == models.CombatMovePhase {
			response["plannedAttacks"] = session.Controller.GetPlannedAttacks()
		}

	case models.ConductCombatPhase:
		names := make([]string, 0, len(session.Controller.PendingBattles))
		for territory := range session.Controller.PendingBattles {
			names = append(names, territory)
		}
		sort.Strings(names)
		battles := make([]PendingBattleDTO, 0, len(names))
		for _, territory := range names {
			dto := ToPendingBattleDTO(g, session.Controller.PendingBattles[territory])
			_, dto.InProgress = session.Controller.LiveBattles[territory]
			battles = append(battles, dto)
		}
		// Bombing raids ride in the same list, flagged, so the map marks
		// them and the battle screen offers to fly them.
		for _, target := range session.Controller.RaidOrder() {
			battles = append(battles, ToPendingRaidDTO(g, session.Controller.PendingRaids[target]))
		}
		response["actions"] = map[string]interface{}{
			"pendingBattles": battles,
		}

	case models.MobilizePhase:
		purchased := GroupPurchasedUnits(g.PurchasedUnits[player.Name])
		// How much each complex can still build this turn, so the browser
		// can offer "place all" honestly and say when a factory is full. A
		// ship launched into a sea zone counts against the yard beside it,
		// so sea-zone targets are mapped to their yard rather than given a
		// capacity of their own.
		capacity := make(map[string]int)
		yardFor := make(map[string]string)
		for i := range purchased {
			purchased[i].Targets = session.Controller.PlacementTargets(player, purchased[i].Type)
			for _, name := range purchased[i].Targets {
				target := g.Board[name]
				if target == nil {
					continue
				}
				if target.Terrain == models.Water {
					if yard := session.Controller.YardWithCapacity(target, player); yard != nil {
						yardFor[name] = yard.Name
						capacity[yard.Name] = session.Controller.FactoryCapacity(yard)
					}
				} else {
					capacity[name] = session.Controller.FactoryCapacity(target)
				}
			}
		}
		response["actions"] = map[string]interface{}{
			"purchasedUnits":  purchased,
			"factoryCapacity": capacity,
			"yardFor":         yardFor,
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
	case "repair-ic":
		s.handleRepairICAction(w, r, session)
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
	case "plan-bombing":
		s.handlePlanBombingAction(w, r, session)
	case "resolve-raid":
		s.handleResolveRaidAction(w, r, session)
	case "battle-begin":
		s.handleBattleBeginAction(w, r, session)
	case "battle-round":
		s.handleBattleRoundAction(w, r, session)
	case "battle-casualties":
		s.handleBattleCasualtiesAction(w, r, session)
	case "battle-retreat":
		s.handleBattleRetreatAction(w, r, session)
	case "battle-submerge":
		s.handleBattleSubmergeAction(w, r, session)
	case "auto-resolve-battles":
		s.handleAutoResolveBattlesAction(w, r, session)
	case "get-reachable":
		s.handleGetReachableAction(w, r, session)
	case "load-transports":
		s.handleLoadTransportsAction(w, r, session)
	case "unload-transport":
		s.handleUnloadTransportAction(w, r, session)
	case "cancel-landing":
		s.handleCancelLandingAction(w, r, session)
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

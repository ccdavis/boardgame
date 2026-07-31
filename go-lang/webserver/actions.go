package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"boardgame/engine"
	"boardgame/game"
	"boardgame/models"
)

// handlePurchaseAction handles POST /api/game/:sessionId/action/purchase
func (s *Server) handlePurchaseAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		UnitType string `json:"unitType"`
		Quantity int    `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Purchase the units
	err := session.Controller.PurchaseUnit(req.UnitType, req.Quantity)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	player := session.Controller.Game.Players[session.HumanPlayer]
	purchased := session.Controller.Game.PurchasedUnits[session.HumanPlayer]

	response := map[string]interface{}{
		"success":        true,
		"remainingIPCs":  player.IPCs,
		"purchasedUnits": GroupPurchasedUnits(purchased),
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handlePlanMoveAction handles POST /api/game/:sessionId/action/plan-move
func (s *Server) handlePlanMoveAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		PieceID int    `json:"pieceId"`
		From    string `json:"from"`
		To      string `json:"to"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Plan the move
	err := session.Controller.PlanMove(req.PieceID, req.From, req.To)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Determine if this creates a battle
	toTerritory := session.Controller.Game.Board[req.To]
	player := session.Controller.Game.Players[session.HumanPlayer]
	willCreateBattle := toTerritory.Owner.Name != player.Name && len(toTerritory.Pieces) > 0

	moveType := "noncombat"
	if session.Controller.Game.CurrentPhase == models.CombatMovePhase {
		moveType = "combat"
	}

	response := map[string]interface{}{
		"success": true,
		"move": map[string]interface{}{
			"pieceId": req.PieceID,
			"from":    req.From,
			"to":      req.To,
			"type":    moveType,
		},
		"willCreateBattle": willCreateBattle,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleCancelMoveAction handles POST /api/game/:sessionId/action/cancel-move
func (s *Server) handleCancelMoveAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		PieceID int `json:"pieceId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := session.Controller.CancelMove(req.PieceID)
	if err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"success": true,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleAdvancePhaseAction handles POST /api/game/:sessionId/action/advance-phase
func (s *Server) handleAdvancePhaseAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	// All of the phase logic lives in engine.Driver now. This handler used to
	// carry its own ~90-line copy, which had already drifted from the terminal's
	// -- and it evaluated every check against session.HumanPlayer rather than
	// the power whose turn it actually is, so during an NPC phase the summary,
	// the unspent-IPC warning, the mobilise gate and the income were all
	// computed for the wrong player.
	driver := session.Driver()

	result, err := driver.AdvancePhase()
	if err != nil {
		var blockers engine.Blockers
		if errors.As(err, &blockers) {
			// Not a failure: the phase is simply not finished.
			s.sendJSON(w, map[string]interface{}{
				"blocked":  true,
				"blockers": blockersToDTO(blockers),
				"summary":  blockers.Error(),
			}, http.StatusConflict)
			return
		}
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if result == nil {
		s.sendJSON(w, map[string]interface{}{
			"advanced": false,
			"summary":  "Phase advance declined",
		}, http.StatusOK)
		return
	}

	response := map[string]interface{}{
		"advanced":        true,
		"summary":         phaseSummary(result),
		"power":           result.Power,
		"previousPhase":   result.From.String(),
		"newPhase":        result.To.String(),
		"newCurrentPower": result.NewPower,
		"turn":            result.NewTurn,
		"turnAdvanced":    result.TurnAdvanced,
		"warnings":        driver.Warnings(),
	}
	if len(result.BattlesCreated) > 0 {
		response["battles"] = result.BattlesCreated
	}
	if result.IncomeCollected > 0 {
		response["incomeCollected"] = result.IncomeCollected
	}

	s.sendJSON(w, response, http.StatusOK)
}

func blockersToDTO(blockers engine.Blockers) []map[string]string {
	out := make([]map[string]string, 0, len(blockers))
	for _, blocker := range blockers {
		out = append(out, map[string]string{"code": blocker.Code, "detail": blocker.Detail})
	}
	return out
}

func phaseSummary(result *engine.PhaseResult) string {
	switch result.From {
	case models.PurchasePhase:
		if len(result.UnitsPurchased) == 0 {
			return "No units purchased"
		}
		total, cost := 0, 0
		for _, unit := range result.UnitsPurchased {
			total += unit.Quantity
			cost += unit.Cost
		}
		return fmt.Sprintf("Purchased %d units for %d IPCs", total, cost)
	case models.CombatMovePhase:
		if len(result.BattlesCreated) == 0 {
			return fmt.Sprintf("Executed %d moves; no battles", result.MovesExecuted)
		}
		return fmt.Sprintf("Executed %d moves; %d battle(s) created",
			result.MovesExecuted, len(result.BattlesCreated))
	case models.NoncombatMovePhase:
		return fmt.Sprintf("Executed %d moves", result.MovesExecuted)
	case models.CollectIncomePhase:
		return fmt.Sprintf("Collected %d IPCs", result.IncomeCollected)
	default:
		return fmt.Sprintf("%s complete", result.From)
	}
}

// handleMobilizeAction handles POST /api/game/:sessionId/action/mobilize
func (s *Server) handleMobilizeAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		UnitType  string `json:"unitType"`
		Territory string `json:"territory"`
		Quantity  int    `json:"quantity"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Place each unit
	for i := 0; i < req.Quantity; i++ {
		err := session.Controller.MobilizeUnit(req.Territory, req.UnitType)
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to place unit %d: %v", i+1, err), http.StatusBadRequest)
			return
		}
	}

	purchased := session.Controller.Game.PurchasedUnits[session.HumanPlayer]

	response := map[string]interface{}{
		"success":        true,
		"remainingUnits": GroupPurchasedUnits(purchased),
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleResolveBattleAction handles POST /api/game/:sessionId/action/resolve-battle
func (s *Server) handleResolveBattleAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		Territory string `json:"territory"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Check if battle exists
	if _, exists := session.Controller.PendingBattles[req.Territory]; !exists {
		s.sendError(w, fmt.Sprintf("No battle pending in %s", req.Territory), http.StatusBadRequest)
		return
	}

	// Resolve the battle
	result, err := session.Controller.ResolveBattle(req.Territory, nil)
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to resolve battle: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"result":  ToBattleResultDTO(req.Territory, result),
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleAutoResolveBattlesAction handles POST /api/game/:sessionId/action/auto-resolve-battles
func (s *Server) handleAutoResolveBattlesAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	// Get list of territories with battles
	territories := make([]string, 0, len(session.Controller.PendingBattles))
	for territory := range session.Controller.PendingBattles {
		territories = append(territories, territory)
	}

	if len(territories) == 0 {
		response := map[string]interface{}{
			"success": true,
			"battles": []BattleResultDTO{},
		}
		s.sendJSON(w, response, http.StatusOK)
		return
	}

	// Resolve all battles
	results := make([]BattleResultDTO, 0, len(territories))
	for _, territory := range territories {
		result, err := session.Controller.ResolveBattle(territory, nil)
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to resolve battle at %s: %v", territory, err), http.StatusInternalServerError)
			return
		}
		results = append(results, ToBattleResultDTO(territory, result))
	}

	response := map[string]interface{}{
		"success": true,
		"battles": results,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleGetReachableAction handles POST /api/game/:sessionId/action/get-reachable
func (s *Server) handleGetReachableAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		PieceID       int    `json:"pieceId"`
		FromTerritory string `json:"fromTerritory"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	piece, exists := session.Controller.Game.Pieces[req.PieceID]
	if !exists {
		s.sendError(w, fmt.Sprintf("Piece %d not found", req.PieceID), http.StatusNotFound)
		return
	}

	// Candidate territories in range by terrain alone, then checked against
	// the real movement rules. The old response used the terrain-only sweep
	// and reported every distance as 1, so the UI highlighted moves the
	// server would then refuse -- blocked paths, hostile waypoints, aircraft
	// with nowhere to land.
	reachable, err := game.GetReachableTerritories(session.Controller.Game, req.PieceID, req.FromTerritory)
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to get reachable territories: %v", err), http.StatusInternalServerError)
		return
	}

	player := session.Controller.Game.Players[session.HumanPlayer]
	currentPhase := session.Controller.Game.CurrentPhase
	moveType := game.NoncombatMove
	if currentPhase == models.CombatMovePhase {
		moveType = game.CombatMove
	}

	reachableDTOs := make([]ReachableTerritoryDTO, 0, len(reachable))
	for _, territory := range reachable {
		distance, _, err := game.CalculateMovementPathForPiece(
			session.Controller.Game, piece, req.FromTerritory, territory.Name, player, moveType)
		if err != nil || distance > int(piece.Movement) {
			continue // not actually reachable under the movement rules
		}

		isAttack := false
		if currentPhase == models.CombatMovePhase && territory.Owner.Name != player.Name {
			isAttack = len(territory.Pieces) > 0
		}

		reachableDTOs = append(reachableDTOs, ReachableTerritoryDTO{
			Name:      territory.Name,
			Distance:  distance,
			Owner:     territory.Owner.Name,
			IsAttack:  isAttack,
			UnitCount: len(territory.Pieces),
		})
	}

	response := map[string]interface{}{
		"piece": map[string]interface{}{
			"id":       req.PieceID,
			"type":     piece.Name,
			"movement": piece.Movement,
		},
		"reachable": reachableDTOs,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// handleExecuteNPCTurn handles POST /api/game/:sessionId/action/execute-npc-turn
func (s *Server) handleExecuteNPCTurn(w http.ResponseWriter, r *http.Request, session *GameSession) {
	// Check if current player is NPC
	currentPlayer := session.Controller.Game.Players[session.Controller.Game.CurrentPower]
	if !currentPlayer.NPC {
		s.sendError(w, "Current player is not an NPC", http.StatusBadRequest)
		return
	}

	// Run through the driver, which always supplies a transcript. Calling
	// TakeTurn with a nil transcript here was a guaranteed panic: the AI logs
	// its first phase before doing anything else.
	err := session.Driver().RunNPCTurn(currentPlayer.Name, nil)
	if err != nil {
		s.sendError(w, fmt.Sprintf("NPC turn failed: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success":          true,
		"player":           currentPlayer.Name,
		"summary":          fmt.Sprintf("%s completed their turn", currentPlayer.Name),
		"newPhase":         session.Controller.Game.CurrentPhase.String(),
		"newCurrentPower":  session.Controller.Game.CurrentPower,
	}

	s.sendJSON(w, response, http.StatusOK)
}

// Helper functions

func countPurchasedCost(units []*models.PendingUnit) int {
	total := 0
	for _, unit := range units {
		total += unit.Cost
	}
	return total
}

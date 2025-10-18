package webserver

import (
	"boardgame/game"
	"boardgame/models"
	"encoding/json"
	"fmt"
	"net/http"
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
	g := session.Controller.Game
	previousPhase := g.CurrentPhase
	player := session.Controller.Game.Players[session.HumanPlayer]

	var summary string
	var warnings []string
	var battles []string

	// Execute phase-specific actions and create summary
	switch previousPhase {
	case models.PurchasePhase:
		purchased := g.PurchasedUnits[player.Name]
		if len(purchased) > 0 {
			summary = fmt.Sprintf("Purchased %d units for %d IPCs", len(purchased), countPurchasedCost(purchased))
		} else {
			summary = "No units purchased"
		}

		// Check for unspent IPCs
		if player.IPCs > 10 {
			warnings = append(warnings, fmt.Sprintf("You have %d unspent IPCs", player.IPCs))
		}

	case models.CombatMovePhase:
		moves := session.Controller.GetPlannedMoves()
		summary = fmt.Sprintf("Executing %d combat moves", len(moves))

		// Execute combat moves
		err := session.Controller.ExecuteCombatMoves()
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to execute combat moves: %v", err), http.StatusBadRequest)
			return
		}

		// Get list of battles
		for territory := range session.Controller.PendingBattles {
			battles = append(battles, territory)
		}

	case models.ConductCombatPhase:
		if len(session.Controller.PendingBattles) > 0 {
			s.sendError(w, fmt.Sprintf("Cannot advance: you still have %d unresolved battles", len(session.Controller.PendingBattles)), http.StatusBadRequest)
			return
		}
		summary = "All battles resolved"

	case models.NoncombatMovePhase:
		moves := session.Controller.GetPlannedMoves()
		summary = fmt.Sprintf("Executing %d noncombat moves", len(moves))

		// Execute noncombat moves
		err := session.Controller.ExecuteNoncombatMoves()
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to execute noncombat moves: %v", err), http.StatusBadRequest)
			return
		}

	case models.MobilizePhase:
		purchased := g.PurchasedUnits[player.Name]
		if len(purchased) > 0 {
			s.sendError(w, fmt.Sprintf("Cannot advance: you still have %d units to place", len(purchased)), http.StatusBadRequest)
			return
		}
		summary = "All units placed"

	case models.CollectIncomePhase:
		income, _ := session.Controller.CalculateIncome(player.Name)
		err := session.Controller.CollectIncome()
		if err != nil {
			s.sendError(w, fmt.Sprintf("Failed to collect income: %v", err), http.StatusInternalServerError)
			return
		}
		summary = fmt.Sprintf("Collected %d IPCs", income)
	}

	// Advance to next phase
	err := session.Controller.AdvancePhase()
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to advance phase: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success":       true,
		"previousPhase": previousPhase.String(),
		"currentPhase":  g.CurrentPhase.String(),
		"summary":       summary,
		"warnings":      warnings,
	}

	if len(battles) > 0 {
		response["battles"] = battles
	}

	s.sendJSON(w, response, http.StatusOK)
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

	// Get reachable territories
	reachable, err := game.GetReachableTerritories(session.Controller.Game, req.PieceID, req.FromTerritory)
	if err != nil {
		s.sendError(w, fmt.Sprintf("Failed to get reachable territories: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to DTOs
	player := session.Controller.Game.Players[session.HumanPlayer]
	currentPhase := session.Controller.Game.CurrentPhase

	reachableDTOs := make([]ReachableTerritoryDTO, len(reachable))
	for i, territory := range reachable {
		isAttack := false
		if currentPhase == models.CombatMovePhase && territory.Owner.Name != player.Name {
			isAttack = len(territory.Pieces) > 0
		}

		// Calculate distance (simplified - could use pathfinding for exact distance)
		reachableDTOs[i] = ReachableTerritoryDTO{
			Name:      territory.Name,
			Distance:  1, // Simplified
			Owner:     territory.Owner.Name,
			IsAttack:  isAttack,
			UnitCount: len(territory.Pieces),
		}
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

	// Create NPC AI player
	npc := game.NewNPCAIPlayer(currentPlayer.Name, "normal")

	// Execute the turn (without transcript for web version)
	err := npc.TakeTurn(session.Controller, nil)
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

package webserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

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

	// Warnings describe the phase being LEFT ("you have 40 unspent IPCs" as
	// you end Purchase), so they must be read before the advance. Read after,
	// they describe the phase that just began -- warning the player about
	// unplaced units the moment the placing phase starts, before they had any
	// chance to place anything.
	warnings := driver.Warnings()

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
		"warnings":        warnings,
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
//
// Accepts either a single pieceId or a list of pieceIds moving together from
// the same territory. With a list, the response is the INTERSECTION: only
// territories every one of those pieces can legally reach, which is what the
// map should highlight when the player has checked off a group to move.
func (s *Server) handleGetReachableAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		PieceID       int    `json:"pieceId"`
		PieceIDs      []int  `json:"pieceIds"`
		FromTerritory string `json:"fromTerritory"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	pieceIDs := req.PieceIDs
	if len(pieceIDs) == 0 {
		pieceIDs = []int{req.PieceID}
	}

	var common map[string]ReachableTerritoryDTO
	for _, pieceID := range pieceIDs {
		reachable, err := reachableForPiece(session, pieceID, req.FromTerritory)
		if err != nil {
			s.sendError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if common == nil {
			common = reachable
			continue
		}
		for name, dto := range common {
			other, ok := reachable[name]
			if !ok {
				delete(common, name)
				continue
			}
			// Report the longest path any of the group needs, so "distance"
			// stays honest for the slowest member.
			if other.Distance > dto.Distance {
				common[name] = other
			}
		}
	}

	names := make([]string, 0, len(common))
	for name := range common {
		names = append(names, name)
	}
	sort.Strings(names)
	reachableDTOs := make([]ReachableTerritoryDTO, 0, len(names))
	for _, name := range names {
		reachableDTOs = append(reachableDTOs, common[name])
	}

	// Transport options are group-level, not per-piece: land units together in
	// one territory may board adjacent transports; cargo together aboard
	// transports in this sea zone may unload onto adjacent friendly shores.
	boarding, boardNotes := transportOptions(session, pieceIDs, req.FromTerritory)
	reachableDTOs = append(reachableDTOs, boarding...)

	response := map[string]interface{}{
		"pieceIds":   pieceIDs,
		"reachable":  reachableDTOs,
		"boardNotes": boardNotes,
	}
	if len(pieceIDs) == 1 {
		if piece, exists := session.Controller.Game.Pieces[pieceIDs[0]]; exists {
			response["piece"] = map[string]interface{}{
				"id":       pieceIDs[0],
				"type":     piece.Name,
				"movement": piece.Movement,
			}
		}
	}

	s.sendJSON(w, response, http.StatusOK)
}

// reachableForPiece answers where one piece may legally end a move this phase.
func reachableForPiece(session *GameSession, pieceID int, from string) (map[string]ReachableTerritoryDTO, error) {
	piece, exists := session.Controller.Game.Pieces[pieceID]
	if !exists {
		return nil, fmt.Errorf("piece %d not found", pieceID)
	}

	// Cargo has no moves of its own -- it goes where its ship goes, or it comes
	// ashore. The pathfinder does not know about holds and would happily offer a
	// loaded infantry a stroll out of the sea zone, hostile shores included, so
	// a loaded piece has no destinations here at all: unloadOptions answers for
	// it instead.
	if session.Controller.Game.IsLoaded(pieceID) {
		return map[string]ReachableTerritoryDTO{}, nil
	}

	// Candidate territories in range by terrain alone, then checked against
	// the real movement rules. The old response used the terrain-only sweep
	// and reported every distance as 1, so the UI highlighted moves the
	// server would then refuse -- blocked paths, hostile waypoints, aircraft
	// with nowhere to land.
	reachable, err := game.GetReachableTerritories(session.Controller.Game, pieceID, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get reachable territories: %v", err)
	}

	player := session.Controller.Game.Players[session.HumanPlayer]
	currentPhase := session.Controller.Game.CurrentPhase
	moveType := game.NoncombatMove
	if currentPhase == models.CombatMovePhase {
		moveType = game.CombatMove
	}

	// Judge range by what the piece has LEFT this turn, not its printed
	// allowance: movement spent attacking in the combat phase is gone. The
	// full allowance lit up destinations that PlanMove then refused.
	remaining := session.Controller.MoveTracker.Remaining(pieceID, int(piece.Movement))

	out := make(map[string]ReachableTerritoryDTO, len(reachable))
	for _, territory := range reachable {
		distance, _, err := game.CalculateMovementPathForPiece(
			session.Controller.Game, piece, from, territory.Name, player, moveType)
		if err != nil || distance > remaining {
			continue // not actually reachable under the movement rules
		}

		// An aircraft may fly OVER any sea zone; whether it may STOP in one is
		// a carrier-deck question the pathfinder defers to the controller.
		// Nothing else about open water is a destination: a plane that ends the
		// turn over it is lost, so a zone with no free deck is offered only
		// during a combat move, and then only if there is a fleet to attack.
		carrier := false
		if piece.Terrain == models.Air && territory.Terrain == models.Water {
			carrier = session.Controller.CarrierSlotFree(piece, territory, player)
			if !carrier && !(moveType == game.CombatMove &&
				hostileForces(session.Controller.Game, territory, player)) {
				continue
			}
		}

		// "Attack" means somebody there will shoot back. Judging it by the
		// territory's nominal owner called every sea zone hostile -- nobody
		// holds an ocean, so its owner is Neutral -- and painted a fighter's
		// own carrier red as if landing on it were a battle.
		isAttack := currentPhase == models.CombatMovePhase &&
			hostileForces(session.Controller.Game, territory, player)

		dto := ReachableTerritoryDTO{
			Name:      territory.Name,
			Distance:  distance,
			Owner:     territory.Owner.Name,
			IsAttack:  isAttack,
			UnitCount: len(territory.Pieces),
		}
		if carrier && !isAttack {
			dto.IsCarrier = true
			dto.Note = carrierDescription(session, territory, player, piece)
		}
		out[territory.Name] = dto
	}
	return out, nil
}

// hostileForces reports whether a territory holds pieces belonging to a power
// this player is at war with. Ownership of the ground says nothing here: a sea
// zone belongs to nobody, and an allied territory may hold an enemy raider.
func hostileForces(g *models.Game, t *models.Territory, player *models.Player) bool {
	for _, id := range t.Pieces {
		piece := g.Pieces[id]
		if piece == nil || piece.Owner == nil || piece.Owner == player {
			continue
		}
		if player.Side == "" || piece.Owner.Side != player.Side {
			return true
		}
	}
	return false
}

// carrierDescription names the decks waiting in a sea zone, for the dialog
// that asks the player to confirm a landing. The seat count is the controller's
// own, so it already discounts decks sailing away and seats promised to other
// aircraft this phase.
func carrierDescription(session *GameSession, zone *models.Territory, player *models.Player, aircraft *models.Piece) string {
	carriers := carriersFor(session.Controller.Game, zone, player, aircraft.Name)
	if len(carriers) == 0 {
		return ""
	}
	// Reads as the object of a sentence -- "will land on the carrier in the
	// North Sea" -- so it carries no leading count when there is only one.
	noun := "carrier"
	if len(carriers) > 1 {
		noun = fmt.Sprintf("%d carriers", len(carriers))
	}
	seats := session.Controller.CarrierSeatsFree(aircraft, zone, player)
	return fmt.Sprintf("%s in %s — %s free",
		noun, zone.Name, plural(seats, "deck space", "deck spaces"))
}

// carriersFor lists the friendly ships in a zone whose decks take this kind of
// aircraft, whether or not they still have room.
func carriersFor(g *models.Game, zone *models.Territory, player *models.Player, aircraft string) []*models.Piece {
	var out []*models.Piece
	for _, id := range zone.Pieces {
		ship := g.Pieces[id]
		if ship == nil || ship.Owner == nil || ship.Capacity == 0 {
			continue
		}
		if ship.Owner != player && (player.Side == "" || ship.Owner.Side != player.Side) {
			continue
		}
		if canCarry(ship, aircraft) {
			out = append(out, ship)
		}
	}
	return out
}

// transcriptLines renders a turn's transcript for a particular viewer. A
// viewer on the acting power's side reads everything; an enemy viewer has the
// secret entries withheld and replaced with a single count, so they learn
// that operations exist but never what they are.
func transcriptLines(entries []game.TranscriptEntry, sameSide bool, power string) []string {
	lines := make([]string, 0, len(entries))
	secrets := 0
	secretAt := -1
	for _, entry := range entries {
		if entry.Secret && !sameSide {
			if secrets == 0 {
				secretAt = len(lines)
			}
			secrets++
			continue
		}
		lines = append(lines, entry.Action)
	}
	if secrets > 0 {
		notice := fmt.Sprintf("%s is working on %d secret operation(s) — details unknown",
			power, secrets)
		lines = append(lines[:secretAt], append([]string{notice}, lines[secretAt:]...)...)
	}
	return lines
}

// handleExecuteNPCTurn handles POST /api/game/:sessionId/action/execute-npc-turn
func (s *Server) handleExecuteNPCTurn(w http.ResponseWriter, r *http.Request, session *GameSession) {
	// Check if current player is NPC
	currentPlayer := session.Controller.Game.Players[session.Controller.Game.CurrentPower]
	if !currentPlayer.NPC {
		s.sendError(w, "Current player is not an NPC", http.StatusBadRequest)
		return
	}

	// Record the turn as it is played and hand the log back to the browser:
	// the transcript dialog is how a human learns what the computer just did,
	// which beats trying to spot the differences on the map.
	transcript := game.NewGameTranscript(currentPlayer.Name + "'s turn")
	err := session.Driver().RunNPCTurn(currentPlayer.Name, transcript)
	if err != nil {
		s.sendError(w, fmt.Sprintf("NPC turn failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Fog of war. Allies share operational planning, so a viewer on the same
	// side reads everything -- including allied operations, which is how a
	// human avoids planning an invasion an NPC ally is already mounting. An
	// enemy viewer gets the secret entries withheld and replaced with a
	// single count: they learn that plans exist, never what they are.
	// (Entries for REVEALED operations are logged as public and pass
	// through: a leak is exactly the enemy learning the details.)
	human := session.Controller.Game.Players[session.HumanPlayer]
	sameSide := human != nil && human.Side != "" && human.Side == currentPlayer.Side
	lines := transcriptLines(transcript.Entries, sameSide, currentPlayer.Name)

	response := map[string]interface{}{
		"success":          true,
		"player":           currentPlayer.Name,
		"summary":          fmt.Sprintf("%s completed their turn", currentPlayer.Name),
		"transcript":       lines,
		"newPhase":         session.Controller.Game.CurrentPhase.String(),
		"newCurrentPower":  session.Controller.Game.CurrentPower,
	}

	s.sendJSON(w, response, http.StatusOK)
}


// transportOptions lists the extra destinations a picked group has by way of
// transports. Both directions are group-level judgements:
//
//   - a group of land units standing in FROM can board in an adjacent sea zone
//     when the player's transports there can actually take the whole group;
//   - a group of cargo pieces aboard transports in FROM can unload onto an
//     adjacent shore: immediately if the shore is friendly, or as a booked
//     amphibious assault (executed with the combat moves) if it is hostile.
//     See unloadOptions for the split.
//
// It also returns the reasons a nearby sea zone was NOT offered as a boarding
// point. Silence is the worst answer here: the picker starts with the whole
// stack selected, one transport holds four, and aircraft cannot board at all,
// so the common first attempt at an invasion is a group that fits nowhere.
// Without a reason the sea zone simply fails to light up and the player is
// left to guess that the rule exists.
func transportOptions(session *GameSession, pieceIDs []int, from string) ([]ReachableTerritoryDTO, []string) {
	g := session.Controller.Game
	player := g.Players[session.HumanPlayer]
	fromTerr, ok := g.Board[from]
	if !ok || player == nil || len(pieceIDs) == 0 {
		return nil, nil
	}

	pieces := make([]*models.Piece, 0, len(pieceIDs))
	cargoCount, airCount := 0, 0
	for _, id := range pieceIDs {
		piece, ok := g.Pieces[id]
		if !ok {
			return nil, nil
		}
		pieces = append(pieces, piece)
		if g.IsLoaded(id) {
			cargoCount++
		}
		if piece.Terrain == models.Air {
			airCount++
		}
	}

	switch {
	case cargoCount == len(pieces):
		return unloadOptions(session, g, player, fromTerr, pieceIDs), nil
	case cargoCount > 0:
		// A mix of cargo and free units has no shared destination.
		return nil, []string{"Units aboard transports and units ashore cannot " +
			"move together — pick one group or the other."}
	case airCount == len(pieces):
		// Aircraft want decks, not holds. Their sea destinations come from the
		// reachability sweep, marked as carrier landings; all that is owed here
		// is an explanation when a nearby deck turns them away.
		return nil, carrierNotes(session, g, player, fromTerr, pieces)
	case fromTerr.Terrain == models.Water:
		return nil, nil // ships at sea have no transport business of their own
	default:
		return boardOptions(g, player, fromTerr, pieces)
	}
}

// carrierNotes explains why an aircraft group in range of a friendly carrier
// was not offered its sea zone. Without this the zone simply fails to light up
// and the player, who can plainly see a carrier sitting there, is left to
// conclude the interface is broken -- and then loses the plane at the end of
// the turn to a rule nobody stated.
func carrierNotes(session *GameSession, g *models.Game, player *models.Player, fromTerr *models.Territory, group []*models.Piece) []string {
	moveType := game.NoncombatMove
	if g.CurrentPhase == models.CombatMovePhase {
		moveType = game.CombatMove
	}

	// One aircraft speaks for the group. The picker offers a whole stack at
	// once and the destinations it gets are the intersection, so a refusal for
	// the first is a refusal for all of them; a list of near-identical
	// sentences would only bury the answer.
	aircraft := group[0]
	remaining := session.Controller.MoveTracker.Remaining(aircraft.ID, int(aircraft.Movement))

	var notes []string
	for _, name := range sortedNames(g) {
		zone := g.Board[name]
		// A zone with no flight deck of ours in it is most of the ocean, and
		// remarking on each one would drown the answer the player needs.
		if zone.Terrain != models.Water || !anyFlightDeck(g, zone, player) {
			continue
		}

		distance, _, err := game.CalculateMovementPathForPiece(
			g, aircraft, fromTerr.Name, zone.Name, player, moveType)
		if err != nil || distance == 0 || distance > remaining {
			continue // out of range: not a refusal, just far away
		}

		switch {
		case len(carriersFor(g, zone, player, aircraft.Name)) == 0:
			notes = append(notes, fmt.Sprintf(
				"%s: no carrier there has a deck for a %s.", zone.Name, aircraft.Name))
		case session.Controller.CarrierSeatsFree(aircraft, zone, player) == 0:
			notes = append(notes, fmt.Sprintf(
				"%s: every deck there is spoken for — cancel a landing, or send the %s elsewhere.",
				zone.Name, aircraft.Name))
		}
	}
	return notes
}

// anyFlightDeck reports whether a zone holds a friendly ship that carries
// aircraft of any kind. A transport is not one: its hold takes troops, and a
// player who parked a fighter over it is owed no note about carriers.
func anyFlightDeck(g *models.Game, zone *models.Territory, player *models.Player) bool {
	for _, id := range zone.Pieces {
		ship := g.Pieces[id]
		if ship == nil || ship.Owner == nil || ship.Capacity == 0 {
			continue
		}
		if ship.Owner != player && (player.Side == "" || ship.Owner.Side != player.Side) {
			continue
		}
		for _, kind := range ship.CanCarry {
			if template, ok := g.GlobalPieceTemplates[kind]; ok && template.Terrain == models.Air {
				return true
			}
		}
	}
	return false
}

// sortedNames keeps the notes in a stable order; Go's map iteration is not.
func sortedNames(g *models.Game) []string {
	names := make([]string, 0, len(g.Board))
	for name := range g.Board {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// boardOptions: adjacent sea zones whose friendly transports can take the
// whole group, plus a plain-English reason for each zone that has transports
// but was refused.
func boardOptions(g *models.Game, player *models.Player, fromTerr *models.Territory, group []*models.Piece) ([]ReachableTerritoryDTO, []string) {
	// Whether to explain anything at all: a group with no sea zone next to it
	// was never trying to board, and saying so would be noise on every inland
	// move. Only the coast gets an explanation.
	coastal := false
	for _, zone := range fromTerr.ConnectedTo {
		if zone.Terrain == models.Water {
			coastal = true
			break
		}
	}

	air := 0
	for _, piece := range group {
		if piece.Terrain != models.Land {
			air++
		}
	}
	if air > 0 {
		if !coastal {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf(
			"Only land units can board transports — leave the %s out of the group.",
			plural(air, "aircraft", "aircraft"))}
	}

	var out []ReachableTerritoryDTO
	var notes []string
	for _, zone := range fromTerr.ConnectedTo {
		if zone.Terrain != models.Water {
			continue
		}

		// Loading under the guns of an enemy fleet is not allowed (the same
		// rule ValidateLoad enforces per piece).
		hostile := false
		freeSlots := make(map[*models.Piece]int)
		room := 0
		for _, pieceID := range zone.Pieces {
			ship := g.Pieces[pieceID]
			if ship == nil || ship.Owner == nil {
				continue
			}
			if ship.Owner.Side != player.Side {
				hostile = true
				break
			}
			if ship.Owner == player && ship.Capacity > 0 {
				free := int(ship.Capacity) - len(ship.Holding)
				freeSlots[ship] = free
				room += free
			}
		}
		if hostile {
			notes = append(notes, fmt.Sprintf(
				"%s: an enemy fleet is there — you cannot load under its guns.", zone.Name))
			continue
		}
		if len(freeSlots) == 0 {
			// No transports of yours at all is the ordinary state of most sea
			// zones, and not worth remarking on.
			continue
		}
		if room == 0 {
			notes = append(notes, fmt.Sprintf(
				"%s: your transports there are already full.", zone.Name))
			continue
		}

		// A unit type no hull in the zone may carry (an AA gun, say) is a
		// different problem from one too many infantry, and the player fixes
		// it differently. Report it as itself.
		var refused []string
		for _, piece := range group {
			carriable := false
			for ship := range freeSlots {
				if canCarry(ship, piece.Name) {
					carriable = true
					break
				}
			}
			if !carriable && !contains(refused, piece.Name) {
				refused = append(refused, piece.Name)
			}
		}
		if len(refused) > 0 {
			notes = append(notes, fmt.Sprintf(
				"%s: your transports there cannot carry %s.",
				zone.Name, strings.Join(refused, " or ")))
			continue
		}

		// Greedy assignment: every unit in the group must find a transport
		// with a free slot that is allowed to carry its type.
		fits := true
		for _, piece := range group {
			assigned := false
			for ship, free := range freeSlots {
				if free <= 0 || !canCarry(ship, piece.Name) {
					continue
				}
				freeSlots[ship] = free - 1
				assigned = true
				break
			}
			if !assigned {
				fits = false
				break
			}
		}
		if !fits {
			// Room is counted in slots, so the shortfall is stated in the terms
			// the player must act on: take fewer units, or bring more hulls.
			notes = append(notes, fmt.Sprintf(
				"%s: your transports there have room for %s, and you picked %d — "+
					"select fewer units.",
				zone.Name, plural(room, "more unit", "more units"), len(group)))
			continue
		}

		out = append(out, ReachableTerritoryDTO{
			Name:      zone.Name,
			Distance:  1,
			Owner:     zone.Owner.Name,
			UnitCount: len(zone.Pieces),
			IsBoard:   true,
		})
	}

	// A zone that works needs no apology for the ones that do not.
	if len(out) > 0 {
		return out, nil
	}
	return nil, notes
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// unloadOptions: the shores this cargo can come out on.
//
// Friendly shores adjacent to the transports' CURRENT zone unload immediately.
// During the combat-move phase, hostile shores are offered too -- an
// amphibious assault, booked through the move planner and executed with the
// other combat moves. Because a transport may sail and land in the same
// phase, assault shores adjacent to a transport's PLANNED destination count
// as well as those adjacent to where it sits now.
func unloadOptions(session *GameSession, g *models.Game, player *models.Player, fromTerr *models.Territory, cargoIDs []int) []ReachableTerritoryDTO {
	combatPhase := g.CurrentPhase == models.CombatMovePhase

	// The sea zones the cargo's transports will be adjacent-capable from:
	// where they are, plus where they are planned to sail.
	zones := map[string]*models.Territory{fromTerr.Name: fromTerr}
	if combatPhase {
		for _, cargoID := range cargoIDs {
			transportID := g.GetTransportForPiece(cargoID)
			if transportID == -1 {
				continue
			}
			for _, move := range session.Controller.MoveTracker.Moves {
				if move.PieceID == transportID {
					if dest := g.Board[move.To]; dest != nil {
						zones[dest.Name] = dest
					}
				}
			}
		}
	}

	seen := make(map[string]bool)
	var out []ReachableTerritoryDTO
	for _, zone := range zones {
		for _, shore := range zone.ConnectedTo {
			if shore.Terrain == models.Water || seen[shore.Name] {
				continue
			}

			if friendlyGround(shore, player) {
				// Immediate unload: only legal from the transports' current
				// position, which is what ValidateUnload checks.
				if zone != fromTerr {
					continue
				}
				allValid := true
				for _, cargoID := range cargoIDs {
					transportID := g.GetTransportForPiece(cargoID)
					if transportID == -1 || game.ValidateUnload(g, transportID, cargoID, shore.Name) != nil {
						allValid = false
						break
					}
				}
				if !allValid {
					continue
				}
				seen[shore.Name] = true
				out = append(out, ReachableTerritoryDTO{
					Name:      shore.Name,
					Distance:  1,
					Owner:     shore.Owner.Name,
					UnitCount: len(shore.Pieces),
					IsUnload:  true,
				})
				continue
			}

			// Hostile or neutral shore: an assault landing, combat phase only,
			// and only where the neutral rules allow this power to attack --
			// the same gate an overland invasion passes through.
			if !combatPhase || !game.CanAttackNeutral(shore, player) {
				continue
			}
			seen[shore.Name] = true
			out = append(out, ReachableTerritoryDTO{
				Name:      shore.Name,
				Distance:  1,
				Owner:     shore.Owner.Name,
				UnitCount: len(shore.Pieces),
				IsUnload:  true,
				IsAttack:  true,
			})
		}
	}
	return out
}

func canCarry(ship *models.Piece, unitType string) bool {
	for _, allowed := range ship.CanCarry {
		if allowed == unitType {
			return true
		}
	}
	return false
}

// friendlyGround: owned by the player or a power on the same side.
func friendlyGround(t *models.Territory, player *models.Player) bool {
	if t.Owner == nil {
		return false
	}
	return t.Owner == player || (player.Side != "" && t.Owner.Side == player.Side)
}

// handleLoadTransportsAction handles POST .../action/load-transports.
// Boards each listed piece onto some transport of the player's in the given
// sea zone, greedily. Loading takes effect immediately (it is not a planned
// move); the client refreshes its state afterwards.
func (s *Server) handleLoadTransportsAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		PieceIDs []int  `json:"pieceIds"`
		SeaZone  string `json:"seaZone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	g := session.Controller.Game
	player := g.Players[session.HumanPlayer]
	zone, ok := g.Board[req.SeaZone]
	if !ok {
		s.sendError(w, fmt.Sprintf("Sea zone %s not found", req.SeaZone), http.StatusBadRequest)
		return
	}

	loaded := 0
	for _, pieceID := range req.PieceIDs {
		piece := g.Pieces[pieceID]
		if piece == nil {
			s.sendError(w, fmt.Sprintf("Piece %d not found", pieceID), http.StatusBadRequest)
			return
		}
		boarded := false
		for _, shipID := range zone.Pieces {
			ship := g.Pieces[shipID]
			if ship == nil || ship.Owner != player || ship.Capacity == 0 ||
				len(ship.Holding) >= int(ship.Capacity) || !canCarry(ship, piece.Name) {
				continue
			}
			if err := session.Controller.LoadUnit(shipID, pieceID); err == nil {
				boarded = true
				break
			}
		}
		if !boarded {
			s.sendError(w, fmt.Sprintf(
				"No transport in %s can take the %s (loaded %d of %d)",
				req.SeaZone, piece.Name, loaded, len(req.PieceIDs)), http.StatusBadRequest)
			return
		}
		loaded++
	}

	s.sendJSON(w, map[string]interface{}{"success": true, "loaded": loaded}, http.StatusOK)
}

// handleUnloadTransportAction handles POST .../action/unload-transport.
// Unloads each listed cargo piece onto the given friendly territory.
func (s *Server) handleUnloadTransportAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		PieceIDs  []int  `json:"pieceIds"`
		Territory string `json:"territory"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	g := session.Controller.Game
	player := g.Players[session.HumanPlayer]
	shore, ok := g.Board[req.Territory]
	if !ok {
		s.sendError(w, fmt.Sprintf("Territory %s not found", req.Territory), http.StatusBadRequest)
		return
	}
	// A hostile shore is an amphibious assault: booked through the move
	// planner and executed with the combat moves, so the troops arrive as
	// registered attackers with bombardment support -- exactly as the NPC's
	// landings do. Only friendly shores unload immediately.
	if !friendlyGround(shore, player) {
		if err := session.Controller.PlanLanding(req.PieceIDs, req.Territory); err != nil {
			s.sendError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.sendJSON(w, map[string]interface{}{
			"success": true,
			"planned": true,
			"landing": len(req.PieceIDs),
		}, http.StatusOK)
		return
	}

	unloaded := 0
	for _, pieceID := range req.PieceIDs {
		transportID := g.GetTransportForPiece(pieceID)
		if transportID == -1 {
			s.sendError(w, fmt.Sprintf("Piece %d is not aboard a transport (unloaded %d of %d)",
				pieceID, unloaded, len(req.PieceIDs)), http.StatusBadRequest)
			return
		}
		if err := session.Controller.UnloadUnit(transportID, pieceID, req.Territory); err != nil {
			s.sendError(w, fmt.Sprintf("%v (unloaded %d of %d)", err, unloaded, len(req.PieceIDs)),
				http.StatusBadRequest)
			return
		}
		unloaded++
	}

	s.sendJSON(w, map[string]interface{}{"success": true, "unloaded": unloaded}, http.StatusOK)
}

// handleCancelLandingAction handles POST .../action/cancel-landing.
// Takes a booked amphibious unit off its landing; it stays aboard.
func (s *Server) handleCancelLandingAction(w http.ResponseWriter, r *http.Request, session *GameSession) {
	var req struct {
		PieceID int `json:"pieceId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := session.Controller.CancelLanding(req.PieceID); err != nil {
		s.sendError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.sendJSON(w, map[string]interface{}{"success": true}, http.StatusOK)
}

// plannedLandingDTOs renders booked landings in the same shape as planned
// moves, so the browser can draw arrows and list them for review. The arrow
// starts where the assault actually launches from: the transport's planned
// destination if it is booked to sail, else its current zone.
func plannedLandingDTOs(session *GameSession) []MoveDTO {
	g := session.Controller.Game
	var out []MoveDTO
	for _, landing := range session.Controller.GetPlannedLandings() {
		for _, cargoID := range landing.CargoIDs {
			transportID := g.GetTransportForPiece(cargoID)
			if transportID == -1 {
				continue
			}
			from := ""
			for _, move := range session.Controller.MoveTracker.Moves {
				if move.PieceID == transportID {
					from = move.To
				}
			}
			if from == "" {
				if zone := territoryNameOf(g, transportID); zone != "" {
					from = zone
				}
			}
			out = append(out, MoveDTO{
				PieceID: cargoID,
				From:    from,
				To:      landing.Target,
				Type:    "combat",
				Landing: true,
			})
		}
	}
	return out
}

// territoryNameOf finds which territory currently lists a piece.
func territoryNameOf(g *models.Game, pieceID int) string {
	for name, territory := range g.Board {
		for _, id := range territory.Pieces {
			if id == pieceID {
				return name
			}
		}
	}
	return ""
}

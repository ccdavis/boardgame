package game

import (
	"boardgame/models"
	"fmt"
)

// GameController manages the game flow and turn sequence
type GameController struct {
	Game           *models.Game
	MoveTracker    *MovementTracker
	PendingBattles map[string]*Battle // Territory name -> Battle
}

// NewGameController creates a new controller for a game
func NewGameController(game *models.Game) *GameController {
	return &GameController{
		Game:           game,
		MoveTracker:    NewMovementTracker(),
		PendingBattles: make(map[string]*Battle),
	}
}

// StartGame initializes the game with the first player's turn
func (gc *GameController) StartGame() error {
	if len(gc.Game.PlayerOrder) == 0 {
		return fmt.Errorf("no players in game")
	}

	// Set to first player in turn order
	gc.Game.CurrentPower = gc.Game.PlayerOrder[0]
	gc.Game.CurrentPhase = models.PurchasePhase
	gc.Game.Turn = 1

	return nil
}

// AdvancePhase moves to the next phase in the current player's turn
func (gc *GameController) AdvancePhase() error {
	switch gc.Game.CurrentPhase {
	case models.PurchasePhase:
		gc.Game.CurrentPhase = models.CombatMovePhase
	case models.CombatMovePhase:
		gc.Game.CurrentPhase = models.ConductCombatPhase
	case models.ConductCombatPhase:
		gc.Game.CurrentPhase = models.NoncombatMovePhase
	case models.NoncombatMovePhase:
		gc.Game.CurrentPhase = models.MobilizePhase
	case models.MobilizePhase:
		gc.Game.CurrentPhase = models.CollectIncomePhase
	case models.CollectIncomePhase:
		// End of turn, advance to next player
		return gc.AdvanceTurn()
	default:
		return fmt.Errorf("unknown phase: %v", gc.Game.CurrentPhase)
	}

	return nil
}

// AdvanceTurn moves to the next player's turn
func (gc *GameController) AdvanceTurn() error {
	if len(gc.Game.PlayerOrder) == 0 {
		return fmt.Errorf("no players in game")
	}

	// Find current player index
	currentIndex := -1
	for i, name := range gc.Game.PlayerOrder {
		if name == gc.Game.CurrentPower {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return fmt.Errorf("current player %s not found in player order", gc.Game.CurrentPower)
	}

	// Move to next player
	nextIndex := (currentIndex + 1) % len(gc.Game.PlayerOrder)
	gc.Game.CurrentPower = gc.Game.PlayerOrder[nextIndex]
	gc.Game.CurrentPhase = models.PurchasePhase

	// If we wrapped around to the first player, increment turn number
	if nextIndex == 0 {
		gc.Game.Turn++
	}

	return nil
}

// GetCurrentPlayer returns the player whose turn it is
func (gc *GameController) GetCurrentPlayer() (*models.Player, error) {
	player, exists := gc.Game.Players[gc.Game.CurrentPower]
	if !exists {
		return nil, fmt.Errorf("current player %s not found", gc.Game.CurrentPower)
	}
	return player, nil
}

// CalculateIncome calculates a player's total income from territories
func (gc *GameController) CalculateIncome(playerName string) (int, error) {
	player, exists := gc.Game.Players[playerName]
	if !exists {
		return 0, fmt.Errorf("player %s not found", playerName)
	}

	income := 0
	for _, territory := range player.Territories {
		income += territory.Production
	}

	return income, nil
}

// CollectIncome adds income to the current player's treasury
func (gc *GameController) CollectIncome() error {
	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	income, err := gc.CalculateIncome(player.Name)
	if err != nil {
		return err
	}

	player.IPCs += income
	return nil
}

// CheckVictoryCondition checks if any side has won the game
// Returns: winner ("Axis" or "Allies"), hasWon (bool), error
func (gc *GameController) CheckVictoryCondition() (string, bool, error) {
	// Count victory cities by side
	// According to Axis & Allies 1942 rules:
	// Axis powers: Germany, Japan
	// Allied powers: USSR, UK, USA

	axisCities := 0
	alliedCities := 0
	totalCities := 0

	for _, territory := range gc.Game.Board {
		if territory.IsVictoryCity {
			totalCities++
			owner := territory.Owner.Name

			switch owner {
			case "Germany", "Japan":
				axisCities++
			case "USSR", "UK", "USA":
				alliedCities++
			}
		}
	}

	// Victory conditions (from rulebook):
	// - Axis wins if they control 9 cities for a full round
	// - Allies win if they control 10 cities for a full round
	// - Either side wins immediately if they control 13+ cities

	// Immediate victory: 13+ cities
	if axisCities >= 13 {
		return "Axis", true, nil
	}
	if alliedCities >= 13 {
		return "Allies", true, nil
	}

	// Sustained victory: Axis 9+, Allies 10+
	// (Note: Tracking "for a full round" requires additional state -
	// for now we'll just report these thresholds)
	if axisCities >= 9 {
		return "Axis", false, nil // Potential victory
	}
	if alliedCities >= 10 {
		return "Allies", false, nil // Potential victory
	}

	return "", false, nil
}

// GetVictoryCityCounts returns the number of victory cities each side controls
func (gc *GameController) GetVictoryCityCounts() (axis int, allies int) {
	for _, territory := range gc.Game.Board {
		if territory.IsVictoryCity {
			owner := territory.Owner.Name

			switch owner {
			case "Germany", "Japan":
				axis++
			case "USSR", "UK", "USA":
				allies++
			}
		}
	}
	return axis, allies
}

// PurchaseUnit allows the current player to purchase a unit
func (gc *GameController) PurchaseUnit(unitType string, quantity int) error {
	if gc.Game.CurrentPhase != models.PurchasePhase {
		return fmt.Errorf("can only purchase units during Purchase phase")
	}

	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	// Check if unit type exists
	template, exists := gc.Game.GlobalPieceTemplates[unitType]
	if !exists {
		return fmt.Errorf("unit type %s not found", unitType)
	}

	// Calculate total cost
	totalCost := int(template.Cost) * quantity
	if player.IPCs < totalCost {
		return fmt.Errorf("insufficient IPCs: need %d, have %d", totalCost, player.IPCs)
	}

	// Deduct cost
	player.IPCs -= totalCost

	// Add to purchased units
	if gc.Game.PurchasedUnits[player.Name] == nil {
		gc.Game.PurchasedUnits[player.Name] = make([]*models.PendingUnit, 0)
	}

	for i := 0; i < quantity; i++ {
		gc.Game.PurchasedUnits[player.Name] = append(
			gc.Game.PurchasedUnits[player.Name],
			&models.PendingUnit{
				Type: unitType,
				Cost: int(template.Cost),
			},
		)
	}

	return nil
}

// MobilizeUnit places a purchased unit on the board at an industrial complex
func (gc *GameController) MobilizeUnit(territoryName string, unitType string) error {
	if gc.Game.CurrentPhase != models.MobilizePhase {
		return fmt.Errorf("can only mobilize units during Mobilize phase")
	}

	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	// Check if territory exists and is owned by player
	territory, exists := gc.Game.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	if territory.Owner != player {
		return fmt.Errorf("you do not own %s", territoryName)
	}

	// Check if player has purchased this unit type
	pending := gc.Game.PurchasedUnits[player.Name]
	unitIndex := -1
	for i, unit := range pending {
		if unit.Type == unitType {
			unitIndex = i
			break
		}
	}

	if unitIndex == -1 {
		return fmt.Errorf("no purchased %s units available", unitType)
	}

	// Remove from purchased units
	gc.Game.PurchasedUnits[player.Name] = append(
		pending[:unitIndex],
		pending[unitIndex+1:]...,
	)

	// Place the unit on the board
	err = gc.Game.PlacePieces(territoryName, unitType, 1)
	if err != nil {
		return fmt.Errorf("failed to place unit: %v", err)
	}

	return nil
}

// RepairIndustrialComplex repairs damage to an IC
func (gc *GameController) RepairIndustrialComplex(territoryName string, amount int) error {
	if gc.Game.CurrentPhase != models.PurchasePhase {
		return fmt.Errorf("can only repair ICs during Purchase phase")
	}

	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	territory, exists := gc.Game.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	if territory.Owner != player {
		return fmt.Errorf("you do not own %s", territoryName)
	}

	// Can't repair more damage than exists
	if amount > territory.ICDamage {
		amount = territory.ICDamage
	}

	// Repair costs 1 IPC per damage point
	if player.IPCs < amount {
		return fmt.Errorf("insufficient IPCs to repair: need %d, have %d", amount, player.IPCs)
	}

	player.IPCs -= amount
	territory.ICDamage -= amount

	return nil
}

// PlanMove plans a unit movement during combat or noncombat move phase
func (gc *GameController) PlanMove(pieceID int, from, to string) error {
	// Determine move type based on current phase
	var moveType MoveType
	switch gc.Game.CurrentPhase {
	case models.CombatMovePhase:
		moveType = CombatMove
	case models.NoncombatMovePhase:
		moveType = NoncombatMove
	default:
		return fmt.Errorf("can only move during Combat Move or Noncombat Move phase")
	}

	// Validate the move
	err := ValidateMovement(gc.Game, pieceID, from, to, moveType)
	if err != nil {
		return err
	}

	// Check if piece can reach territory
	canReach, err := CanReachTerritory(gc.Game, pieceID, from, to)
	if err != nil {
		return err
	}
	if !canReach {
		piece := gc.Game.Pieces[pieceID]
		return fmt.Errorf("piece %s cannot reach %s from %s (movement=%d)",
			piece.Name, to, from, piece.Movement)
	}

	// Add to movement tracker
	err = gc.MoveTracker.AddMove(pieceID, from, to, moveType)
	if err != nil {
		return err
	}

	return nil
}

// CancelMove cancels a planned move
func (gc *GameController) CancelMove(pieceID int) error {
	return gc.MoveTracker.RemoveMove(pieceID)
}

// ExecuteCombatMoves executes all combat moves and sets up battles
func (gc *GameController) ExecuteCombatMoves() error {
	if gc.Game.CurrentPhase != models.CombatMovePhase {
		return fmt.Errorf("can only execute combat moves during Combat Move phase")
	}

	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	// Get all combat moves
	combatMoves := gc.MoveTracker.GetMovesByType(CombatMove)

	// Execute each move
	for _, move := range combatMoves {
		err := gc.Game.MovePiece(move.PieceID, move.From, move.To)
		if err != nil {
			return fmt.Errorf("failed to execute move: %v", err)
		}

		// Check if move creates a battle
		toTerritory := gc.Game.Board[move.To]
		if toTerritory.Owner.Name != player.Name {
			// Moving into hostile territory - create battle
			if _, exists := gc.PendingBattles[move.To]; !exists {
				// Create new battle
				battle := NewBattle(move.To, LandBattle, player.Name, toTerritory.Owner.Name)
				gc.PendingBattles[move.To] = battle
			}
			// Track this piece as an attacker
			gc.PendingBattles[move.To].AttackingPieceIDs = append(
				gc.PendingBattles[move.To].AttackingPieceIDs, move.PieceID)
		}
	}

	// Clear combat moves from tracker
	newMoves := gc.MoveTracker.GetMovesByType(NoncombatMove)
	gc.MoveTracker.Clear()
	for _, move := range newMoves {
		gc.MoveTracker.AddMove(move.PieceID, move.From, move.To, move.Type)
	}

	return nil
}

// ExecuteNoncombatMoves executes all noncombat moves
func (gc *GameController) ExecuteNoncombatMoves() error {
	if gc.Game.CurrentPhase != models.NoncombatMovePhase {
		return fmt.Errorf("can only execute noncombat moves during Noncombat Move phase")
	}

	// Get all noncombat moves
	noncombatMoves := gc.MoveTracker.GetMovesByType(NoncombatMove)

	// Execute each move
	for _, move := range noncombatMoves {
		err := gc.Game.MovePiece(move.PieceID, move.From, move.To)
		if err != nil {
			return fmt.Errorf("failed to execute move: %v", err)
		}
	}

	// Clear all moves
	gc.MoveTracker.Clear()

	return nil
}

// GetPlannedMoves returns all currently planned moves
func (gc *GameController) GetPlannedMoves() []*Move {
	return gc.MoveTracker.Moves
}

// GetPlannedAttacks returns list of territories that will be attacked
func (gc *GameController) GetPlannedAttacks() []string {
	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return []string{}
	}

	attacks := make(map[string]bool)
	combatMoves := gc.MoveTracker.GetMovesByType(CombatMove)

	for _, move := range combatMoves {
		toTerritory := gc.Game.Board[move.To]
		if toTerritory.Owner.Name != player.Name {
			attacks[move.To] = true
		}
	}

	result := make([]string, 0, len(attacks))
	for territory := range attacks {
		result = append(result, territory)
	}

	return result
}

// ResolveBattle resolves a battle in a territory and handles territory capture
func (gc *GameController) ResolveBattle(territoryName string, diceRoller *DiceRoller) (*BattleResult, error) {
	battle, exists := gc.PendingBattles[territoryName]
	if !exists {
		return nil, fmt.Errorf("no battle pending in %s", territoryName)
	}

	// Get attacker and territory
	territory := gc.Game.Board[territoryName]
	attacker := gc.Game.Players[battle.AttackerID]

	// Populate battle with actual pieces using the tracked attacking piece IDs
	attackerPieces := make([]*models.Piece, 0)
	defenderPieces := make([]*models.Piece, 0)

	// Create a map of attacking piece IDs for quick lookup
	attackingIDs := make(map[int]bool)
	for _, id := range battle.AttackingPieceIDs {
		attackingIDs[id] = true
	}

	// Separate pieces into attackers and defenders
	for _, pieceID := range territory.Pieces {
		piece := gc.Game.Pieces[pieceID]
		if attackingIDs[pieceID] {
			attackerPieces = append(attackerPieces, piece)
		} else {
			defenderPieces = append(defenderPieces, piece)
		}
	}

	battle.Attackers = attackerPieces
	battle.Defenders = defenderPieces

	// Resolve the combat
	result, err := ResolveCombat(battle, diceRoller, 100)
	if err != nil {
		return nil, err
	}

	// Remove casualties from the board
	for _, casualty := range result.AttackerCasualties {
		gc.removePieceFromBoard(casualty, territoryName)
	}
	for _, casualty := range result.DefenderCasualties {
		gc.removePieceFromBoard(casualty, territoryName)
	}

	// Handle territory capture
	if result.AttackerWins {
		err = gc.CaptureTerritory(territoryName, attacker.Name)
		if err != nil {
			return result, fmt.Errorf("failed to capture territory: %v", err)
		}
	}

	// Remove battle from pending
	delete(gc.PendingBattles, territoryName)

	return result, nil
}

// CaptureTerritory transfers ownership of a territory
func (gc *GameController) CaptureTerritory(territoryName, newOwnerName string) error {
	territory, exists := gc.Game.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	newOwner, exists := gc.Game.Players[newOwnerName]
	if !exists {
		return fmt.Errorf("player %s not found", newOwnerName)
	}

	// Use the existing ChangeOwnership function from models
	models.ChangeOwnership(territory, newOwner)

	return nil
}

// removePieceFromBoard removes a piece from the game entirely
func (gc *GameController) removePieceFromBoard(piece *models.Piece, territoryName string) {
	// Find the piece ID
	var pieceID int
	for id, p := range gc.Game.Pieces {
		if p == piece {
			pieceID = id
			break
		}
	}

	// Remove from territory
	territory := gc.Game.Board[territoryName]
	newPieces := make([]int, 0)
	for _, id := range territory.Pieces {
		if id != pieceID {
			newPieces = append(newPieces, id)
		}
	}
	territory.Pieces = newPieces

	// Remove from game
	delete(gc.Game.Pieces, pieceID)
}

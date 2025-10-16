package models

import "fmt"

// AdvancePhase moves to the next phase of the turn
func (g *Game) AdvancePhase() error {
	switch g.CurrentPhase {
	case PurchasePhase:
		g.CurrentPhase = CombatMovePhase
	case CombatMovePhase:
		g.CurrentPhase = ConductCombatPhase
	case ConductCombatPhase:
		g.CurrentPhase = NoncombatMovePhase
	case NoncombatMovePhase:
		g.CurrentPhase = MobilizePhase
	case MobilizePhase:
		g.CurrentPhase = CollectIncomePhase
	case CollectIncomePhase:
		// End of turn - advance to next player
		return g.AdvanceToNextPlayer()
	default:
		return fmt.Errorf("unknown phase: %v", g.CurrentPhase)
	}
	return nil
}

// AdvanceToNextPlayer moves to the next player's turn
func (g *Game) AdvanceToNextPlayer() error {
	if len(g.PlayerOrder) == 0 {
		return fmt.Errorf("no players in game")
	}

	// Find current player index
	currentIndex := -1
	for i, playerName := range g.PlayerOrder {
		if playerName == g.CurrentPower {
			currentIndex = i
			break
		}
	}

	// Move to next player
	nextIndex := (currentIndex + 1) % len(g.PlayerOrder)
	g.CurrentPower = g.PlayerOrder[nextIndex]

	// If we wrapped around to the first player, increment turn number
	if nextIndex == 0 {
		g.Turn++
	}

	// Reset to Purchase phase
	g.CurrentPhase = PurchasePhase

	return nil
}

// StartGame initializes the game state for the first turn
func (g *Game) StartGame() error {
	if len(g.PlayerOrder) == 0 {
		return fmt.Errorf("no players in game")
	}

	g.CurrentPower = g.PlayerOrder[0]
	g.CurrentPhase = PurchasePhase
	g.Turn = 1

	// Initialize purchased units map for all players
	for playerName := range g.Players {
		g.PurchasedUnits[playerName] = make([]*PendingUnit, 0)
	}

	return nil
}

// PurchaseUnit adds a unit to the player's pending purchases
func (g *Game) PurchaseUnit(playerName, unitType string) error {
	if g.CurrentPhase != PurchasePhase {
		return fmt.Errorf("can only purchase units in Purchase phase, currently in %v", g.CurrentPhase)
	}

	if playerName != g.CurrentPower {
		return fmt.Errorf("only current power (%s) can purchase units, not %s", g.CurrentPower, playerName)
	}

	player, exists := g.Players[playerName]
	if !exists {
		return fmt.Errorf("player %s not found", playerName)
	}

	template, exists := g.GlobalPieceTemplates[unitType]
	if !exists {
		return fmt.Errorf("unit type %s not found", unitType)
	}

	cost := int(template.Cost)

	// Check if player has enough IPCs
	if player.IPCs < cost {
		return fmt.Errorf("insufficient IPCs: need %d, have %d", cost, player.IPCs)
	}

	// Deduct IPCs
	player.IPCs -= cost

	// Add to pending units
	pendingUnit := &PendingUnit{
		Type: unitType,
		Cost: cost,
	}

	g.PurchasedUnits[playerName] = append(g.PurchasedUnits[playerName], pendingUnit)

	return nil
}

// MobilizeUnits places purchased units on the board
func (g *Game) MobilizeUnit(playerName, unitType, territoryName string) error {
	if g.CurrentPhase != MobilizePhase {
		return fmt.Errorf("can only mobilize units in Mobilize phase, currently in %v", g.CurrentPhase)
	}

	if playerName != g.CurrentPower {
		return fmt.Errorf("only current power (%s) can mobilize units, not %s", g.CurrentPower, playerName)
	}

	territory, exists := g.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	player, exists := g.Players[playerName]
	if !exists {
		return fmt.Errorf("player %s not found", playerName)
	}

	// Check that territory has an industrial complex
	hasIC := false
	for _, pieceID := range territory.Pieces {
		piece := g.Pieces[pieceID]
		if piece.Name == "industrial_complex" {
			hasIC = true
			break
		}
	}

	if !hasIC {
		return fmt.Errorf("territory %s has no industrial complex", territoryName)
	}

	// Check that player controls the territory (since start of turn)
	if territory.Owner != player {
		return fmt.Errorf("territory %s not controlled by %s", territoryName, playerName)
	}

	// Check that unit is in pending purchases
	pendingUnits := g.PurchasedUnits[playerName]
	found := false
	foundIndex := -1

	for i, pu := range pendingUnits {
		if pu.Type == unitType {
			found = true
			foundIndex = i
			break
		}
	}

	if !found {
		return fmt.Errorf("no pending %s unit to mobilize", unitType)
	}

	// Remove from pending units
	g.PurchasedUnits[playerName] = append(pendingUnits[:foundIndex], pendingUnits[foundIndex+1:]...)

	// Place the unit on the board
	err := g.PlacePieces(territoryName, unitType, 1)
	if err != nil {
		// Refund the unit if placement fails
		g.PurchasedUnits[playerName] = append(g.PurchasedUnits[playerName], pendingUnits[foundIndex])
		return fmt.Errorf("failed to place unit: %v", err)
	}

	return nil
}

// CollectIncome adds income to the current player based on territories controlled
func (g *Game) CollectIncome() error {
	if g.CurrentPhase != CollectIncomePhase {
		return fmt.Errorf("can only collect income in Collect Income phase, currently in %v", g.CurrentPhase)
	}

	player, exists := g.Players[g.CurrentPower]
	if !exists {
		return fmt.Errorf("current power %s not found", g.CurrentPower)
	}

	// Calculate income from territories
	income := 0
	for _, territory := range g.Board {
		if territory.Owner == player {
			income += territory.Production
		}
	}

	// Add income to player's IPCs
	player.IPCs += income

	return nil
}

// GetCurrentPlayer returns the player whose turn it is
func (g *Game) GetCurrentPlayer() (*Player, error) {
	player, exists := g.Players[g.CurrentPower]
	if !exists {
		return nil, fmt.Errorf("current power %s not found", g.CurrentPower)
	}
	return player, nil
}

// CanPurchase checks if the current player can afford a unit
func (g *Game) CanPurchase(unitType string) (bool, int, error) {
	if g.CurrentPhase != PurchasePhase {
		return false, 0, fmt.Errorf("not in Purchase phase")
	}

	player, err := g.GetCurrentPlayer()
	if err != nil {
		return false, 0, err
	}

	template, exists := g.GlobalPieceTemplates[unitType]
	if !exists {
		return false, 0, fmt.Errorf("unit type %s not found", unitType)
	}

	cost := int(template.Cost)
	return player.IPCs >= cost, cost, nil
}

// CaptureCapital handles capturing an enemy capital and stealing treasury
func (g *Game) CaptureCapital(capitalName, conquerorName string) error {
	territory, exists := g.Board[capitalName]
	if !exists {
		return fmt.Errorf("territory %s not found", capitalName)
	}

	conqueror, exists := g.Players[conquerorName]
	if !exists {
		return fmt.Errorf("player %s not found", conquerorName)
	}

	// Find the player whose capital was captured
	var capturedPlayer *Player
	for _, player := range g.Players {
		if player.Capital == capitalName {
			capturedPlayer = player
			break
		}
	}

	if capturedPlayer == nil {
		return fmt.Errorf("no player has %s as their capital", capitalName)
	}

	// Transfer all IPCs from captured player to conqueror
	conqueror.IPCs += capturedPlayer.IPCs
	capturedPlayer.IPCs = 0

	// Change territory ownership
	// Remove from old owner's territories
	for i, t := range capturedPlayer.Territories {
		if t == territory {
			capturedPlayer.Territories = append(capturedPlayer.Territories[:i], capturedPlayer.Territories[i+1:]...)
			break
		}
	}

	// Add to new owner
	territory.Owner = conqueror
	conqueror.Territories = append(conqueror.Territories, territory)

	return nil
}

// CountVictoryCities returns the number of victory cities controlled by each side
func (g *Game) CountVictoryCities() (axisCount, alliesCount int) {
	for _, territory := range g.Board {
		if territory.IsVictoryCity && territory.Owner != nil {
			if territory.Owner.Side == "Axis" {
				axisCount++
			} else if territory.Owner.Side == "Allies" {
				alliesCount++
			}
		}
	}
	return axisCount, alliesCount
}

// CheckVictoryConditions checks if any side has won the game
// Returns: (winner, hasWon, description)
func (g *Game) CheckVictoryConditions() (string, bool, string) {
	axisVC, alliesVC := g.CountVictoryCities()
	totalVC := axisVC + alliesVC

	// Total victory: control all victory cities
	if totalVC > 0 {
		if axisVC == totalVC {
			return "Axis", true, "Axis controls all victory cities (Total Victory)"
		}
		if alliesVC == totalVC {
			return "Allies", true, "Allies control all victory cities (Total Victory)"
		}
	}

	// Standard victory thresholds (for 1942 2nd Edition with 13 total VCs)
	// Axis needs 9, Allies need 10 (because Axis starts with slight disadvantage)
	if axisVC >= 9 {
		return "Axis", true, fmt.Sprintf("Axis controls %d victory cities (9+ required)", axisVC)
	}
	if alliesVC >= 10 {
		return "Allies", true, fmt.Sprintf("Allies control %d victory cities (10+ required)", alliesVC)
	}

	return "", false, fmt.Sprintf("Axis: %d VCs, Allies: %d VCs", axisVC, alliesVC)
}

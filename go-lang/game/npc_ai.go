package game

import (
	"boardgame/models"
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// NPCAIPlayer represents an AI player that can make decisions
type NPCAIPlayer struct {
	Name       string
	Difficulty string // "simple", "normal", "aggressive"
	rng        *rand.Rand
}

// NewNPCAIPlayer creates a new NPC AI player
func NewNPCAIPlayer(name string, difficulty string) *NPCAIPlayer {
	return &NPCAIPlayer{
		Name:       name,
		Difficulty: difficulty,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// TakeTurn executes a complete turn for an NPC player
func (npc *NPCAIPlayer) TakeTurn(controller *GameController, transcript *GameTranscript) error {
	player, err := controller.GetCurrentPlayer()
	if err != nil {
		return err
	}

	transcript.LogPhaseStart(player.Name, controller.Game.CurrentPhase)

	// Phase 1: Purchase
	err = npc.PurchasePhase(controller, transcript)
	if err != nil {
		return fmt.Errorf("purchase phase failed: %v", err)
	}
	controller.AdvancePhase()

	// Phase 2: Combat Move
	err = npc.CombatMovePhase(controller, transcript)
	if err != nil {
		return fmt.Errorf("combat move phase failed: %v", err)
	}
	controller.AdvancePhase()

	// Phase 3: Conduct Combat
	err = npc.ConductCombatPhase(controller, transcript)
	if err != nil {
		return fmt.Errorf("conduct combat phase failed: %v", err)
	}
	controller.AdvancePhase()

	// Phase 4: Noncombat Move
	err = npc.NoncombatMovePhase(controller, transcript)
	if err != nil {
		return fmt.Errorf("noncombat move phase failed: %v", err)
	}
	controller.AdvancePhase()

	// Phase 5: Mobilize
	err = npc.MobilizePhase(controller, transcript)
	if err != nil {
		return fmt.Errorf("mobilize phase failed: %v", err)
	}
	controller.AdvancePhase()

	// Phase 6: Collect Income
	err = npc.CollectIncomePhase(controller, transcript)
	if err != nil {
		return fmt.Errorf("collect income phase failed: %v", err)
	}
	controller.AdvancePhase() // This advances to next player

	return nil
}

// PurchasePhase decides what units to purchase
func (npc *NPCAIPlayer) PurchasePhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()
	game := controller.Game

	transcript.LogPhaseStart(player.Name, models.PurchasePhase)

	// Simple strategy: spend about 80% of IPCs on a balanced force
	budget := player.IPCs * 8 / 10
	spent := 0

	// Priority order: infantry (cheap), armor (strong), fighters (versatile)
	unitPriorities := []string{"infantry", "armor", "fighter"}

	purchases := make(map[string]int)

	for spent < budget {
		// Pick a unit type to buy
		for _, unitType := range unitPriorities {
			template, exists := game.GlobalPieceTemplates[unitType]
			if !exists {
				continue
			}

			cost := int(template.Cost)
			if spent+cost <= budget {
				err := controller.PurchaseUnit(unitType, 1)
				if err == nil {
					spent += cost
					purchases[unitType]++
				}
				break
			}
		}

		// If we can't afford anything from priority list, try to buy the cheapest unit
		if spent == player.IPCs*8/10 {
			break
		}

		// Prevent infinite loop
		if spent >= budget-2 {
			break
		}
	}

	if len(purchases) > 0 {
		transcript.LogPurchase(player.Name, purchases, spent)
	} else {
		transcript.LogAction(player.Name, "No purchases made")
	}

	return nil
}

// CombatMovePhase decides which units to move for combat
func (npc *NPCAIPlayer) CombatMovePhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()
	game := controller.Game

	transcript.LogPhaseStart(player.Name, models.CombatMovePhase)

	// Find enemy territories adjacent to our territories
	targets := npc.findAttackTargets(game, player)

	movesMade := 0
	for _, target := range targets {
		// Find our adjacent territories that can attack this target
		attackers := npc.findAttackersFor(game, player, target)

		// Move some units to attack (don't leave territories completely empty)
		for territoryName, pieces := range attackers {
			// Move up to half the pieces from this territory
			numToMove := len(pieces) / 2
			if numToMove == 0 && len(pieces) > 0 {
				numToMove = 1 // Move at least one if we have any
			}

			for i := 0; i < numToMove && i < len(pieces); i++ {
				pieceID := findPieceID(game, pieces[i])
				err := controller.PlanMove(pieceID, territoryName, target.Name)
				if err == nil {
					transcript.LogMove(player.Name, pieces[i].Name, territoryName, target.Name, "combat")
					movesMade++
				}
			}
		}

		// Limit number of attacks to keep things manageable
		if movesMade >= 5 {
			break
		}
	}

	// Execute all combat moves
	err := controller.ExecuteCombatMoves()
	if err != nil {
		return err
	}

	if movesMade == 0 {
		transcript.LogAction(player.Name, "No combat moves made")
	}

	return nil
}

// ConductCombatPhase resolves all battles
func (npc *NPCAIPlayer) ConductCombatPhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()

	transcript.LogPhaseStart(player.Name, models.ConductCombatPhase)

	roller := NewDiceRoller()
	battlesResolved := 0

	// Resolve all pending battles
	for territoryName := range controller.PendingBattles {
		transcript.LogBattleStart(territoryName)

		result, err := controller.ResolveBattle(territoryName, roller)
		if err != nil {
			transcript.LogAction(player.Name, fmt.Sprintf("Battle in %s failed: %v", territoryName, err))
			continue
		}

		battlesResolved++
		transcript.LogBattleResult(territoryName, result)
	}

	if battlesResolved == 0 {
		transcript.LogAction(player.Name, "No battles to resolve")
	}

	return nil
}

// NoncombatMovePhase consolidates forces after combat
func (npc *NPCAIPlayer) NoncombatMovePhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()
	game := controller.Game

	transcript.LogPhaseStart(player.Name, models.NoncombatMovePhase)

	movesMade := 0

	// Simple strategy: move units from safe territories to border territories
	borderTerritories := npc.findBorderTerritories(game, player)
	safeTerritories := npc.findSafeTerritories(game, player)

	for _, safeTerritory := range safeTerritories {
		pieces := game.GetPiecesInTerritory(safeTerritory.Name)

		// Don't leave territories completely empty
		if len(pieces) <= 2 {
			continue
		}

		// Move half the pieces to a nearby border territory
		numToMove := len(pieces) / 3
		if numToMove > 3 {
			numToMove = 3 // Don't move too many at once
		}

		// Find nearest border territory
		for _, borderTerritory := range borderTerritories {
			// Check if connected
			connected := false
			for _, neighbor := range safeTerritory.ConnectedTo {
				if neighbor == borderTerritory {
					connected = true
					break
				}
			}

			if connected {
				// Move some pieces
				for i := 0; i < numToMove && i < len(pieces); i++ {
					pieceID := findPieceID(game, pieces[i])
					err := controller.PlanMove(pieceID, safeTerritory.Name, borderTerritory.Name)
					if err == nil {
						transcript.LogMove(player.Name, pieces[i].Name, safeTerritory.Name, borderTerritory.Name, "noncombat")
						movesMade++
					}
				}
				break // Found a target for this safe territory
			}
		}

		if movesMade >= 5 {
			break
		}
	}

	// Execute noncombat moves
	err := controller.ExecuteNoncombatMoves()
	if err != nil {
		return err
	}

	if movesMade == 0 {
		transcript.LogAction(player.Name, "No noncombat moves made")
	}

	return nil
}

// MobilizePhase places purchased units
func (npc *NPCAIPlayer) MobilizePhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()
	game := controller.Game

	transcript.LogPhaseStart(player.Name, models.MobilizePhase)

	// Find all territories with industrial complexes
	icTerritories := make([]*models.Territory, 0)
	for _, territory := range game.Board {
		if territory.Owner != player {
			continue
		}

		for _, pieceID := range territory.Pieces {
			piece := game.Pieces[pieceID]
			if piece.Name == "industrial_complex" {
				icTerritories = append(icTerritories, territory)
				break
			}
		}
	}

	if len(icTerritories) == 0 {
		transcript.LogAction(player.Name, "No industrial complexes to mobilize units")
		return nil
	}

	// Mobilize all purchased units at the first available IC (simple strategy)
	// In a more complex AI, would distribute based on strategic value
	unitsPlaced := 0
	for len(game.PurchasedUnits[player.Name]) > 0 {
		pending := game.PurchasedUnits[player.Name]
		if len(pending) == 0 {
			break
		}

		unitType := pending[0].Type

		// Try each IC territory
		placed := false
		for _, territory := range icTerritories {
			err := controller.MobilizeUnit(territory.Name, unitType)
			if err == nil {
				transcript.LogMobilize(player.Name, unitType, territory.Name)
				unitsPlaced++
				placed = true
				break
			}
		}

		if !placed {
			// Can't place this unit anywhere, skip it
			transcript.LogAction(player.Name, fmt.Sprintf("Could not place %s", unitType))
			break
		}
	}

	if unitsPlaced == 0 {
		transcript.LogAction(player.Name, "No units mobilized")
	}

	return nil
}

// CollectIncomePhase collects income
func (npc *NPCAIPlayer) CollectIncomePhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()

	transcript.LogPhaseStart(player.Name, models.CollectIncomePhase)

	ipcsBeforeCollection := player.IPCs
	err := controller.CollectIncome()
	if err != nil {
		return err
	}

	incomeCollected := player.IPCs - ipcsBeforeCollection
	transcript.LogIncomeCollection(player.Name, incomeCollected, player.IPCs)

	return nil
}

// findAttackTargets finds enemy territories that are adjacent to our territories
func (npc *NPCAIPlayer) findAttackTargets(game *models.Game, player *models.Player) []*models.Territory {
	targets := make([]*models.Territory, 0)
	seen := make(map[string]bool)

	for _, ourTerritory := range player.Territories {
		for _, neighbor := range ourTerritory.ConnectedTo {
			if neighbor.Owner != player && !seen[neighbor.Name] {
				// Enemy or neutral territory
				targets = append(targets, neighbor)
				seen[neighbor.Name] = true
			}
		}
	}

	// Sort by value (prefer higher production territories)
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Production > targets[j].Production
	})

	return targets
}

// findAttackersFor finds our pieces that can attack a target territory
func (npc *NPCAIPlayer) findAttackersFor(game *models.Game, player *models.Player, target *models.Territory) map[string][]*models.Piece {
	attackers := make(map[string][]*models.Piece)

	for _, ourTerritory := range player.Territories {
		// Check if this territory is adjacent to target
		adjacent := false
		for _, neighbor := range ourTerritory.ConnectedTo {
			if neighbor == target {
				adjacent = true
				break
			}
		}

		if adjacent {
			pieces := game.GetPiecesInTerritory(ourTerritory.Name)
			// Filter out immobile pieces and industrial complexes
			attackingPieces := make([]*models.Piece, 0)
			for _, piece := range pieces {
				if piece.Movement > 0 && piece.Name != "industrial_complex" && piece.Name != "AAA" {
					attackingPieces = append(attackingPieces, piece)
				}
			}

			if len(attackingPieces) > 0 {
				attackers[ourTerritory.Name] = attackingPieces
			}
		}
	}

	return attackers
}

// findBorderTerritories finds our territories adjacent to enemy territories
func (npc *NPCAIPlayer) findBorderTerritories(game *models.Game, player *models.Player) []*models.Territory {
	borders := make([]*models.Territory, 0)

	for _, territory := range player.Territories {
		isBorder := false
		for _, neighbor := range territory.ConnectedTo {
			if neighbor.Owner != player {
				isBorder = true
				break
			}
		}

		if isBorder {
			borders = append(borders, territory)
		}
	}

	return borders
}

// findSafeTerritories finds our territories not adjacent to enemy territories
func (npc *NPCAIPlayer) findSafeTerritories(game *models.Game, player *models.Player) []*models.Territory {
	safe := make([]*models.Territory, 0)

	for _, territory := range player.Territories {
		isSafe := true
		for _, neighbor := range territory.ConnectedTo {
			if neighbor.Owner != player {
				isSafe = false
				break
			}
		}

		if isSafe {
			safe = append(safe, territory)
		}
	}

	return safe
}

// findPieceID finds the piece ID for a given piece in the game
func findPieceID(game *models.Game, piece *models.Piece) int {
	for id, p := range game.Pieces {
		if p == piece {
			return id
		}
	}
	return -1
}

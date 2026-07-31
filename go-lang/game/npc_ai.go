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

	// roller is the dice this player fights with. Combat used to build a fresh,
	// time-seeded roller at the point of use, which meant a whole game could
	// never be replayed -- and a bug found by running one was a bug you could
	// not reproduce.
	roller *DiceRoller
}

// NewNPCAIPlayer creates a new NPC AI player with unpredictable dice.
func NewNPCAIPlayer(name string, difficulty string) *NPCAIPlayer {
	return NewSeededNPCAIPlayer(name, difficulty, time.Now().UnixNano())
}

// NewSeededNPCAIPlayer creates an NPC whose decisions and dice follow from a
// seed, so a game can be replayed exactly.
func NewSeededNPCAIPlayer(name string, difficulty string, seed int64) *NPCAIPlayer {
	return &NPCAIPlayer{
		Name:       name,
		Difficulty: difficulty,
		rng:        rand.New(rand.NewSource(seed)),
		roller:     NewSeededDiceRoller(seed),
	}
}

// dice returns this player's roller, building one if the player was assembled
// without going through a constructor.
func (npc *NPCAIPlayer) dice() *DiceRoller {
	if npc.roller == nil {
		npc.roller = NewDiceRoller()
	}
	return npc.roller
}

// TakeTurn executes a complete turn for an NPC player
func (npc *NPCAIPlayer) TakeTurn(controller *GameController, transcript *GameTranscript) error {
	player, err := controller.GetCurrentPlayer()
	if err != nil {
		return err
	}

	transcript.LogPhaseStart(player.Name, controller.Game.CurrentPhase)

	// Bring standing plans up to date before deciding anything. This is what
	// makes the computer players non-stateless: a plan formed several turns ago
	// tells this turn what to buy, where to march, and when to sail.
	npc.ReviewPlans(controller, player, transcript)
	npc.ReviewNaval(controller, player, transcript)
	npc.ReviewDefences(controller, player, transcript)

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

	// Spend about 80% of the treasury, divided by this power's temperament.
	//
	// The split is what makes the computer players differ from one another:
	// Germany presses and keeps a thin garrison, Italy garrisons and rarely
	// sails, the United States and the Soviet Union build the largest forces.
	budget := player.IPCs * 8 / 10
	posture := PostureFor(player.Name)
	defenceBudget, _, offenceBudget := posture.Budget(budget)
	spent := 0

	purchases := make(map[string]int)

	// Check if we should buy a factory first
	// Only consider if we have enough IPCs and a good territory without a factory
	// Find whatever this board calls its factory, rather than guessing at names.
	// The old code looked up "factory" then "industrial_complex"; a board using
	// any other spelling meant the AI could never build one.
	factoryName, factoryTemplate, hasFactory := findStructureTemplate(game)

	if hasFactory && player.IPCs >= int(factoryTemplate.Cost)+20 {
		// Find high-value territories without factories
		bestTerritory := npc.findBestTerritoryForFactory(game, player)
		if bestTerritory != nil && bestTerritory.Production >= 3 {
			// Buy one factory if we can afford it and still have money for units
			err := controller.PurchaseUnit(factoryName, 1)
			if err == nil {
				spent += int(factoryTemplate.Cost)
				purchases[factoryName]++
			}
		}
	}

	// Target share of the budget for each unit type.
	//
	// This used to be a plain priority list walked from the top, and the top
	// entry was the cheapest unit. Infantry is always affordable, so the loop
	// bought infantry, broke, and started again at infantry -- for the whole
	// game. A full game produced 1,075 infantry, zero armour and zero aircraft,
	// while the code claimed to build a balanced force.
	//
	// Spending to a target share instead means each type is bought when it is
	// furthest behind its share, so the mix holds at any income. The roles are
	// picked from the board's roster rather than hardcoded names, so a board
	// that spells its units differently still gets an army.
	unitMix := buildupMix(game)
	spentOn := make(map[string]int)
	unaffordable := make(map[string]bool)

	// Garrisons first: they are cheap, they are what a defensive power exists
	// to buy, and an undefended factory loses the war quietly.
	defenceSpent := 0
	defenceWants := npc.DefencePurchases(controller, player)
	for _, unitType := range sortedWants(defenceWants) {
		count := defenceWants[unitType]
		template, exists := game.GlobalPieceTemplates[unitType]
		if !exists {
			continue
		}
		for i := 0; i < count; i++ {
			cost := int(template.Cost)
			if spent+cost > budget || defenceSpent+cost > defenceBudget {
				break
			}
			if err := controller.PurchaseUnit(unitType, 1); err != nil {
				break
			}
			spent += cost
			defenceSpent += cost
			spentOn[unitType] += cost
			purchases[unitType]++
		}
	}

	// Expeditionary work takes its own share, and keeps what it cannot spend.
	//
	// An invasion short of shipping stays short forever if production ignores
	// it; one given the whole budget builds a fleet and no army to land. But a
	// share alone is not enough either, because a share smaller than the cheapest
	// ship never buys one -- so the unspent part is carried to next turn instead
	// of falling through to the general buildup.
	wants := npc.PlanPurchases(controller, player)
	planBudget := offenceBudget + controller.Plans.Reserve(player.Name)
	planSpent := 0

	for _, unitType := range planPurchaseOrder(game, wants) {
		template, exists := game.GlobalPieceTemplates[unitType]
		if !exists {
			continue
		}
		for i := 0; i < wants[unitType]; i++ {
			cost := int(template.Cost)
			if spent+cost > budget || planSpent+cost > planBudget {
				break
			}
			if err := controller.PurchaseUnit(unitType, 1); err != nil {
				break
			}
			spent += cost
			planSpent += cost
			spentOn[unitType] += cost
			purchases[unitType]++
		}
	}

	// Save only against something actually wanted, and only money that is still
	// in the treasury: a power with no operation under way puts everything into
	// the army rather than hoarding for a plan it does not have.
	held := planBudget - planSpent
	if outstanding := wantedCost(game, wants) - planSpent; held > outstanding {
		held = outstanding
	}
	if held > budget-spent {
		held = budget - spent
	}
	if held < 0 {
		held = 0
	}
	controller.Plans.SetReserve(player.Name, held)

	for spent < budget-held {
		// Pick a unit type to buy.
		//
		// A failed purchase must end the loop, not be swallowed. The previous
		// version discarded the error and broke out of the inner loop either
		// way, leaving `spent` unchanged -- so if PurchaseUnit refused for a
		// reason unrelated to cost (chiefly being called outside the Purchase
		// phase, which the web server does) the outer loop span forever and took
		// the request with it.
		// Choose the affordable type that is furthest behind its target share.
		bought := false
		best, bestDeficit := "", 0.0
		for _, entry := range unitMix {
			template, exists := game.GlobalPieceTemplates[entry.name]
			if !exists || unaffordable[entry.name] {
				continue
			}
			cost := int(template.Cost)
			if spent+cost > budget-held {
				continue
			}

			deficit := entry.share*float64(budget) - float64(spentOn[entry.name])
			if best == "" || deficit > bestDeficit {
				best, bestDeficit = entry.name, deficit
			}
		}

		if best != "" {
			cost := int(game.GlobalPieceTemplates[best].Cost)
			if err := controller.PurchaseUnit(best, 1); err != nil {
				// Not a budget problem: stop trying this unit type rather than
				// asking again with the same arguments and the same answer.
				unaffordable[best] = true
			} else {
				spent += cost
				spentOn[best] += cost
				purchases[best]++
				bought = true
			}
		}

		if !bought {
			break // nothing left that we can afford or are allowed to buy
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

	// Evaluate each potential attack
	movesMade := 0
	attacksPlanned := 0
	for _, target := range targets {
		// Find our adjacent territories that can attack this target
		attackers := npc.findAttackersFor(controller, player, target)

		// Collect all available attacking pieces
		var allAttackers []*models.Piece
		for _, pieces := range attackers {
			allAttackers = append(allAttackers, pieces...)
		}

		if len(allAttackers) == 0 {
			continue
		}

		// Get defenders
		defenders := game.GetPiecesInTerritory(target.Name)

		// Estimate success probability
		successProb := EstimateAttackSuccess(allAttackers, defenders)

		// Calculate territory value
		territoryValue := CalculateTerritoryValue(target, target.IsVictoryCity)

		// Decision logic based on difficulty and situation
		shouldAttack := false
		minProbability := 0.6 // Default: need 60% success chance

		// Adjust threshold based on difficulty
		if npc.Difficulty == "aggressive" {
			minProbability = 0.4 // More willing to take risks
		} else if npc.Difficulty == "simple" {
			minProbability = 0.7 // More conservative
		}

		// High-value targets (victory cities, high production) are worth more risk
		if territoryValue >= 5 {
			minProbability -= 0.15
		}

		// If we're losing badly, be more desperate
		playerTerritoryCount := len(player.Territories)
		if playerTerritoryCount < 5 {
			minProbability -= 0.2 // More desperate when losing
		}

		shouldAttack = successProb >= minProbability

		if !shouldAttack {
			continue // Skip this target
		}

		// Move units to attack (don't leave territories completely empty or
		// vulnerable). Source territories are visited in a fixed order: map
		// iteration order varies between runs, which broke seeded replay.
		sourceNames := make([]string, 0, len(attackers))
		for territoryName := range attackers {
			sourceNames = append(sourceNames, territoryName)
		}
		sort.Strings(sourceNames)

		for _, territoryName := range sourceNames {
			pieces := attackers[territoryName]
			sourceTerritory := game.Board[territoryName]

			// Calculate how many to move based on success probability
			percentToMove := 0.5 // Default: move half
			if successProb < 0.7 {
				percentToMove = 0.7 // Move more if uncertain
			}
			if successProb > 0.85 {
				percentToMove = 0.4 // Move less if very confident
			}

			numToMove := int(float64(len(pieces)) * percentToMove)
			if numToMove == 0 && len(pieces) > 0 && successProb >= 0.5 {
				numToMove = 1 // Move at least one if viable
			}

			// IMPORTANT: Check if moving these units would leave the source territory vulnerable
			if npc.wouldLeaveTerritoryVulnerable(game, sourceTerritory, numToMove, player) {
				// Reduce the number of units to move
				numToMove = numToMove / 2
				if numToMove == 0 {
					continue // Don't move any units from this vulnerable territory
				}
			}

			// Extra caution for our own victory cities - never leave them undefended
			if sourceTerritory.IsVictoryCity {
				// Keep at least 3 units in victory cities
				if len(pieces)-numToMove < 3 {
					numToMove = len(pieces) - 3
					if numToMove < 0 {
						numToMove = 0
					}
				}
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

		attacksPlanned++

		// Limit number of attacks to keep things manageable
		if attacksPlanned >= 5 {
			break
		}
	}

	// Launch any plan whose force is assembled. Loading, sailing and landing
	// all happen in this phase, so an operation that has been forming for
	// several turns executes here in one go.
	launched := npc.ExecuteReadyPlans(controller, player, transcript)

	// A convoy held up by a stationed fleet fights its way past, provided it
	// has the cover to do so.
	npc.ForceConvoysThrough(controller, player, transcript)

	// Execute all combat moves
	err := controller.ExecuteCombatMoves()
	if err != nil {
		return err
	}

	// The convoys have arrived; put the troops ashore. This has to follow the
	// move, not precede it, or the transports would still be at the port.
	if launched > 0 {
		controller.LandAssaultTroops(player.Name, transcript)
	}

	if movesMade == 0 && launched == 0 {
		transcript.LogAction(player.Name, "No combat moves made")
	}

	return nil
}

// ConductCombatPhase resolves all battles
func (npc *NPCAIPlayer) ConductCombatPhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()

	transcript.LogPhaseStart(player.Name, models.ConductCombatPhase)

	roller := npc.dice()
	battlesResolved := 0

	// Resolve battles in a fixed order. Ranging the map resolved them in
	// whatever order Go's map iteration produced, and with one shared dice
	// roller that meant each battle consumed different rolls on different runs
	// -- so the same seed produced different games, and "replay with
	// GAME_SEED=N" was a promise the code did not keep.
	pending := make([]string, 0, len(controller.PendingBattles))
	for territoryName := range controller.PendingBattles {
		pending = append(pending, territoryName)
	}
	sort.Strings(pending)

	for _, territoryName := range pending {
		transcript.LogBattleStart(territoryName)

		// Create retreat decider based on NPC difficulty
		retreatDecider := func(initialAttackers, currentAttackers, initialDefenders, currentDefenders, round int) bool {
			// Use the retreat evaluation logic
			shouldRetreat := ShouldAttackerRetreat(initialAttackers, currentAttackers, initialDefenders, currentDefenders, round)

			// Adjust based on difficulty
			if npc.Difficulty == "aggressive" {
				// Aggressive NPCs are less likely to retreat
				// Only retreat if losses are extreme (>80% casualties and still losing)
				attackerLossRatio := float64(initialAttackers-currentAttackers) / float64(initialAttackers)
				return attackerLossRatio > 0.8 && currentDefenders > currentAttackers
			} else if npc.Difficulty == "simple" {
				// Simple NPCs retreat more readily
				return shouldRetreat
			}

			// Normal difficulty
			return shouldRetreat
		}

		result, err := controller.ResolveBattleWithRetreat(territoryName, roller, retreatDecider)
		if err != nil {
			transcript.LogAction(player.Name, fmt.Sprintf("Battle in %s failed: %v", territoryName, err))
			continue
		}

		battlesResolved++

		if result.AttackerRetreated {
			transcript.LogAction(player.Name, fmt.Sprintf("Retreated from battle in %s", territoryName))
		}

		transcript.LogBattleResult(territoryName, result)
	}

	if battlesResolved == 0 {
		transcript.LogAction(player.Name, "No battles to resolve")
	}

	return nil
}

// NoncombatMovePhase consolidates forces after combat

// uncommittedPieces returns the units in a territory that are free to be moved
// by the ordinary movement logic.
//
// Units reserved by a standing plan are held back. They are massing for an
// operation, and general movement would otherwise walk them off the quayside
// again every turn -- which it did, so no invasion force ever assembled.
func (npc *NPCAIPlayer) uncommittedPieces(controller *GameController, player *models.Player, territoryName string) []*models.Piece {
	all := controller.Game.GetPiecesInTerritory(territoryName)
	free := make([]*models.Piece, 0, len(all))
	for _, piece := range all {
		if controller.Plans.Committed(player.Name, piece.ID) {
			continue
		}
		free = append(free, piece)
	}
	return free
}

func (npc *NPCAIPlayer) NoncombatMovePhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()
	game := controller.Game

	transcript.LogPhaseStart(player.Name, models.NoncombatMovePhase)

	// Gathering comes first: units committed to a plan walk to their port and
	// shipping sails to meet them. Doing this before general movement stops the
	// ordinary logic scattering an invasion force that has been assembling for
	// several turns.
	movesMade := npc.GatherForPlans(controller, player, transcript)
	movesMade += npc.SailNavalPlans(controller, player, transcript)

	// Identify strategic targets (victory cities)
	strategicTargets := npc.identifyStrategicTargets(game, player)

	// Find territories that need reinforcement
	threatenedTerritories := make([]*models.Territory, 0)
	for _, territory := range player.Territories {
		threat := npc.evaluateTerritoryThreat(game, territory, player)
		if threat > 0 {
			// High-value or threatened territories need reinforcement
			if territory.IsVictoryCity || territory.Production >= 3 || threat > 5 {
				threatenedTerritories = append(threatenedTerritories, territory)
			}
		}
	}

	// Strategy:
	// 1. First priority: defend threatened high-value territories
	// 2. Second priority: position units to attack victory cities
	// 3. Third priority: general border consolidation

	safeTerritories := npc.findSafeTerritories(game, player)

	// PRIORITY 1: Reinforce threatened territories
	for _, threatened := range threatenedTerritories {
		if movesMade >= 8 {
			break
		}

		for _, safe := range safeTerritories {
			pieces := npc.uncommittedPieces(controller, player, safe.Name)

			if len(pieces) <= 2 {
				continue // Don't leave empty
			}

			// Calculate safe number to move
			numToMove := len(pieces) / 3
			if numToMove > 2 {
				numToMove = 2
			}

			// Check if we'd leave this territory vulnerable
			if npc.wouldLeaveTerritoryVulnerable(game, safe, numToMove, player) {
				continue // Don't weaken this territory
			}

			// Try to move units to threatened territory
			moved := 0
			for i := 0; i < numToMove && i < len(pieces); i++ {
				pieceID := findPieceID(game, pieces[i])
				err := controller.PlanMove(pieceID, safe.Name, threatened.Name)
				if err == nil {
					transcript.LogMove(player.Name, pieces[i].Name, safe.Name, threatened.Name, "noncombat")
					movesMade++
					moved++
				}
			}

			if moved > 0 {
				break // Reinforced this threatened territory
			}
		}
	}

	// PRIORITY 2: Position units near victory cities for future attacks
	if len(strategicTargets) > 0 && movesMade < 10 {
		// For each strategic target, find staging territories (adjacent friendly territories)
		for _, target := range strategicTargets {
			if movesMade >= 10 {
				break
			}

			// Find friendly territories adjacent to the target
			stagingTerritories := make([]*models.Territory, 0)
			for _, neighbor := range target.ConnectedTo {
				if neighbor.Owner == player {
					stagingTerritories = append(stagingTerritories, neighbor)
				}
			}

			if len(stagingTerritories) == 0 {
				continue // Can't stage near this target
			}

			// Move units toward staging territories
			for _, safe := range safeTerritories {
				if movesMade >= 10 {
					break
				}

				pieces := npc.uncommittedPieces(controller, player, safe.Name)

				if len(pieces) <= 2 {
					continue
				}

				// Find nearest staging territory
				nearestStaging := stagingTerritories[0]
				// Try to move units there
				numToMove := len(pieces) / 4 // Move fewer when positioning
				if numToMove < 1 && len(pieces) > 3 {
					numToMove = 1
				}

				// Check defensive vulnerability
				if npc.wouldLeaveTerritoryVulnerable(game, safe, numToMove, player) {
					continue
				}

				for i := 0; i < numToMove && i < len(pieces); i++ {
					pieceID := findPieceID(game, pieces[i])
					err := controller.PlanMove(pieceID, safe.Name, nearestStaging.Name)
					if err == nil {
						transcript.LogMove(player.Name, pieces[i].Name, safe.Name, nearestStaging.Name, "noncombat")
						movesMade++
					}
				}
			}
		}
	}

	// PRIORITY 3: General border consolidation (if we haven't moved many units yet)
	if movesMade < 5 {
		borderTerritories := npc.findBorderTerritories(game, player)

		for _, safeTerritory := range safeTerritories {
			if movesMade >= 8 {
				break
			}

			pieces := npc.uncommittedPieces(controller, player, safeTerritory.Name)

			if len(pieces) <= 2 {
				continue
			}

			numToMove := len(pieces) / 3
			if numToMove > 2 {
				numToMove = 2
			}

			// Check defensive vulnerability
			if npc.wouldLeaveTerritoryVulnerable(game, safeTerritory, numToMove, player) {
				continue
			}

			// Find nearest border territory
			for _, borderTerritory := range borderTerritories {
				moved := 0
				for i := 0; i < numToMove && i < len(pieces); i++ {
					pieceID := findPieceID(game, pieces[i])
					err := controller.PlanMove(pieceID, safeTerritory.Name, borderTerritory.Name)
					if err == nil {
						transcript.LogMove(player.Name, pieces[i].Name, safeTerritory.Name, borderTerritory.Name, "noncombat")
						movesMade++
						moved++
					}
				}

				if moved > 0 {
					break
				}
			}
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

	// Find all territories with industrial complexes (called "factory" in
	// aaa.gdf), in a fixed order -- ranging the board map placed new units at a
	// different factory on every run, which broke seeded replay.
	icTerritories := make([]*models.Territory, 0)
	for _, territory := range sortedTerritories(player) {
		for _, pieceID := range territory.Pieces {
			piece := game.Pieces[pieceID]
			if game.Units().For(piece).IsStructure {
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

		// Land units appear at the factory; ships are launched into a sea zone
		// beside it.
		spots := make([]string, 0, len(icTerritories))
		if template, ok := game.GlobalPieceTemplates[unitType]; ok && template.Terrain == models.Water {
			for _, territory := range icTerritories {
				spots = append(spots, adjacentSeaZones(game, territory.Name)...)
			}
		} else {
			for _, territory := range icTerritories {
				spots = append(spots, territory.Name)
			}
		}

		placed := false
		for _, spot := range spots {
			err := controller.MobilizeUnit(spot, unitType)
			if err == nil {
				transcript.LogMobilize(player.Name, unitType, spot)
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
// Prioritizes victory cities and high-value territories
func (npc *NPCAIPlayer) findAttackTargets(game *models.Game, player *models.Player) []*models.Territory {
	type targetScore struct {
		territory *models.Territory
		score     int
	}

	targetScores := make([]targetScore, 0)
	seen := make(map[string]bool)

	for _, ourTerritory := range player.Territories {
		for _, neighbor := range ourTerritory.ConnectedTo {
			if neighbor.Owner != player && !seen[neighbor.Name] {
				// Rulebook page 14: "At no time can an Allied power attack another Allied power,
				// or an Axis power attack another Axis power"
				// Skip allied territories - only target true enemies or neutrals
				if areAllies(player, neighbor.Owner) {
					continue // Skip allies
				}

				// Policy, not rule: the AI does not violate strict neutrals.
				// The rules allow it -- 3 IPCs to the bank, the country raises
				// a garrison, and every other strict neutral turns hostile --
				// but that diplomatic price is invisible to this scoring, so a
				// dumb attacker would hand the whole neutral bloc to its enemy
				// for a 4-IPC province.
				if neighbor.Owner.Name == "Neutral" && neighbor.NeutralType == models.StrictNeutral {
					continue
				}

				// Calculate strategic score
				score := neighbor.Production

				// Victory cities are MUCH more valuable
				if neighbor.IsVictoryCity {
					score += 15 // Massive bonus for victory cities
				}

				// Bonus for territories that connect to more of our territories (easier to attack/defend)
				connectivityBonus := 0
				for _, connectedTo := range neighbor.ConnectedTo {
					if connectedTo.Owner == player {
						connectivityBonus++
					}
				}
				score += connectivityBonus

				// Penalty for heavily defended territories (we'll still consider them but lower priority)
				defenders := game.GetPiecesInTerritory(neighbor.Name)
				defenseStrength := 0
				for _, piece := range defenders {
					defenseStrength += int(piece.Defend)
				}
				// Reduce score slightly for heavily defended territories
				if defenseStrength > 10 {
					score -= 2
				}

				targetScores = append(targetScores, targetScore{
					territory: neighbor,
					score:     score,
				})
				seen[neighbor.Name] = true
			}
		}
	}

	// Sort by score (highest first)
	sort.Slice(targetScores, func(i, j int) bool {
		return targetScores[i].score > targetScores[j].score
	})

	// Extract territories in priority order
	targets := make([]*models.Territory, len(targetScores))
	for i, ts := range targetScores {
		targets[i] = ts.territory
	}

	return targets
}

// findAttackersFor finds our pieces that can attack a target territory
func (npc *NPCAIPlayer) findAttackersFor(controller *GameController, player *models.Player, target *models.Territory) map[string][]*models.Piece {
	game := controller.Game
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
				caps := game.Units().For(piece)
				// Units committed to a standing plan are left alone. Without
				// this the ordinary movement logic walks an invasion force back
				// off the quayside every turn, and the plan never assembles.
				if controller.Plans.Committed(player.Name, piece.ID) {
					continue
				}
				if piece.Movement > 0 && !caps.IsStructure && !caps.IsAA {
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
	if piece == nil {
		return -1
	}
	// Pieces carry their own ID, so this no longer walks the whole board by
	// pointer identity to answer a question the piece already knows.
	return piece.ID
}

// findBestTerritoryForFactory finds the best territory to build a new factory
// Returns nil if no suitable territory is found
func (npc *NPCAIPlayer) findBestTerritoryForFactory(game *models.Game, player *models.Player) *models.Territory {
	var bestTerritory *models.Territory
	highestValue := 0

	for _, territory := range player.Territories {
		// Territory must have production value of at least 1 (per rulebook page 21)
		if territory.Production < 1 {
			continue
		}

		// Check if territory already has a factory
		hasFactory := false
		for _, pieceID := range territory.Pieces {
			piece := game.Pieces[pieceID]
			if game.Units().For(piece).IsStructure {
				hasFactory = true
				break
			}
		}

		if hasFactory {
			continue
		}

		// Prefer territories with higher production values
		if territory.Production > highestValue {
			highestValue = territory.Production
			bestTerritory = territory
		}
	}

	return bestTerritory
}

// findNearestVictoryCity finds the nearest enemy or neutral victory city
// Returns the territory and distance, or nil if none found
func (npc *NPCAIPlayer) findNearestVictoryCity(game *models.Game, fromTerritory *models.Territory, player *models.Player) (*models.Territory, int) {
	// BFS to find nearest victory city
	type node struct {
		territory *models.Territory
		distance  int
	}

	visited := make(map[string]bool)
	queue := []node{{fromTerritory, 0}}
	visited[fromTerritory.Name] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Check if this is an enemy victory city
		if current.territory.IsVictoryCity && current.territory.Owner != player && current.distance > 0 {
			return current.territory, current.distance
		}

		// Explore neighbors
		for _, neighbor := range current.territory.ConnectedTo {
			if !visited[neighbor.Name] {
				visited[neighbor.Name] = true
				queue = append(queue, node{neighbor, current.distance + 1})
			}
		}
	}

	return nil, 0
}

// evaluateTerritoryThreat evaluates if a territory is under threat from enemy forces
// Returns a threat score (0 = safe, higher = more threatened)
func (npc *NPCAIPlayer) evaluateTerritoryThreat(game *models.Game, territory *models.Territory, player *models.Player) int {
	threatScore := 0
	friendlyPieces := game.GetPiecesInTerritory(territory.Name)

	// Check all adjacent territories for enemy forces
	for _, neighbor := range territory.ConnectedTo {
		if neighbor.Owner == player || areAllies(player, neighbor.Owner) {
			continue // Not a threat
		}

		enemyPieces := game.GetPiecesInTerritory(neighbor.Name)

		// Count enemy mobile units (those that can attack)
		enemyAttackPower := 0
		for _, piece := range enemyPieces {
			if piece.Movement > 0 && !game.Units().For(piece).IsStructure {
				enemyAttackPower += int(piece.Attack)
			}
		}

		// Count friendly defense power
		friendlyDefensePower := 0
		for _, piece := range friendlyPieces {
			friendlyDefensePower += int(piece.Defend)
		}

		// If enemy has more attack power than we have defense, it's a threat
		if enemyAttackPower > friendlyDefensePower {
			threatScore += (enemyAttackPower - friendlyDefensePower)
		}
	}

	return threatScore
}

// wouldLeaveTerritoryVulnerable checks if removing units would make territory vulnerable
func (npc *NPCAIPlayer) wouldLeaveTerritoryVulnerable(game *models.Game, territory *models.Territory, numUnitsToRemove int, player *models.Player) bool {
	currentPieces := game.GetPiecesInTerritory(territory.Name)

	// If we'd be left with 0 or 1 unit, it's vulnerable
	if len(currentPieces)-numUnitsToRemove <= 1 {
		// Check if there are adjacent enemies
		for _, neighbor := range territory.ConnectedTo {
			if neighbor.Owner != player && !areAllies(player, neighbor.Owner) {
				enemyPieces := game.GetPiecesInTerritory(neighbor.Name)
				if len(enemyPieces) > 0 {
					return true // Vulnerable!
				}
			}
		}
	}

	// Calculate remaining defense power
	remainingDefense := 0
	for i := numUnitsToRemove; i < len(currentPieces); i++ {
		remainingDefense += int(currentPieces[i].Defend)
	}

	// Check threat from adjacent territories
	maxThreat := 0
	for _, neighbor := range territory.ConnectedTo {
		if neighbor.Owner == player || areAllies(player, neighbor.Owner) {
			continue
		}

		enemyPieces := game.GetPiecesInTerritory(neighbor.Name)
		enemyAttack := 0
		for _, piece := range enemyPieces {
			if piece.Movement > 0 && !game.Units().For(piece).IsStructure {
				enemyAttack += int(piece.Attack)
			}
		}

		if enemyAttack > maxThreat {
			maxThreat = enemyAttack
		}
	}

	// Vulnerable if remaining defense is less than 60% of max threat
	return remainingDefense < int(float64(maxThreat)*0.6)
}

// identifyStrategicTargets identifies high-value targets to focus on
func (npc *NPCAIPlayer) identifyStrategicTargets(game *models.Game, player *models.Player) []*models.Territory {
	targets := make([]*models.Territory, 0)

	// Find all enemy victory cities
	for _, territory := range game.Board {
		if territory.IsVictoryCity && territory.Owner != player && !areAllies(player, territory.Owner) {
			targets = append(targets, territory)
		}
	}

	// Sort by production value (higher first), then by name: the candidates come
	// out of a map, so without a total order the list differs between runs.
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Production != targets[j].Production {
			return targets[i].Production > targets[j].Production
		}
		return targets[i].Name < targets[j].Name
	})

	return targets
}


// unitShare is one role in the general buildup and its slice of the budget.
type unitShare struct {
	name  string
	share float64
}

// buildupMix picks the general army's composition from the board's roster:
// a line unit (best defence per IPC), a punch unit (best attack per IPC), and
// an air unit (best combat value per IPC). On aaa.gdf these come out as
// infantry, armor and fighter -- the names the mix used to hardcode, which
// bought nothing at all on a board that spelled them differently.
func buildupMix(g *models.Game) []unitShare {
	units := g.Units()

	pickLand := func(value func(*models.Piece) int) string {
		best, bestRatio := "", 0.0
		for _, name := range sortedTemplateNames(g) {
			template := g.GlobalPieceTemplates[name]
			caps := units.Of(name)
			if template.Terrain != models.Land || template.Cost <= 0 ||
				caps.IsStructure || caps.IsAA || template.Movement <= 0 {
				continue
			}
			if v := value(template); v > 0 {
				if ratio := float64(v) / float64(template.Cost); ratio > bestRatio {
					best, bestRatio = name, ratio
				}
			}
		}
		return best
	}

	line := pickLand(func(p *models.Piece) int { return int(p.Defend) })
	punch := pickLand(func(p *models.Piece) int { return int(p.Attack) })

	air, bestAir := "", 0.0
	for _, name := range sortedTemplateNames(g) {
		template := g.GlobalPieceTemplates[name]
		if template.Terrain != models.Air || template.Cost <= 0 {
			continue
		}
		if ratio := float64(template.Attack+template.Defend) / float64(template.Cost); ratio > bestAir {
			air, bestAir = name, ratio
		}
	}

	mix := make([]unitShare, 0, 3)
	add := func(name string, share float64) {
		if name == "" {
			return
		}
		for i := range mix {
			if mix[i].name == name {
				mix[i].share += share // roles collapsed onto one unit type
				return
			}
		}
		mix = append(mix, unitShare{name, share})
	}
	add(line, 0.50)
	add(punch, 0.30)
	add(air, 0.20)
	return mix
}

// findStructureTemplate returns the board's buildable structure -- its factory
// or industrial complex -- under whatever name the board gives it.
func findStructureTemplate(game *models.Game) (string, *models.Piece, bool) {
	registry := game.Units()
	for name, template := range game.GlobalPieceTemplates {
		if registry.Of(name).IsStructure {
			return name, template, true
		}
	}
	return "", nil, false
}

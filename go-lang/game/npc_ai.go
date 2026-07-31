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

	// attacksThisTurn is how many attacks and landings this turn's combat move
	// actually launched. The quartermaster reads it in the noncombat phase: an
	// outproduced power that found nothing worth hitting falls back to the
	// third rung of the ladder -- dig in, and hope to outlast an enemy who is
	// busy elsewhere. Valid only within a single TakeTurn.
	attacksThisTurn int
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

	// Spend about 80% of the treasury, divided by this power's temperament --
	// or nearly all of it when the clock is against us. The 20% cushion
	// accumulated into 30-50 idle IPCs by the end of observed games; a power
	// that needs to act now has no business sitting on a war chest.
	//
	// The split is what makes the computer players differ from one another:
	// Germany presses and keeps a thin garrison, Italy garrisons and rarely
	// sails, the United States and the Soviet Union build the largest forces.
	spendPercent := treasurySpendPercent
	if outproduced(strategicPressure(game, player)) {
		spendPercent = urgentSpendPercent
	}
	budget := player.IPCs * spendPercent / 100
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

	if hasFactory && player.IPCs >= int(factoryTemplate.Cost)+factoryCashCushion {
		// Find high-value territories without factories
		bestTerritory := npc.findBestTerritoryForFactory(game, player)
		if bestTerritory != nil && bestTerritory.Production >= factoryMinProduction {
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

	// The clock sets the tempo -- whichever of the production race and the
	// victory race is going worse for this side. Outproduced or behind on
	// cities, thinner odds are accepted now, because the same attack will
	// only be worse later; ahead on both, marginal fights are declined,
	// since patience will turn them into sure ones.
	pressure := strategicPressure(game, player)

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

		// Get defenders -- including the garrison a neutral would mobilise.
		defenders := expectedDefenders(game, target)

		// Estimate success probability
		successProb := EstimateAttackSuccess(allAttackers, defenders)

		// Calculate territory value
		territoryValue := CalculateTerritoryValue(target, target.IsVictoryCity)

		// The required odds: the difficulty's base, bent by what the target is
		// worth, how desperate we are, and the production race.
		minProbability := DoctrineFor(npc.Difficulty).MinOdds
		if territoryValue >= richTargetValue {
			minProbability -= richTargetDiscount
		}
		if len(player.Territories) < desperationTerritories {
			minProbability -= desperationDiscount
		}
		minProbability = pressureThreshold(minProbability, pressure)

		if successProb < minProbability {
			continue // Skip this target
		}

		// Choose the force first, then judge THAT force.
		//
		// The old flow approved the attack on the odds of every candidate in
		// every adjacent territory, then sent a fraction of them -- so battles
		// routinely went in at half the strength the estimate had priced, and
		// the observed attack record across six games was a coin flip (134
		// wins, 127 losses). Now the fraction-per-source and the vulnerability
		// limits pick the actual force, the estimate is recomputed on it, and
		// an attack that no longer clears the bar is not made at all.
		type sortie struct {
			piece *models.Piece
			from  string
		}
		var force []sortie

		// Source territories in a fixed order: map iteration order varies
		// between runs, which broke seeded replay.
		sourceNames := make([]string, 0, len(attackers))
		for territoryName := range attackers {
			sourceNames = append(sourceNames, territoryName)
		}
		sort.Strings(sourceNames)

		for _, territoryName := range sourceNames {
			pieces := attackers[territoryName]
			sourceTerritory := game.Board[territoryName]

			// Calculate how many to move based on success probability
			percentToMove := commitDefault
			if successProb < pressingOdds {
				percentToMove = commitPressing // a marginal attack needs weight
			}
			if successProb > cautiousOdds {
				percentToMove = commitCautious // a sure thing should not strip the source
			}

			numToMove := int(float64(len(pieces)) * percentToMove)
			if numToMove == 0 && len(pieces) > 0 && successProb >= minViableOdds {
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
				if len(pieces)-numToMove < victoryCityGarrison {
					numToMove = len(pieces) - victoryCityGarrison
					if numToMove < 0 {
						numToMove = 0
					}
				}
			}

			for i := 0; i < numToMove && i < len(pieces); i++ {
				force = append(force, sortie{pieces[i], territoryName})
			}
		}

		// Judge the force actually going, not the force that might have.
		chosen := make([]*models.Piece, len(force))
		for i, s := range force {
			chosen[i] = s.piece
		}
		if len(chosen) == 0 || EstimateAttackSuccess(chosen, defenders) < minProbability {
			continue // the fraction that can actually march does not justify the attack
		}

		for _, s := range force {
			if err := controller.PlanMove(s.piece.ID, s.from, target.Name); err == nil {
				transcript.LogMove(player.Name, s.piece.Name, s.from, target.Name, "combat")
				movesMade++
			}
		}

		attacksPlanned++

		// Limit number of attacks to keep things manageable
		if attacksPlanned >= maxAttacksPerTurn {
			break
		}
	}

	// Launch any plan whose force is assembled. Loading, sailing and landing
	// all happen in this phase, so an operation that has been forming for
	// several turns executes here in one go.
	launched := npc.ExecuteReadyPlans(controller, player, transcript)

	// The quartermaster reads this later: an outproduced power that found
	// nothing to hit this turn digs in instead (attack, expand, or fortify).
	npc.attacksThisTurn = attacksPlanned + launched

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

		// Retreat per the difficulty's doctrine: stubborn fighters break off
		// only when losses are extreme and they are still losing; everyone
		// else follows the general retreat evaluation.
		doctrine := DoctrineFor(npc.Difficulty)
		retreatDecider := func(initialAttackers, currentAttackers, initialDefenders, currentDefenders, round int) bool {
			if doctrine.Stubborn {
				lossRatio := float64(initialAttackers-currentAttackers) / float64(initialAttackers)
				return lossRatio > stubbornLossRatio && currentDefenders > currentAttackers
			}
			return ShouldAttackerRetreat(initialAttackers, currentAttackers, initialDefenders, currentDefenders, round)
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


func (npc *NPCAIPlayer) NoncombatMovePhase(controller *GameController, transcript *GameTranscript) error {
	player, _ := controller.GetCurrentPlayer()

	transcript.LogPhaseStart(player.Name, models.NoncombatMovePhase)

	// Gathering comes first: units committed to a plan walk to their port and
	// shipping sails to meet them. Doing this before general movement stops the
	// ordinary logic scattering an invasion force that has been assembling for
	// several turns.
	movesMade := npc.GatherForPlans(controller, player, transcript)
	movesMade += npc.SailNavalPlans(controller, player, transcript)

	// The quartermaster's pass: everything not held back by a defence plan or
	// an operation marches toward the fighting. Outnumbered fronts first,
	// until at least equal with the enemy next door; leftover surplus walks
	// forward anyway; surplus stranded on frontless islands goes by ferry.
	//
	// This replaced three priority loops that only ever sourced from "safe"
	// territories -- defined so strictly that a capital bordering an ally, a
	// neutral or someone else's nominal sea zone never qualified -- and were
	// capped at a handful of moves a turn against a factory output twice
	// that. Germany kept 135 of its 178 units in Berlin; the fronts starved
	// on five or six; the war froze by round six of every observed game.
	movesMade += npc.DisperseToFronts(controller, player, transcript)
	movesMade += npc.FerrySurplus(controller, player, transcript)

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

	// Place every purchased unit that has anywhere to go. One unplaceable unit
	// must not block the rest: the old loop always tried the head of the queue
	// and gave up entirely on the first failure, so a single transport bought
	// by a power with no coastal factory jammed the queue for the rest of the
	// game -- the USSR ended one probe with 126 units in the backlog and a
	// 41-unit army on the board.
	unitsPlaced := 0
	stuck := make(map[string]int)
	for progress := true; progress; {
		progress = false

		// Distinct pending types, in queue order.
		seen := make(map[string]bool)
		types := make([]string, 0)
		for _, unit := range game.PurchasedUnits[player.Name] {
			if !seen[unit.Type] {
				seen[unit.Type] = true
				types = append(types, unit.Type)
			}
		}

		for _, unitType := range types {
			for _, spot := range npc.placementSpots(controller, player, icTerritories, unitType) {
				if err := controller.MobilizeUnit(spot, unitType); err == nil {
					transcript.LogMobilize(player.Name, unitType, spot)
					unitsPlaced++
					progress = true
					break
				}
			}
		}
	}

	for _, unit := range game.PurchasedUnits[player.Name] {
		stuck[unit.Type]++
	}
	for _, unitType := range sortedWants(stuck) {
		transcript.LogAction(player.Name, fmt.Sprintf(
			"Could not place %dx %s (no valid location)", stuck[unitType], unitType))
	}
	if unitsPlaced == 0 {
		transcript.LogAction(player.Name, "No units mobilized")
	}

	return nil
}

// placementSpots lists where a unit of this type could be placed, best first.
func (npc *NPCAIPlayer) placementSpots(controller *GameController, player *models.Player, icTerritories []*models.Territory, unitType string) []string {
	game := controller.Game
	template, ok := game.GlobalPieceTemplates[unitType]
	if !ok {
		return nil
	}

	// A new factory goes where the purchase decided it should: the best
	// factory-less territory. The old path placed every unit "at the first
	// factory territory", which for a factory meant stacking it on top of an
	// existing one -- the USA ended one probe with six factories in Eastern US.
	if game.Units().Of(unitType).IsStructure {
		if best := npc.findBestTerritoryForFactory(game, player); best != nil {
			return []string{best.Name}
		}
		return nil
	}

	// Ships are launched into a sea zone beside a factory.
	if template.Terrain == models.Water {
		spots := make([]string, 0)
		for _, territory := range icTerritories {
			spots = append(spots, adjacentSeaZones(game, territory.Name)...)
		}
		return spots
	}

	spots := make([]string, 0, len(icTerritories))
	for _, territory := range icTerritories {
		spots = append(spots, territory.Name)
	}
	return spots
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
	pressure := strategicPressure(game, player)

	// The chain cost is a board-wide sum; price it once, not per neighbour.
	// Excluding the violated territory itself is handled below by adding its
	// own production back.
	allStrictProduction := strictChainCost(game, "")

	for _, ourTerritory := range player.Territories {
		for _, neighbor := range ourTerritory.ConnectedTo {
			if neighbor.Owner != player && !seen[neighbor.Name] {
				// Rulebook page 14: "At no time can an Allied power attack another Allied power,
				// or an Axis power attack another Axis power"
				// Skip allied territories - only target true enemies or neutrals
				if areAllies(player, neighbor.Owner) {
					continue // Skip allies
				}

				// Calculate strategic score
				score := neighbor.Production

				// Strict neutrals are on the table only for a power losing the
				// production race with nothing better to hit, and only at their
				// honest net worth: the province's production, minus what every
				// OTHER strict neutral would hand the enemy side by turning
				// hostile, minus a point for the toll. Enemy territory always
				// outranks a neutral of the same value -- taking it swings the
				// race twice, theirs down and ours up -- and a violation whose
				// diplomacy costs more than it gains is skipped entirely.
				if neighbor.Owner.Name == "Neutral" && neighbor.NeutralType == models.StrictNeutral {
					if !outproduced(pressure) {
						continue // time is not against us; leave the neutrals alone
					}
					chainCost := allStrictProduction - neighbor.Production
					score = neighbor.Production - chainCost - 1
					if score <= 0 {
						continue // the chain would hand the enemy more than we gain
					}
				}

				// Victory cities are MUCH more valuable
				if neighbor.IsVictoryCity {
					score += attackTargetVCBonus
				}

				// Bonus for territories that connect to more of our territories (easier to attack/defend)
				connectivityBonus := 0
				for _, connectedTo := range neighbor.ConnectedTo {
					if connectedTo.Owner == player {
						connectivityBonus++
					}
				}
				score += connectivityBonus

				// Penalty for heavily defended territories, counting the
				// garrison a neutral would raise (we'll still consider them
				// but lower priority)
				defenseStrength := 0
				for _, piece := range expectedDefenders(game, neighbor) {
					defenseStrength += int(piece.Defend)
				}
				if defenseStrength > defendedTargetStrength {
					score -= defendedTargetPenalty
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
				// A unit that cannot roll a die contributes nothing to an
				// attack. Transports and empty carriers (attack 0) used to be
				// swept along -- one game opened with a lone carrier attacking
				// a defended sea zone, rolling nothing for five rounds, and
				// dying.
				if piece.Attack <= 0 {
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

	// Vulnerable if the remaining defence cannot stand up to the worst threat
	return remainingDefense < int(float64(maxThreat)*vulnerableDefenceRatio)
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
	add(line, lineShare)
	add(punch, punchShare)
	add(air, airShare)
	return mix
}

// findStructureTemplate returns the board's buildable structure -- its factory
// or industrial complex -- under whatever name the board gives it. Sorted
// order, so a board with several structure types picks the same one every run.
func findStructureTemplate(game *models.Game) (string, *models.Piece, bool) {
	registry := game.Units()
	for _, name := range sortedTemplateNames(game) {
		if registry.Of(name).IsStructure {
			return name, game.GlobalPieceTemplates[name], true
		}
	}
	return "", nil, false
}

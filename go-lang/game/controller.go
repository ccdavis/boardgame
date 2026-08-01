package game

import (
	"boardgame/models"
	"fmt"
	"sort"
)

// GameController manages the game flow and turn sequence
type GameController struct {
	Game           *models.Game
	MoveTracker    *MovementTracker
	PendingBattles map[string]*Battle // Territory name -> Battle

	// PlannedLandings are a human player's booked amphibious assaults,
	// executed with (after) the combat moves. The NPC's equivalent lives in
	// Plans; see game/landing.go for why these are planned rather than
	// immediate.
	PlannedLandings []*PlannedLanding

	// Plans are the computer players' standing intentions, which outlive a
	// turn. They live here rather than on the AI because an NPCAIPlayer is
	// rebuilt for each turn in some paths -- the web server constructs one per
	// request -- so state held on the AI would be thrown away between turns.
	Plans *PlanBook
}

// NewGameController creates a new controller for a game
func NewGameController(game *models.Game) *GameController {
	plans := NewPlanBook()
	// Operations are christened from their side's codename list.
	plans.sideOf = func(power string) string {
		if player, ok := game.Players[power]; ok && player != nil {
			return player.Side
		}
		return ""
	}
	return &GameController{
		Game:           game,
		MoveTracker:    NewMovementTracker(),
		PendingBattles: make(map[string]*Battle),
		Plans:          plans,
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

	// Move to the next power that actually plays.
	//
	// The Players line doubles as the turn order and includes Neutral, which
	// exists only to own unclaimed territory. Stepping to it gave Neutral a full
	// turn: buying units and collecting income from a dozen territories.
	nextIndex := currentIndex
	for i := 0; i < len(gc.Game.PlayerOrder); i++ {
		nextIndex = (nextIndex + 1) % len(gc.Game.PlayerOrder)
		candidate := gc.Game.Players[gc.Game.PlayerOrder[nextIndex]]
		// A board with no Sides section marks nobody as turn-taking; fall back to
		// the old behaviour rather than deadlocking.
		if candidate == nil || candidate.TakesTurns || !gc.anyPowerTakesTurns() {
			break
		}
	}

	// Wrapping past the end of the order means a new round.
	if nextIndex <= currentIndex {
		gc.Game.Turn++
		gc.recordVictoryHold()
	}

	gc.Game.CurrentPower = gc.Game.PlayerOrder[nextIndex]
	gc.Game.CurrentPhase = models.PurchasePhase

	// Movement allowances are per turn, so the incoming power starts fresh.
	gc.MoveTracker.ResetTurn()

	return nil
}

// anyPowerTakesTurns reports whether the board declares playing powers at all.
func (gc *GameController) anyPowerTakesTurns() bool {
	for _, player := range gc.Game.Players {
		if player.TakesTurns {
			return true
		}
	}
	return false
}

// GetCurrentPlayer returns the player whose turn it is.
func (gc *GameController) GetCurrentPlayer() (*models.Player, error) {
	return gc.Game.GetCurrentPlayer()
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

// CollectIncome adds income to the current player's treasury.
//
// A power whose capital is in enemy hands collects nothing: the treasury was
// seized with the city, and no income flows until it is liberated.
func (gc *GameController) CollectIncome() error {
	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	if gc.capitalHeldByEnemy(player) {
		return nil
	}

	income, err := gc.CalculateIncome(player.Name)
	if err != nil {
		return err
	}

	player.IPCs += income
	return nil
}

// The victory-city thresholds from the rulebook: either side wins outright at
// immediateVictoryCities; the Axis wins by holding axisVictoryCities (and the
// Allies alliesVictoryCities) across a full round of play.
const (
	immediateVictoryCities = 13
	axisVictoryCities      = 9
	alliesVictoryCities    = 10
)

// CheckVictoryCondition checks if any side has won the game.
// Returns: winner ("Axis" or "Allies"), hasWon (bool), error.
//
// Cities are counted by the owner's Side, not by a list of power names. The
// old hardcoded list -- Germany and Japan against USSR, UK and USA -- meant a
// sixth power's victory cities counted for nobody: Italy could hold Rome and
// Southern Europe and the Axis got no credit for either. Whether holding
// cities ends the game at all is the board's VictoryCitiesEnabled switch, a
// play-time toggle rather than a property of the rules.
func (gc *GameController) CheckVictoryCondition() (string, bool, error) {
	if !gc.Game.VictoryCitiesEnabled {
		return "", false, nil
	}

	axisCities, alliedCities := gc.Game.CountVictoryCities()

	// Victory conditions (from rulebook):
	// - Axis wins if they control 9 cities for a full round
	// - Allies win if they control 10 cities for a full round
	// - Either side wins immediately if they control 13+ cities

	// Immediate victory
	if axisCities >= immediateVictoryCities {
		return "Axis", true, nil
	}
	if alliedCities >= immediateVictoryCities {
		return "Allies", true, nil
	}

	// Sustained victory: the threshold held across a full round of play.
	// recordVictoryHold counts round boundaries; two consecutive boundaries at
	// or above the threshold means every power had its turn and the cities
	// were still held. This was a comment for a long time ("requires
	// additional state") -- and without it, games between evenly matched
	// computer players could not end at all: the observed stable split was
	// 8-6, the immediate threshold 13, and every game ran to the turn cap.
	if gc.Game.VictoryHoldRounds >= 2 {
		return gc.Game.VictoryHoldSide, true, nil
	}

	// Potential victory: at the threshold but not yet held for a full round.
	if axisCities >= axisVictoryCities {
		return "Axis", false, nil
	}
	if alliedCities >= alliesVictoryCities {
		return "Allies", false, nil
	}

	return "", false, nil
}

// recordVictoryHold notes, at a round boundary, whether a side stands at or
// above its sustained-victory threshold, and for how many consecutive
// boundaries it has done so.
func (gc *GameController) recordVictoryHold() {
	if !gc.Game.VictoryCitiesEnabled {
		gc.Game.VictoryHoldSide, gc.Game.VictoryHoldRounds = "", 0
		return
	}

	axis, allies := gc.Game.CountVictoryCities()
	side := ""
	switch {
	case axis >= axisVictoryCities:
		side = "Axis"
	case allies >= alliesVictoryCities:
		side = "Allies"
	}

	if side == "" || side != gc.Game.VictoryHoldSide {
		gc.Game.VictoryHoldSide = side
		if side == "" {
			gc.Game.VictoryHoldRounds = 0
		} else {
			gc.Game.VictoryHoldRounds = 1
		}
		return
	}
	gc.Game.VictoryHoldRounds++
}

// GetVictoryCityCounts returns the number of victory cities each side controls.
func (gc *GameController) GetVictoryCityCounts() (axis int, allies int) {
	return gc.Game.CountVictoryCities()
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

	// Check if territory exists
	territory, exists := gc.Game.Board[territoryName]
	if !exists {
		return fmt.Errorf("territory %s not found", territoryName)
	}

	units := gc.Game.Units()
	template, hasTemplate := gc.Game.GlobalPieceTemplates[unitType]
	if !hasTemplate {
		return fmt.Errorf("unit type %s not found", unitType)
	}

	if template.Terrain == models.Water {
		// A ship is launched into a sea zone beside the yard that built it,
		// not parked in the factory's home province. Sea zones have nominal
		// owners in the board file that mean nothing, so the requirement is
		// adjacency to one of this power's production centres -- transports
		// and battleships used to be placed on dry land and then awkwardly
		// sailed out.
		if territory.Terrain != models.Water {
			return fmt.Errorf("%s is a sea unit and must be placed in a sea zone", unitType)
		}
		if !gc.adjacentToOwnProduction(territory, player) {
			return fmt.Errorf("%s does not border one of your industrial complexes", territoryName)
		}
	} else {
		if territory.Owner != player {
			return fmt.Errorf("you do not own %s", territoryName)
		}

		// New units appear at a production centre, not anywhere you happen to
		// hold. The rule existed only in the dead copy of this method in
		// models/turn_state.go, so the live path let units be placed on any
		// owned territory at all.
		hasProduction := false
		for _, pieceID := range territory.Pieces {
			if units.For(gc.Game.Pieces[pieceID]).IsStructure {
				hasProduction = true
				break
			}
		}
		if !hasProduction && !units.Of(unitType).IsStructure {
			return fmt.Errorf("%s has no industrial complex to build in", territoryName)
		}
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

	// PlacePieces stamps the territory's owner on the piece, which is right on
	// land but wrong at sea: a launched ship would inherit the sea zone's
	// meaningless nominal owner. The ship belongs to whoever built it.
	if newPiece := gc.Game.Pieces[gc.Game.NextPieceID-1]; newPiece != nil {
		newPiece.Owner = player
	}

	return nil
}

// adjacentToOwnProduction reports whether a sea zone borders a territory this
// player owns that contains an industrial complex.
func (gc *GameController) adjacentToOwnProduction(seaZone *models.Territory, player *models.Player) bool {
	units := gc.Game.Units()
	for _, neighbour := range seaZone.ConnectedTo {
		if neighbour.Owner != player {
			continue
		}
		for _, pieceID := range neighbour.Pieces {
			if units.For(gc.Game.Pieces[pieceID]).IsStructure {
				return true
			}
		}
	}
	return false
}

// PlacementTargets lists every territory where this player could mobilize a
// unit of the given type right now, sorted by name. It answers the question a
// placement interface has to ask before offering a destination, applying the
// same rules MobilizeUnit enforces: ships launch into sea zones beside an
// owned industrial complex, land units appear at an owned complex, and a new
// structure may rise in any owned land territory.
func (gc *GameController) PlacementTargets(player *models.Player, unitType string) []string {
	template, hasTemplate := gc.Game.GlobalPieceTemplates[unitType]
	if !hasTemplate || player == nil {
		return nil
	}
	units := gc.Game.Units()

	hasComplex := func(territory *models.Territory) bool {
		for _, pieceID := range territory.Pieces {
			if units.For(gc.Game.Pieces[pieceID]).IsStructure {
				return true
			}
		}
		return false
	}

	var targets []string
	for name, territory := range gc.Game.Board {
		switch {
		case template.Terrain == models.Water:
			if territory.Terrain == models.Water && gc.adjacentToOwnProduction(territory, player) {
				targets = append(targets, name)
			}
		case territory.Owner != player || territory.Terrain == models.Water:
			// Land units only appear on land the player holds.
		case units.Of(unitType).IsStructure || hasComplex(territory):
			targets = append(targets, name)
		}
	}
	sort.Strings(targets)
	return targets
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

	// Get current player
	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	// Validate the move
	err = ValidateMovement(gc.Game, pieceID, from, to, moveType)
	if err != nil {
		return err
	}

	// Check if piece can reach territory using pathfinding that considers enemy units
	piece := gc.Game.Pieces[pieceID]
	distance, path, err := CalculateMovementPathForPiece(gc.Game, piece, from, to, player, moveType)
	if err != nil {
		return fmt.Errorf("cannot move %s from %s to %s: %v", piece.Name, from, to, err)
	}

	// Check against what the piece has left this turn, not its full allowance.
	remaining := gc.MoveTracker.Remaining(pieceID, int(piece.Movement))
	if distance > remaining {
		return fmt.Errorf("piece %s cannot reach %s from %s (distance=%d, movement left this turn=%d)",
			piece.Name, to, from, distance, remaining)
	}

	// An aircraft landing at sea needs a carrier slot that is still free once
	// every already-planned move is counted -- aircraft planned onto the same
	// carrier, carriers planned to sail away, carriers planned to arrive. The
	// static per-slot check in pathfinding cannot see the tracker, so two
	// fighters could both be promised the last slot in one phase.
	if piece.Terrain == models.Air && moveType == NoncombatMove {
		if dest := gc.Game.Board[to]; dest != nil && dest.Terrain == models.Water {
			if !gc.CarrierSlotFree(piece, dest, player) {
				return fmt.Errorf("no carrier slot left in %s once planned moves are counted", to)
			}
		}
	}

	// Record any territory this move blitzes through, so execution can take it.
	blitzed := BlitzedTerritories(gc.Game, piece, path, player)

	// Add to movement tracker
	err = gc.MoveTracker.AddMoveWithCost(pieceID, from, to, moveType, distance, blitzed...)
	if err != nil {
		return err
	}

	return nil
}

// CancelMove cancels a planned move
func (gc *GameController) CancelMove(pieceID int) error {
	return gc.MoveTracker.RemoveMove(pieceID)
}

// CarrierSlotFree reports whether a sea zone will still have a carrier slot
// for this aircraft after every planned move this phase is accounted for.
//
// Slots come from friendly carriers that will be in the zone when moves
// execute: those already there and not planned to leave, plus those planned to
// arrive. Occupants are the friendly aircraft already parked there plus every
// aircraft already planned to land there. Cancelling a planned move frees its
// slot again automatically, because this recounts from the tracker each time.
func (gc *GameController) CarrierSlotFree(aircraft *models.Piece, zone *models.Territory, player *models.Player) bool {
	leaving := make(map[int]bool)
	arriving := make(map[int]bool)
	bookings := 0
	for _, move := range gc.MoveTracker.Moves {
		mover := gc.Game.Pieces[move.PieceID]
		if mover == nil || mover.Owner == nil {
			continue
		}
		friendly := mover.Owner == player || areAllies(mover.Owner, player)
		if !friendly {
			continue
		}
		switch {
		case mover.Terrain == models.Water && move.From == zone.Name:
			leaving[move.PieceID] = true
		case mover.Terrain == models.Water && move.To == zone.Name:
			arriving[move.PieceID] = true
		case mover.Terrain == models.Air && move.To == zone.Name:
			bookings++
		}
	}

	slotsOn := func(ship *models.Piece) int {
		if ship == nil || ship.Owner == nil {
			return 0
		}
		if ship.Owner != player && !areAllies(ship.Owner, player) {
			return 0
		}
		for _, kind := range ship.CanCarry {
			if kind == aircraft.Name {
				return int(ship.Capacity) - len(ship.Holding)
			}
		}
		return 0
	}

	slots := 0
	occupants := 0
	for _, id := range zone.Pieces {
		occupant := gc.Game.Pieces[id]
		if occupant == nil {
			continue
		}
		if !leaving[id] {
			slots += slotsOn(occupant)
		}
		// Friendly aircraft already in the zone are parked on those carriers.
		if occupant.Terrain == models.Air && occupant.Owner != nil &&
			(occupant.Owner == player || areAllies(occupant.Owner, player)) {
			occupants++
		}
	}
	for id := range arriving {
		slots += slotsOn(gc.Game.Pieces[id])
	}

	return occupants+bookings < slots
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

	// Track if any strict neutral is being attacked
	strictNeutralAttacked := false

	// Execute each move
	for _, move := range combatMoves {
		err := gc.Game.MovePiece(move.PieceID, move.From, move.To)
		if err != nil {
			return fmt.Errorf("failed to execute move: %v", err)
		}

		// Undefended enemy territory driven through is taken on the way past.
		for _, name := range move.Blitzed {
			if blitzed := gc.Game.Board[name]; blitzed != nil && blitzed.Owner != player {
				if err := gc.CaptureTerritory(name, player.Name); err != nil {
					return fmt.Errorf("failed to take %s while blitzing: %v", name, err)
				}
			}
		}

		// Check if move creates a battle
		toTerritory := gc.Game.Board[move.To]
		if toTerritory.Owner.Name != player.Name {
			// A neutral under attack defends itself, and violating a strict
			// one levies the toll and rouses the rest. violateNeutral is
			// idempotent -- a mobilised country is not violated twice -- so a
			// second attacker needs no separate bookkeeping here.
			if gc.violateNeutral(toTerritory, player) {
				strictNeutralAttacked = true
			}

			// Moving into hostile territory - create battle
			if _, exists := gc.PendingBattles[move.To]; !exists {
				// Create new battle
				// The battle type follows the terrain. It was hardcoded to
				// LandBattle, and every submarine rule in CombatRound is gated
				// on SeaBattle -- so in a real game submarines never rolled a
				// single die, attacking or defending.
				battleType := LandBattle
				if toTerritory.Terrain == models.Water {
					battleType = SeaBattle
				}
				battle := NewBattle(move.To, battleType, player.Name, toTerritory.Owner.Name)
				gc.PendingBattles[move.To] = battle
			}
			// Track this piece as an attacker, and where it came from.
			gc.PendingBattles[move.To].AttackingPieceIDs = append(
				gc.PendingBattles[move.To].AttackingPieceIDs, move.PieceID)
			if gc.PendingBattles[move.To].AttackerOrigins == nil {
				gc.PendingBattles[move.To].AttackerOrigins = make(map[int]string)
			}
			gc.PendingBattles[move.To].AttackerOrigins[move.PieceID] = move.From
		}
	}

	// If a strict neutral was attacked, trigger chain reaction
	if strictNeutralAttacked {
		gc.TriggerStrictNeutralChainReaction(player)
	}

	// Booked amphibious assaults come ashore now, after the fleet has moved:
	// a transport may load, sail and land within this one phase, so its drop
	// zone is only certain once the moves above have run.
	gc.executePlannedLandings(player)

	// Drop the executed combat plans but keep the movement they consumed --
	// clearing that too handed every unit a second full allowance for the
	// noncombat phase.
	newMoves := gc.MoveTracker.GetMovesByType(NoncombatMove)
	gc.MoveTracker.ClearPlans()
	for _, move := range newMoves {
		gc.MoveTracker.AddMoveWithCost(move.PieceID, move.From, move.To, move.Type, move.DistanceCost)
	}

	return nil
}

// ExecuteNoncombatMoves executes all noncombat moves
func (gc *GameController) ExecuteNoncombatMoves() error {
	if gc.Game.CurrentPhase != models.NoncombatMovePhase {
		return fmt.Errorf("can only execute noncombat moves during Noncombat Move phase")
	}

	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	// Get all noncombat moves
	noncombatMoves := gc.MoveTracker.GetMovesByType(NoncombatMove)

	// Execute each move
	for _, move := range noncombatMoves {
		toTerritory := gc.Game.Board[move.To]

		// Check if this is activating a pro-Allied or pro-Axis neutral
		if toTerritory.Owner.Name == "Neutral" &&
			(toTerritory.NeutralType == models.ProAlliedNeutral || toTerritory.NeutralType == models.ProAxisNeutral) {
			// Activate the neutral territory
			err := gc.ActivateNeutralTerritory(move.To, player.Name)
			if err != nil {
				return fmt.Errorf("failed to activate neutral: %v", err)
			}
		}

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
	return gc.ResolveBattleWithRetreat(territoryName, diceRoller, nil)
}

// ResolveBattleWithRetreat resolves a battle with optional retreat decision callback
func (gc *GameController) ResolveBattleWithRetreat(territoryName string, diceRoller *DiceRoller, retreatDecider RetreatDecider) (*BattleResult, error) {
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
	// Structures -- factories and industrial complexes -- are captured with the
	// territory, not fought over. Treating them as defenders let them roll
	// defence dice, be chosen as casualties, and keep a battle alive after every
	// real defender was gone, because the "no defenders left" test counted them.
	units := gc.Game.Units()
	var capturedStructures []*models.Piece

	for _, pieceID := range territory.Pieces {
		piece := gc.Game.Pieces[pieceID]
		switch {
		case attackingIDs[pieceID]:
			attackerPieces = append(attackerPieces, piece)
		case units.For(piece).IsStructure:
			capturedStructures = append(capturedStructures, piece)
		default:
			defenderPieces = append(defenderPieces, piece)
		}
	}
	// Structures survive the battle and change hands with the territory.
	defer func() {
		if territory.Owner != nil {
			for _, structure := range capturedStructures {
				structure.Owner = territory.Owner
			}
		}
	}()

	battle.Attackers = attackerPieces
	battle.Defenders = defenderPieces

	// Fire support was promised at landing time; keep only the ships that are
	// still afloat. Battles resolve in a fixed order within the phase, and a
	// sea battle in the drop zone can sink the bombarding squadron before the
	// land battle it was covering is fought.
	if len(battle.Bombarding) > 0 {
		afloat := make([]*models.Piece, 0, len(battle.Bombarding))
		for _, ship := range battle.Bombarding {
			if _, ok := gc.Game.Pieces[ship.ID]; ok {
				afloat = append(afloat, ship)
			}
		}
		battle.Bombarding = afloat
	}

	// Resolve the combat with retreat option
	result, err := ResolveCombatWithRetreat(battle, diceRoller, 100, retreatDecider)
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

	// A broken-off attack withdraws. Survivors go back where they came from;
	// leaving them in the contested territory made them defenders of the enemy
	// they had just failed to dislodge.
	if result.AttackerRetreated {
		gc.withdrawAttackers(battle, territoryName, result.AttackersRemaining)
	}

	// Handle territory capture (only if attacker won, not if they retreated)
	if result.AttackerWins && !result.AttackerRetreated {
		err = gc.CaptureTerritory(territoryName, attacker.Name)
		if err != nil {
			return result, fmt.Errorf("failed to capture territory: %v", err)
		}
	}

	// Remove battle from pending
	delete(gc.PendingBattles, territoryName)

	return result, nil
}

// withdrawAttackers moves surviving attackers back to where they came from.
func (gc *GameController) withdrawAttackers(battle *Battle, territoryName string, survivors []*models.Piece) {
	territory := gc.Game.Board[territoryName]
	if territory == nil {
		return
	}

	for _, piece := range survivors {
		if piece == nil {
			continue
		}
		origin := battle.AttackerOrigins[piece.ID]
		if origin == "" || origin == territoryName {
			continue // nowhere recorded to fall back to
		}
		from := gc.Game.Board[origin]
		if from == nil {
			continue
		}

		remaining := make([]int, 0, len(territory.Pieces))
		for _, id := range territory.Pieces {
			if id != piece.ID {
				remaining = append(remaining, id)
			}
		}
		territory.Pieces = remaining
		from.Pieces = append(from.Pieces, piece.ID)
	}
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

	// Everything left standing in the territory changes hands with it. That is
	// how a captured factory ends up building for its new owner.
	for _, pieceID := range territory.Pieces {
		if piece := gc.Game.Pieces[pieceID]; piece != nil {
			piece.Owner = newOwner
		}
	}

	// Capturing a capital seizes its treasury. The Capitals section, the
	// grammar and the docs all promised this; nothing implemented it, so a
	// capital was just another province with a flag. Only an enemy loots --
	// an ally walking into a fallen capital is liberating it, not sacking it.
	for _, name := range gc.Game.PlayerOrder {
		player := gc.Game.Players[name]
		if player == nil || player == newOwner || player.Capital != territoryName {
			continue
		}
		if areAllies(player, newOwner) {
			continue
		}
		if player.IPCs > 0 {
			newOwner.IPCs += player.IPCs
			player.IPCs = 0
		}
	}

	return nil
}

// capitalHeldByEnemy reports whether a power's capital is in enemy hands.
func (gc *GameController) capitalHeldByEnemy(player *models.Player) bool {
	if player.Capital == "" {
		return false
	}
	capital, ok := gc.Game.Board[player.Capital]
	if !ok || capital.Owner == nil || capital.Owner == player {
		return false
	}
	return !areAllies(capital.Owner, player)
}

// removePieceFromBoard removes a piece from the game entirely
func (gc *GameController) removePieceFromBoard(piece *models.Piece, territoryName string) {
	if piece == nil {
		return
	}
	// The piece knows its own ID. The previous scan defaulted to 0 when it found
	// nothing, then deleted key 0 -- harmless only because IDs start at 1, and it
	// silently swallowed double removals.
	pieceID := piece.ID

	// Anything the piece was carrying goes down with it. Cargo lives only in
	// Holding and is absent from the territory's piece list, so without this it
	// stays in Game.Pieces forever, in no territory, still counted by unit
	// tallies.
	for _, cargoID := range piece.Holding {
		delete(gc.Game.Pieces, cargoID)
	}

	// Remove from territory
	territory := gc.Game.Board[territoryName]
	if territory == nil {
		delete(gc.Game.Pieces, pieceID)
		return
	}
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

// LoadUnit loads a unit onto a transport during noncombat move phase
func (gc *GameController) LoadUnit(transportID, pieceID int) error {
	// Loading happens during either movement phase. Restricting it to noncombat
	// movement made an amphibious assault impossible: the rules have a transport
	// load, sail and land within the combat-move phase, so a landing force could
	// never get aboard.
	if !gc.inMovementPhase() {
		return fmt.Errorf("can only load units during a movement phase")
	}

	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}

	// Validate the load operation
	err = ValidateLoad(gc.Game, transportID, pieceID, player.Name)
	if err != nil {
		return err
	}

	// Execute the load
	err = gc.Game.LoadPiece(transportID, pieceID)
	if err != nil {
		return fmt.Errorf("failed to load piece: %v", err)
	}

	return nil
}

// UnloadUnit unloads a unit from a transport during noncombat move phase
func (gc *GameController) UnloadUnit(transportID, pieceID int, destinationName string) error {
	// Unloading into a hostile territory is an attack, so it belongs to the
	// combat-move phase; unloading onto friendly ground is a noncombat move.
	if !gc.inMovementPhase() {
		return fmt.Errorf("can only unload units during a movement phase")
	}

	// Only the owner works the winches. ValidateUnload checks geometry, not
	// allegiance, and the web layer names transports by their cargo -- without
	// this, any request could disembark another power's troops.
	player, err := gc.GetCurrentPlayer()
	if err != nil {
		return err
	}
	if transport, ok := gc.Game.Pieces[transportID]; !ok || transport.Owner != player {
		return fmt.Errorf("transport %d does not belong to %s", transportID, player.Name)
	}
	if piece, ok := gc.Game.Pieces[pieceID]; !ok || piece.Owner != player {
		return fmt.Errorf("piece %d does not belong to %s", pieceID, player.Name)
	}

	// Validate the unload operation
	if err := ValidateUnload(gc.Game, transportID, pieceID, destinationName); err != nil {
		return err
	}

	// Execute the unload
	err = gc.Game.UnloadPiece(transportID, pieceID, destinationName)
	if err != nil {
		return fmt.Errorf("failed to unload piece: %v", err)
	}

	return nil
}

// inMovementPhase reports whether units may currently be moved.
func (gc *GameController) inMovementPhase() bool {
	return gc.Game.CurrentPhase == models.CombatMovePhase ||
		gc.Game.CurrentPhase == models.NoncombatMovePhase
}

// GetTransportCargo returns the piece IDs held by a transport
func (gc *GameController) GetTransportCargo(transportID int) ([]int, error) {
	transport, exists := gc.Game.Pieces[transportID]
	if !exists {
		return nil, fmt.Errorf("transport %d not found", transportID)
	}

	return transport.Holding, nil
}

func sortedTerritoryNames(g *models.Game) []string {
	names := make([]string, 0, len(g.Board))
	for name := range g.Board {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}


package game

import (
	"fmt"
	"sort"

	"boardgame/models"
)

// A battle fought one round at a time.
//
// ResolveBattle runs a fight from first shot to last with the engine choosing
// every casualty and never breaking off. That is right for the computer
// players and for "resolve everything", but a human attacker is owed the
// decisions the rules give them between rounds: press on or retreat, which
// units to lose, whether the submarines submerge. A LiveBattle holds a fight
// paused between rounds so a front end can ask.
//
// The mechanics are the engine's own -- CombatRound rolls the dice, the same
// pre-combat steps run once -- and concludeLive ends the fight exactly as
// ResolveBattle does, so a battle fought this way leaves the board in the same
// state it would have reached under automatic resolution.

// LiveBattle is a battle in progress.
type LiveBattle struct {
	Territory string
	Battle    *Battle

	Attackers []*models.Piece
	Defenders []*models.Piece

	initialAttackers int
	initialDefenders int
	boosted          map[*models.Piece]int16
	drowned          []*models.Piece

	// Casualties so far, both sides, in the order they fell.
	AttackerCasualties []*models.Piece
	DefenderCasualties []*models.Piece

	// Round is how many rounds of dice have been rolled.
	Round int

	// PendingAttackerHits is how many hits the attacker still has to assign
	// to their own units before the next round may be fought. Zero when the
	// engine chooses (AutoCasualties) or nothing is owed.
	PendingAttackerHits int

	// AutoCasualties lets the engine pick the attacker's losses, as it does
	// for the defender's. A front end that offers casualty selection leaves
	// it false; "fight it out" sets it.
	AutoCasualties bool

	// Retreated is set once the attacker has broken off; amphibious units,
	// which have no line of retreat, fight on if any remain.
	Retreated bool

	Log  []BattleRoundReport
	Done bool
	// Result is set when Done.
	Result *BattleResult
}

// BattleRoundReport is what happened in one round, for display.
type BattleRoundReport struct {
	Round          int      `json:"round"`
	AttackerHits   []Hit    `json:"attackerHits"`
	DefenderHits   []Hit    `json:"defenderHits"`
	AttackerLosses []string `json:"attackerLosses"`
	DefenderLosses []string `json:"defenderLosses"`
	Notes          []string `json:"notes,omitempty"`
}

// BeginBattle opens a pending battle for round-by-round play, or returns the
// one already open. The pre-combat steps -- the sea fight's verdict on the
// landing, anti-aircraft fire, shore bombardment, artillery support -- run
// here, once, as round zero.
func (gc *GameController) BeginBattle(territoryName string) (*LiveBattle, error) {
	if live, ok := gc.LiveBattles[territoryName]; ok {
		return live, nil
	}
	battle, exists := gc.PendingBattles[territoryName]
	if !exists {
		return nil, fmt.Errorf("no battle pending in %s", territoryName)
	}
	territory := gc.Game.Board[territoryName]
	attacker := gc.Game.Players[battle.AttackerID]

	for _, zone := range battle.AmphibiousFrom {
		if _, pending := gc.PendingBattles[zone]; pending {
			return nil, fmt.Errorf("the sea battle in %s must be resolved before the landing in %s",
				zone, territoryName)
		}
	}

	live := &LiveBattle{
		Territory:      territoryName,
		Battle:         battle,
		AutoCasualties: false,
	}
	if gc.LiveBattles == nil {
		gc.LiveBattles = make(map[string]*LiveBattle)
	}

	// Troops whose drop zone stayed in enemy hands never made it ashore.
	live.drowned = gc.drownCutOffAttackers(battle, territoryName, attacker)
	if len(battle.AttackingPieceIDs) == 0 {
		live.Done = true
		live.Result = &BattleResult{DefenderWins: true, AttackerCasualties: live.drowned}
		delete(gc.PendingBattles, territoryName)
		return live, nil
	}

	attackingIDs := make(map[int]bool, len(battle.AttackingPieceIDs))
	for _, id := range battle.AttackingPieceIDs {
		attackingIDs[id] = true
	}
	units := gc.Game.Units()
	for _, pieceID := range territory.Pieces {
		piece := gc.Game.Pieces[pieceID]
		switch {
		case piece == nil:
		case attackingIDs[pieceID]:
			live.Attackers = append(live.Attackers, piece)
		case units.For(piece).IsStructure:
			// captured with the ground, not fought over
		default:
			live.Defenders = append(live.Defenders, piece)
		}
	}
	battle.Attackers = live.Attackers
	battle.Defenders = live.Defenders

	if len(battle.Bombarding) > 0 {
		afloat := make([]*models.Piece, 0, len(battle.Bombarding))
		for _, ship := range battle.Bombarding {
			if _, ok := gc.Game.Pieces[ship.ID]; ok {
				afloat = append(afloat, ship)
			}
		}
		battle.Bombarding = afloat
	}

	report := BattleRoundReport{Round: 0}
	roller := gc.battleDice()

	// Anti-aircraft fire, once, then the guns sit the battle out.
	var aaaUnits, airUnits []*models.Piece
	for _, unit := range live.Defenders {
		if models.CapabilitiesOf(unit).IsAA {
			aaaUnits = append(aaaUnits, unit)
		}
	}
	for _, unit := range live.Attackers {
		if unit.Terrain == models.Air {
			airUnits = append(airUnits, unit)
		}
	}
	if len(aaaUnits) > 0 && len(airUnits) > 0 {
		hits := roller.RollAAAFire(aaaUnits, airUnits)
		if len(hits) > 0 {
			lost := SelectCasualties(airUnits, len(hits))
			live.AttackerCasualties = append(live.AttackerCasualties, lost...)
			live.Attackers = RemoveCasualties(live.Attackers, lost)
			report.AttackerLosses = append(report.AttackerLosses, names(lost)...)
			report.Notes = append(report.Notes, fmt.Sprintf("Anti-aircraft fire: %d shot down", len(lost)))
		} else {
			report.Notes = append(report.Notes, "Anti-aircraft fire missed")
		}
	}
	live.Defenders = RemoveCasualties(live.Defenders, aaaUnits)

	// Shore bombardment, once, before the first round.
	if len(battle.Bombarding) > 0 && len(live.Defenders) > 0 && battle.AmphibiousUnits > 0 {
		hits := roller.RollBombardment(battle.Bombarding, battle.AmphibiousUnits)
		if len(hits) > 0 {
			lost := SelectCasualties(live.Defenders, len(hits))
			live.DefenderCasualties = append(live.DefenderCasualties, lost...)
			live.Defenders = RemoveCasualties(live.Defenders, lost)
			report.DefenderLosses = append(report.DefenderLosses, names(lost)...)
			report.Notes = append(report.Notes, fmt.Sprintf("Shore bombardment: %d hit", len(hits)))
		} else {
			report.Notes = append(report.Notes, "Shore bombardment missed")
		}
	}

	live.boosted = ApplyArtillerySupport(live.Attackers)
	live.initialAttackers = len(live.Attackers)
	live.initialDefenders = len(live.Defenders)
	if len(report.Notes) > 0 || len(report.AttackerLosses) > 0 || len(report.DefenderLosses) > 0 {
		live.Log = append(live.Log, report)
	}

	gc.LiveBattles[territoryName] = live
	gc.settleIfDecided(live)
	return live, nil
}

// battleDice is the roller used for live battles: seeded when the controller
// has one (replayable games), fresh otherwise.
func (gc *GameController) battleDice() *DiceRoller {
	if gc.Dice != nil {
		return gc.Dice
	}
	gc.Dice = NewDiceRoller()
	return gc.Dice
}

// FightRound rolls one round of combat. If the attacker took hits and is
// choosing casualties, the round ends with PendingAttackerHits set and the
// next round waits on AssignCasualties.
func (gc *GameController) FightRound(territoryName string) (*LiveBattle, error) {
	live, ok := gc.LiveBattles[territoryName]
	if !ok {
		return nil, fmt.Errorf("no battle in progress in %s", territoryName)
	}
	if live.Done {
		return live, nil
	}
	if live.PendingAttackerHits > 0 {
		return live, fmt.Errorf("%d hit(s) still to assign before the next round", live.PendingAttackerHits)
	}

	battle := live.Battle
	battle.Attackers = live.Attackers
	battle.Defenders = live.Defenders
	attackerHits, defenderHits, surprise := gc.battleDice().CombatRound(battle)
	live.Attackers = battle.Attackers
	live.Defenders = battle.Defenders

	report := BattleRoundReport{Round: live.Round + 1}
	report.AttackerHits = append(append([]Hit{}, surprise.AttackerHits...), attackerHits...)
	report.DefenderHits = append(append([]Hit{}, surprise.DefenderHits...), defenderHits...)
	live.AttackerCasualties = append(live.AttackerCasualties, surprise.Attacker...)
	live.DefenderCasualties = append(live.DefenderCasualties, surprise.Defender...)
	report.AttackerLosses = append(report.AttackerLosses, names(surprise.Attacker)...)
	report.DefenderLosses = append(report.DefenderLosses, names(surprise.Defender)...)
	if len(surprise.AttackerHits)+len(surprise.DefenderHits) > 0 {
		report.Notes = append(report.Notes, "Submarine surprise strike")
	}

	// The defender's losses are always the engine's choice.
	lost := SelectCasualties(live.Defenders, len(attackerHits))
	live.DefenderCasualties = append(live.DefenderCasualties, lost...)
	live.Defenders = RemoveCasualties(live.Defenders, lost)
	report.DefenderLosses = append(report.DefenderLosses, names(lost)...)

	live.Round++
	live.Log = append(live.Log, report)

	// The attacker's losses: chosen by the engine, or owed to the player.
	owed := len(defenderHits)
	if owed > 0 {
		if live.AutoCasualties || owed >= absorbable(live.Attackers) {
			lost := SelectCasualties(live.Attackers, owed)
			live.AttackerCasualties = append(live.AttackerCasualties, lost...)
			live.Attackers = RemoveCasualties(live.Attackers, lost)
			live.Log[len(live.Log)-1].AttackerLosses = append(live.Log[len(live.Log)-1].AttackerLosses, names(lost)...)
		} else {
			live.PendingAttackerHits = owed
			return live, nil
		}
	}

	gc.settleIfDecided(live)
	return live, nil
}

// absorbable is how many hits a force can take before nothing is left.
func absorbable(units []*models.Piece) int {
	n := 0
	for _, unit := range units {
		n += models.CapabilitiesOf(unit).MaxHits - unit.Hits
	}
	return n
}

// AssignCasualties applies the attacker's chosen losses: hits per piece ID.
// The total must match what is owed; a multi-hit unit may take more than one.
func (gc *GameController) AssignCasualties(territoryName string, hits map[int]int) (*LiveBattle, error) {
	live, ok := gc.LiveBattles[territoryName]
	if !ok {
		return nil, fmt.Errorf("no battle in progress in %s", territoryName)
	}
	if live.PendingAttackerHits == 0 {
		return live, fmt.Errorf("no casualties are owed in %s", territoryName)
	}
	total := 0
	byID := make(map[int]*models.Piece, len(live.Attackers))
	for _, unit := range live.Attackers {
		byID[unit.ID] = unit
	}
	for id, n := range hits {
		unit, ok := byID[id]
		if !ok {
			return live, fmt.Errorf("piece %d is not among the attackers", id)
		}
		if n < 0 || n > models.CapabilitiesOf(unit).MaxHits-unit.Hits {
			return live, fmt.Errorf("%s %d cannot take %d hit(s)", unit.Name, id, n)
		}
		total += n
	}
	if total != live.PendingAttackerHits {
		return live, fmt.Errorf("assign exactly %d hit(s); %d given", live.PendingAttackerHits, total)
	}

	var lost []*models.Piece
	ids := make([]int, 0, len(hits))
	for id := range hits {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		n := hits[id]
		if n == 0 {
			continue
		}
		unit := byID[id]
		unit.Hits += n
		if unit.Hits >= models.CapabilitiesOf(unit).MaxHits {
			lost = append(lost, unit)
		}
	}
	live.AttackerCasualties = append(live.AttackerCasualties, lost...)
	live.Attackers = RemoveCasualties(live.Attackers, lost)
	if len(live.Log) > 0 {
		last := &live.Log[len(live.Log)-1]
		last.AttackerLosses = append(last.AttackerLosses, names(lost)...)
	}
	live.PendingAttackerHits = 0

	gc.settleIfDecided(live)
	return live, nil
}

// CanRetreat reports whether the attacker may break off: at least one round
// fought, no casualties owed, and some attacker with a line of retreat.
// Troops landed from the sea have none.
func (live *LiveBattle) CanRetreat() bool {
	if live.Done || live.Round == 0 || live.PendingAttackerHits > 0 || live.Retreated {
		return false
	}
	for _, unit := range live.Attackers {
		if live.Battle.AttackerOrigins[unit.ID] != "" {
			return true
		}
	}
	return false
}

// Retreat withdraws every attacker that has somewhere to withdraw to.
// Amphibious troops stay and fight on; if nothing stays, the battle ends.
func (gc *GameController) Retreat(territoryName string) (*LiveBattle, error) {
	live, ok := gc.LiveBattles[territoryName]
	if !ok {
		return nil, fmt.Errorf("no battle in progress in %s", territoryName)
	}
	if !live.CanRetreat() {
		return live, fmt.Errorf("the attackers in %s cannot retreat now", territoryName)
	}
	var leaving, staying []*models.Piece
	for _, unit := range live.Attackers {
		if live.Battle.AttackerOrigins[unit.ID] != "" {
			leaving = append(leaving, unit)
		} else {
			staying = append(staying, unit)
		}
	}
	gc.withdrawAttackers(live.Battle, territoryName, leaving)
	// Withdrawn units are no longer attackers here.
	kept := make([]int, 0, len(live.Battle.AttackingPieceIDs))
	gone := make(map[int]bool, len(leaving))
	for _, unit := range leaving {
		gone[unit.ID] = true
	}
	for _, id := range live.Battle.AttackingPieceIDs {
		if !gone[id] {
			kept = append(kept, id)
		}
	}
	live.Battle.AttackingPieceIDs = kept
	live.Attackers = staying
	live.Retreated = true
	live.Log = append(live.Log, BattleRoundReport{
		Round: live.Round,
		Notes: []string{fmt.Sprintf("%d unit(s) withdrew", len(leaving))},
	})
	if len(staying) == 0 {
		gc.concludeLive(live)
	}
	return live, nil
}

// CanSubmerge reports whether the attacker's submarines may submerge: any
// are present, and no defending destroyer keeps them on the surface.
func (live *LiveBattle) CanSubmerge() bool {
	if live.Done || live.PendingAttackerHits > 0 || live.Battle.Type != SeaBattle {
		return false
	}
	return len(getSubmarines(live.Attackers)) > 0 && !hasDestroyer(live.Defenders)
}

// SubmergeSubmarines takes the attacker's submarines out of the fight. They
// stay in the sea zone, neither attacking nor lost; if nothing else is
// attacking, the battle ends with the defenders still there.
func (gc *GameController) SubmergeSubmarines(territoryName string) (*LiveBattle, error) {
	live, ok := gc.LiveBattles[territoryName]
	if !ok {
		return nil, fmt.Errorf("no battle in progress in %s", territoryName)
	}
	if !live.CanSubmerge() {
		return live, fmt.Errorf("the submarines in %s cannot submerge now", territoryName)
	}
	subs := getSubmarines(live.Attackers)
	live.Attackers = RemoveCasualties(live.Attackers, subs)
	gone := make(map[int]bool, len(subs))
	for _, sub := range subs {
		gone[sub.ID] = true
	}
	kept := make([]int, 0, len(live.Battle.AttackingPieceIDs))
	for _, id := range live.Battle.AttackingPieceIDs {
		if !gone[id] {
			kept = append(kept, id)
		}
	}
	live.Battle.AttackingPieceIDs = kept
	live.Log = append(live.Log, BattleRoundReport{
		Round: live.Round,
		Notes: []string{fmt.Sprintf("%d submarine(s) submerged", len(subs))},
	})
	if len(live.Attackers) == 0 {
		live.Retreated = true
		gc.concludeLive(live)
	}
	return live, nil
}

// FinishBattle fights a live battle to its end with the engine choosing the
// attacker's casualties from here on.
func (gc *GameController) FinishBattle(territoryName string) (*LiveBattle, error) {
	live, ok := gc.LiveBattles[territoryName]
	if !ok {
		return nil, fmt.Errorf("no battle in progress in %s", territoryName)
	}
	live.AutoCasualties = true
	if live.PendingAttackerHits > 0 {
		lost := SelectCasualties(live.Attackers, live.PendingAttackerHits)
		live.AttackerCasualties = append(live.AttackerCasualties, lost...)
		live.Attackers = RemoveCasualties(live.Attackers, lost)
		if len(live.Log) > 0 {
			last := &live.Log[len(live.Log)-1]
			last.AttackerLosses = append(last.AttackerLosses, names(lost)...)
		}
		live.PendingAttackerHits = 0
		gc.settleIfDecided(live)
	}
	for !live.Done && live.Round < 100 {
		if _, err := gc.FightRound(territoryName); err != nil {
			return live, err
		}
	}
	if !live.Done {
		gc.concludeLive(live)
	}
	return live, nil
}

// settleIfDecided ends the battle when a side has nothing left.
func (gc *GameController) settleIfDecided(live *LiveBattle) {
	if live.Done {
		return
	}
	if len(live.Attackers) == 0 || len(live.Defenders) == 0 {
		gc.concludeLive(live)
	}
}

// concludeLive applies the outcome to the board exactly as ResolveBattle
// does: casualties leave, a won land battle with troops standing captures,
// damage is repaired, and the battle stops being pending.
func (gc *GameController) concludeLive(live *LiveBattle) {
	territoryName := live.Territory
	territory := gc.Game.Board[territoryName]
	attacker := gc.Game.Players[live.Battle.AttackerID]

	result := &BattleResult{
		Rounds:             live.Round,
		AttackerCasualties: append(append([]*models.Piece{}, live.AttackerCasualties...), live.drowned...),
		DefenderCasualties: live.DefenderCasualties,
		AttackersRemaining: live.Attackers,
		DefendersRemaining: live.Defenders,
		AttackerRetreated:  live.Retreated && len(live.Attackers) == 0,
	}
	switch {
	case len(live.Defenders) == 0 && len(live.Attackers) > 0:
		result.AttackerWins = true
	default:
		result.DefenderWins = true
	}

	RemoveArtillerySupport(live.boosted)
	RepairDamagedUnits(live.Attackers)
	RepairDamagedUnits(live.Defenders)

	for _, casualty := range live.AttackerCasualties {
		gc.removePieceFromBoard(casualty, territoryName)
	}
	for _, casualty := range live.DefenderCasualties {
		gc.removePieceFromBoard(casualty, territoryName)
	}

	if result.AttackerWins && territory != nil && territory.Terrain == models.Land &&
		anyLandUnit(result.AttackersRemaining) {
		_ = gc.CaptureTerritory(territoryName, attacker.Name)
	}
	// Structures survive and change hands with the territory.
	if territory != nil && territory.Owner != nil {
		units := gc.Game.Units()
		for _, id := range territory.Pieces {
			if piece := gc.Game.Pieces[id]; piece != nil && units.For(piece).IsStructure {
				piece.Owner = territory.Owner
			}
		}
	}

	live.Result = result
	live.Done = true
	delete(gc.PendingBattles, territoryName)
	delete(gc.LiveBattles, territoryName)
}

func names(pieces []*models.Piece) []string {
	out := make([]string, 0, len(pieces))
	for _, piece := range pieces {
		if piece != nil {
			out = append(out, piece.Name)
		}
	}
	return out
}

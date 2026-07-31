// Package engine drives a game forward.
//
// It exists because the same ~100 lines of phase-advance logic were written
// twice -- once in the terminal front end and once in the web handlers -- and
// had already drifted apart. Rules belong in one place; a front end should
// decide what to show and what to ask, not what the rules are.
//
// Three seams:
//
//	Decider   supplies choices the rules need (confirm, retreat, casualties).
//	Observer  receives what happened, in structured form, for display.
//	Driver    owns the sequence and is the only implementation of it.
//
// Nothing here formats a string for a particular medium. The terminal prints;
// the web server marshals; the transcript records. All three see the same events.
package engine

import (
	"fmt"

	"boardgame/game"
	"boardgame/models"
)

// PurchasedUnit is one unit type bought this phase.
type PurchasedUnit struct {
	Type     string
	Quantity int
	Cost     int
}

// PhaseResult describes a completed phase transition.
type PhaseResult struct {
	From, To     models.Phase
	Power        string
	TurnAdvanced bool
	NewPower     string
	NewTurn      int

	UnitsPurchased  []PurchasedUnit
	MovesExecuted   int
	BattlesCreated  []string
	IncomeCollected int
	RemainingIPCs   int
}

// Blocker is a rule-level refusal to advance: something the player must deal
// with first. It is not an error -- nothing went wrong, the phase simply is not
// finished.
type Blocker struct {
	Code   string
	Detail string
}

func (b Blocker) Error() string { return b.Detail }

// Blockers is the set of reasons a phase cannot end yet.
type Blockers []Blocker

func (b Blockers) Error() string {
	if len(b) == 0 {
		return ""
	}
	if len(b) == 1 {
		return b[0].Detail
	}
	out := ""
	for i, blocker := range b {
		if i > 0 {
			out += "; "
		}
		out += blocker.Detail
	}
	return out
}

// Decider answers questions the rules cannot answer themselves.
//
// The terminal prompts, the web server replies from the request, and an NPC
// decides for itself -- but the rules ask the same question of all three.
type Decider interface {
	// ConfirmAdvance is asked when a phase can be ended but probably should not
	// be. Returning false leaves the phase where it is.
	ConfirmAdvance(warnings []string) bool
}

// AutoConfirm accepts every advance. Used for NPC powers and tests.
type AutoConfirm struct{}

func (AutoConfirm) ConfirmAdvance([]string) bool { return true }

// Observer receives progress events.
type Observer interface {
	PhaseCompleted(result PhaseResult)
	BattleResolved(territory string, result *game.BattleResult)
	TurnStarted(turn int, power string)
}

// Driver is the single implementation of the turn sequence.
type Driver struct {
	Controller *game.GameController

	// Deciders is keyed by power name. A power with no decider auto-confirms,
	// which is what an NPC wants.
	Deciders map[string]Decider

	Observers []Observer
}

// New builds a driver over a controller.
func New(controller *game.GameController) *Driver {
	return &Driver{
		Controller: controller,
		Deciders:   make(map[string]Decider),
	}
}

// AddObserver registers an observer.
func (d *Driver) AddObserver(observer Observer) {
	if observer != nil {
		d.Observers = append(d.Observers, observer)
	}
}

func (d *Driver) deciderFor(power string) Decider {
	if decider, ok := d.Deciders[power]; ok && decider != nil {
		return decider
	}
	return AutoConfirm{}
}

// Warnings lists things worth mentioning before a phase ends. None of them stop
// the phase; they are advice.
func (d *Driver) Warnings() []string {
	warnings := make([]string, 0)

	player, err := d.Controller.GetCurrentPlayer()
	if err != nil || player == nil {
		return warnings
	}

	switch d.Controller.Game.CurrentPhase {
	case models.PurchasePhase:
		if player.IPCs > 10 {
			warnings = append(warnings,
				fmt.Sprintf("You have %d unspent IPCs", player.IPCs))
		}
	case models.CombatMovePhase:
		if len(d.Controller.GetPlannedMoves()) == 0 {
			warnings = append(warnings,
				"No combat moves planned - you won't attack any territories")
		}
	case models.MobilizePhase:
		if pending := d.Controller.Game.PurchasedUnits[player.Name]; len(pending) > 0 {
			warnings = append(warnings,
				fmt.Sprintf("You have %d unit(s) not yet placed", len(pending)))
		}
	}
	return warnings
}

// Blockers lists rule reasons the current phase cannot end.
func (d *Driver) Blockers() Blockers {
	var blockers Blockers

	player, err := d.Controller.GetCurrentPlayer()
	if err != nil || player == nil {
		return blockers
	}

	switch d.Controller.Game.CurrentPhase {
	case models.ConductCombatPhase:
		if n := len(d.Controller.PendingBattles); n > 0 {
			blockers = append(blockers, Blocker{
				Code:   "unresolved_battles",
				Detail: fmt.Sprintf("%d battle(s) still to resolve", n),
			})
		}
	case models.MobilizePhase:
		if n := len(d.Controller.Game.PurchasedUnits[player.Name]); n > 0 {
			blockers = append(blockers, Blocker{
				Code:   "unplaced_units",
				Detail: fmt.Sprintf("%d unit(s) still to place", n),
			})
		}
	}
	return blockers
}

// AdvancePhase ends the current phase and begins the next.
//
// Returns Blockers when the phase is not finished, which callers should present
// rather than treat as failure. A nil PhaseResult with nil error means the
// player declined to advance.
func (d *Driver) AdvancePhase() (*PhaseResult, error) {
	controller := d.Controller
	from := controller.Game.CurrentPhase

	player, err := controller.GetCurrentPlayer()
	if err != nil {
		return nil, err
	}

	if blockers := d.Blockers(); len(blockers) > 0 {
		return nil, blockers
	}

	if warnings := d.Warnings(); len(warnings) > 0 {
		if !d.deciderFor(player.Name).ConfirmAdvance(warnings) {
			return nil, nil
		}
	}

	result := &PhaseResult{From: from, Power: player.Name}

	switch from {
	case models.PurchasePhase:
		for unitType, units := range groupPending(controller.Game.PurchasedUnits[player.Name]) {
			result.UnitsPurchased = append(result.UnitsPurchased, PurchasedUnit{
				Type: unitType, Quantity: units.count, Cost: units.cost,
			})
		}

	case models.CombatMovePhase:
		result.MovesExecuted = len(controller.GetPlannedMoves())
		if err := controller.ExecuteCombatMoves(); err != nil {
			return nil, fmt.Errorf("executing combat moves: %w", err)
		}
		for territory := range controller.PendingBattles {
			result.BattlesCreated = append(result.BattlesCreated, territory)
		}

	case models.NoncombatMovePhase:
		result.MovesExecuted = len(controller.GetPlannedMoves())
		if err := controller.ExecuteNoncombatMoves(); err != nil {
			return nil, fmt.Errorf("executing noncombat moves: %w", err)
		}

	case models.CollectIncomePhase:
		income, _ := controller.CalculateIncome(player.Name)
		if err := controller.CollectIncome(); err != nil {
			return nil, fmt.Errorf("collecting income: %w", err)
		}
		result.IncomeCollected = income
	}

	if err := controller.AdvancePhase(); err != nil {
		return nil, err
	}

	result.To = controller.Game.CurrentPhase
	result.NewPower = controller.Game.CurrentPower
	result.NewTurn = controller.Game.Turn
	result.TurnAdvanced = result.NewPower != result.Power
	result.RemainingIPCs = player.IPCs

	for _, observer := range d.Observers {
		observer.PhaseCompleted(*result)
		if result.TurnAdvanced {
			observer.TurnStarted(result.NewTurn, result.NewPower)
		}
	}
	return result, nil
}

type pendingGroup struct {
	count int
	cost  int
}

func groupPending(units []*models.PendingUnit) map[string]pendingGroup {
	out := make(map[string]pendingGroup)
	for _, unit := range units {
		group := out[unit.Type]
		group.count++
		group.cost += unit.Cost
		out[unit.Type] = group
	}
	return out
}

// ResolveBattle fights one pending battle and reports the result.
func (d *Driver) ResolveBattle(territory string) (*game.BattleResult, error) {
	result, err := d.Controller.ResolveBattle(territory, nil)
	if err != nil {
		return nil, err
	}
	for _, observer := range d.Observers {
		observer.BattleResolved(territory, result)
	}
	return result, nil
}

// ResolveAllBattles fights every pending battle.
//
// The pending set is snapshotted first: resolving a battle deletes it from the
// map, and one battle can create or capture territory that touches others.
func (d *Driver) ResolveAllBattles() ([]*game.BattleResult, error) {
	territories := make([]string, 0, len(d.Controller.PendingBattles))
	for territory := range d.Controller.PendingBattles {
		territories = append(territories, territory)
	}

	results := make([]*game.BattleResult, 0, len(territories))
	for _, territory := range territories {
		if _, still := d.Controller.PendingBattles[territory]; !still {
			continue
		}
		result, err := d.ResolveBattle(territory)
		if err != nil {
			return results, fmt.Errorf("resolving %s: %w", territory, err)
		}
		results = append(results, result)
	}
	return results, nil
}

// RunNPCTurn plays one NPC power's whole turn.
//
// A transcript is always supplied. The web server used to pass nil here and the
// AI's first log call dereferenced it, so triggering an NPC turn from the
// browser was a guaranteed panic.
func (d *Driver) RunNPCTurn(power string, transcript *game.GameTranscript) error {
	player, ok := d.Controller.Game.Players[power]
	if !ok {
		return fmt.Errorf("unknown power %q", power)
	}
	if !player.TakesTurns {
		return fmt.Errorf("%s does not take turns", power)
	}
	if transcript == nil {
		transcript = game.NewGameTranscript("NPC turn")
	}

	npc := game.NewNPCAIPlayer(power, "normal")
	return npc.TakeTurn(d.Controller, transcript)
}

// RunUntilHumanTurn plays NPC powers until it is the human's turn again.
//
// maxTurns bounds the walk so a rule bug cannot hang the caller: without it a
// power that never finishes its turn spins forever, which for the web server
// means a wedged request holding the session lock.
func (d *Driver) RunUntilHumanTurn(human string, maxTurns int) ([]string, error) {
	played := make([]string, 0)

	for i := 0; i < maxTurns; i++ {
		current := d.Controller.Game.CurrentPower
		if current == human || current == "" {
			return played, nil
		}
		player, ok := d.Controller.Game.Players[current]
		if !ok || !player.TakesTurns {
			return played, fmt.Errorf("current power %q cannot take a turn", current)
		}

		if err := d.RunNPCTurn(current, nil); err != nil {
			return played, fmt.Errorf("%s: %w", current, err)
		}
		played = append(played, current)

		if d.Controller.Game.CurrentPower == current {
			return played, fmt.Errorf("%s did not end its turn", current)
		}
	}
	return played, fmt.Errorf("gave up after %d NPC turns without reaching %s", maxTurns, human)
}

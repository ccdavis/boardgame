package engine

import (
	"errors"
	"testing"

	"boardgame/game"
	"boardgame/models"
)

func testGame(t *testing.T) *models.Game {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "USSR"}
	for _, name := range g.PlayerOrder {
		player := g.GetOrCreatePlayer(name)
		player.IPCs = 30
		player.TakesTurns = true
	}
	g.Players["Germany"].Side = "Axis"
	g.Players["USSR"].Side = "Allies"

	g.AddTerritory("Germany", models.Land, "Germany", 10)
	g.AddTerritory("Russia", models.Land, "USSR", 8)
	g.ConnectTerritories("Germany", "Russia")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)
	if err := g.PlacePieces("Germany", "factory", 1); err != nil {
		t.Fatalf("placing factory: %v", err)
	}
	return g
}

func newDriver(t *testing.T) *Driver {
	t.Helper()
	controller := game.NewGameController(testGame(t))
	if err := controller.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	return New(controller)
}

func TestDriver_AdvancePhaseReportsTheTransition(t *testing.T) {
	driver := newDriver(t)

	result, err := driver.AdvancePhase()
	if err != nil {
		t.Fatalf("AdvancePhase: %v", err)
	}
	if result == nil {
		t.Fatal("expected a result")
	}
	if result.From != models.PurchasePhase {
		t.Errorf("From = %v, want Purchase", result.From)
	}
	if result.To != models.CombatMovePhase {
		t.Errorf("To = %v, want Combat Move", result.To)
	}
	if result.Power != "Germany" {
		t.Errorf("Power = %q, want Germany", result.Power)
	}
}

// Unplaced units block the Mobilize phase, and a blocker is not an error: the
// caller should show it, not fail on it.
func TestDriver_UnplacedUnitsBlockMobilize(t *testing.T) {
	driver := newDriver(t)

	if err := driver.Controller.PurchaseUnit("infantry", 2); err != nil {
		t.Fatalf("purchasing: %v", err)
	}
	driver.Controller.Game.CurrentPhase = models.MobilizePhase

	_, err := driver.AdvancePhase()
	if err == nil {
		t.Fatal("expected the phase to be blocked")
	}

	var blockers Blockers
	if !errors.As(err, &blockers) {
		t.Fatalf("expected Blockers, got %T: %v", err, err)
	}
	if len(blockers) != 1 || blockers[0].Code != "unplaced_units" {
		t.Errorf("unexpected blockers: %+v", blockers)
	}
}

// A decider that declines leaves the phase exactly where it was.
func TestDriver_DecliningLeavesThePhaseAlone(t *testing.T) {
	driver := newDriver(t)
	driver.Deciders["Germany"] = refuse{}

	before := driver.Controller.Game.CurrentPhase
	result, err := driver.AdvancePhase()
	if err != nil {
		t.Fatalf("AdvancePhase: %v", err)
	}
	if result != nil {
		t.Error("expected no result when the advance is declined")
	}
	if driver.Controller.Game.CurrentPhase != before {
		t.Errorf("phase moved to %v despite being declined", driver.Controller.Game.CurrentPhase)
	}
}

type refuse struct{}

func (refuse) ConfirmAdvance([]string) bool { return false }

func TestDriver_WarningsMentionUnspentIPCs(t *testing.T) {
	driver := newDriver(t)

	warnings := driver.Warnings()
	if len(warnings) == 0 {
		t.Fatal("expected a warning about the 30 unspent IPCs")
	}
}

// Neutral owns territory but is not a playing power, so it must be refused
// rather than quietly given a turn.
func TestDriver_RefusesToRunANonPlayingPower(t *testing.T) {
	driver := newDriver(t)
	neutral := driver.Controller.Game.GetOrCreatePlayer("Neutral")
	neutral.TakesTurns = false

	if err := driver.RunNPCTurn("Neutral", nil); err == nil {
		t.Error("expected an error running a turn for a power that does not take turns")
	}
}

// The web server used to call TakeTurn with a nil transcript, and the AI logs
// before it does anything else -- so this path was a guaranteed panic.
func TestDriver_NPCTurnWithNoTranscriptDoesNotPanic(t *testing.T) {
	driver := newDriver(t)
	driver.Controller.Game.CurrentPower = "Germany"

	if err := driver.RunNPCTurn("Germany", nil); err != nil {
		t.Fatalf("RunNPCTurn: %v", err)
	}
}

func TestDriver_ObserverSeesPhaseCompletion(t *testing.T) {
	driver := newDriver(t)
	spy := &recorder{}
	driver.AddObserver(spy)

	if _, err := driver.AdvancePhase(); err != nil {
		t.Fatalf("AdvancePhase: %v", err)
	}
	if len(spy.phases) != 1 {
		t.Fatalf("observer saw %d phase completions, want 1", len(spy.phases))
	}
	if spy.phases[0].From != models.PurchasePhase {
		t.Errorf("observed From = %v, want Purchase", spy.phases[0].From)
	}
}

type recorder struct {
	phases []PhaseResult
}

func (r *recorder) PhaseCompleted(result PhaseResult)         { r.phases = append(r.phases, result) }
func (r *recorder) BattleResolved(string, *game.BattleResult) {}
func (r *recorder) TurnStarted(int, string)                   {}

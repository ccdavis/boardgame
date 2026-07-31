package game

import (
	"boardgame/models"
	"fmt"
	"os"
	"strings"
	"time"
)

// GameTranscript records all actions in a game
type GameTranscript struct {
	Entries   []TranscriptEntry
	StartTime time.Time
	GameTitle string
}

// TranscriptEntry represents a single log entry
type TranscriptEntry struct {
	Turn      int
	Player    string
	Phase     models.Phase
	Action    string
	Timestamp time.Time
}

// NewGameTranscript creates a new transcript
func NewGameTranscript(title string) *GameTranscript {
	return &GameTranscript{
		Entries:   make([]TranscriptEntry, 0),
		StartTime: time.Now(),
		GameTitle: title,
	}
}

// Log adds a generic entry to the transcript.
//
// A nil transcript is a no-op rather than a panic. Recording is optional -- the
// web server passed nil for a long time and the first Log call took the whole
// request down with a nil dereference. Every other Log* method routes through
// here, so guarding once covers them all.
func (t *GameTranscript) Log(turn int, player string, phase models.Phase, action string) {
	if t == nil {
		return
	}
	entry := TranscriptEntry{
		Turn:      turn,
		Player:    player,
		Phase:     phase,
		Action:    action,
		Timestamp: time.Now(),
	}
	t.Entries = append(t.Entries, entry)
}

// LogTurnStart logs the beginning of a new turn
func (t *GameTranscript) LogTurnStart(turn int, player string) {
	action := fmt.Sprintf("═══ TURN %d - %s ═══", turn, player)
	t.Log(turn, player, models.PurchasePhase, action)
}

// LogPhaseStart logs the start of a phase
func (t *GameTranscript) LogPhaseStart(player string, phase models.Phase) {
	action := fmt.Sprintf("▶ Phase: %s", phase.String())
	t.Log(0, player, phase, action)
}

// LogPurchase logs unit purchases
func (t *GameTranscript) LogPurchase(player string, purchases map[string]int, totalCost int) {
	// Sorted so the same purchases always read the same way; map order would
	// make two identical games produce different transcripts.
	items := make([]string, 0, len(purchases))
	for _, unitType := range sortedWants(purchases) {
		items = append(items, fmt.Sprintf("%dx %s", purchases[unitType], unitType))
	}
	action := fmt.Sprintf("Purchased: %s (Cost: %d IPCs)", strings.Join(items, ", "), totalCost)
	t.Log(0, player, models.PurchasePhase, action)
}

// LogMove logs a unit movement
func (t *GameTranscript) LogMove(player string, unitType string, from string, to string, moveType string) {
	action := fmt.Sprintf("Move %s: %s → %s (%s)", unitType, from, to, moveType)
	phase := models.CombatMovePhase
	if moveType == "noncombat" {
		phase = models.NoncombatMovePhase
	}
	t.Log(0, player, phase, action)
}

// LogBattleStart logs the beginning of a battle
func (t *GameTranscript) LogBattleStart(territory string) {
	action := fmt.Sprintf("⚔ Battle begins in %s", territory)
	t.Log(0, "", models.ConductCombatPhase, action)
}

// LogBattleResult logs the outcome of a battle
func (t *GameTranscript) LogBattleResult(territory string, result *BattleResult) {
	winner := "Stalemate"
	if result.AttackerWins {
		winner = "Attacker Victory"
	} else if result.DefenderWins {
		winner = "Defender Victory"
	}

	// The outcome describes what actually happened. This used to read
	// "<territory> captured!" for every battle, producing entries like
	// "France captured! Defender Victory after 3 rounds".
	var outcome string
	switch {
	case result.AttackerWins:
		outcome = fmt.Sprintf("%s captured", territory)
	case result.DefenderWins:
		outcome = fmt.Sprintf("%s held", territory)
	case result.AttackerRetreated:
		outcome = fmt.Sprintf("attack on %s broken off", territory)
	default:
		outcome = fmt.Sprintf("fighting in %s ended inconclusively", territory)
	}

	if n := len(result.BombardmentHits); n > 0 {
		t.Log(0, "", models.ConductCombatPhase,
			fmt.Sprintf("  shore bombardment: %d hit(s) before the assault", n))
	}

	action := fmt.Sprintf("  %s - %s after %d rounds (Att casualties: %d, Def casualties: %d)",
		outcome, winner, result.Rounds, len(result.AttackerCasualties), len(result.DefenderCasualties))

	t.Log(0, "", models.ConductCombatPhase, action)
}

// LogMobilize logs unit mobilization
func (t *GameTranscript) LogMobilize(player string, unitType string, territory string) {
	action := fmt.Sprintf("Mobilized %s at %s", unitType, territory)
	t.Log(0, player, models.MobilizePhase, action)
}

// LogIncomeCollection logs income collection
func (t *GameTranscript) LogIncomeCollection(player string, income int, totalIPCs int) {
	action := fmt.Sprintf("Collected %d IPCs (Total: %d IPCs)", income, totalIPCs)
	t.Log(0, player, models.CollectIncomePhase, action)
}

// LogAction logs a generic action
func (t *GameTranscript) LogAction(player string, action string) {
	t.Log(0, player, models.PurchasePhase, action)
}

// LogVictory logs a victory condition being met
func (t *GameTranscript) LogVictory(winner string, reason string) {
	action := fmt.Sprintf("🏆 VICTORY! %s wins! (%s)", winner, reason)
	t.Log(0, winner, models.CollectIncomePhase, action)
}

// LogGameState logs current game state summary
func (t *GameTranscript) LogGameState(game *models.Game) {
	axisVC, alliesVC := game.CountVictoryCities()
	action := fmt.Sprintf("Game State: Turn %d | Victory Cities: Axis=%d, Allies=%d", game.Turn, axisVC, alliesVC)
	t.Log(game.Turn, "", models.PurchasePhase, action)
}

// LogPlayerState logs a player's current state
func (t *GameTranscript) LogPlayerState(player *models.Player) {
	action := fmt.Sprintf("%s: %d IPCs, %d territories", player.Name, player.IPCs, len(player.Territories))
	t.Log(0, player.Name, models.PurchasePhase, action)
}

// String returns the full transcript as a formatted string
func (t *GameTranscript) String() string {
	if t == nil {
		return ""
	}
	var builder strings.Builder

	builder.WriteString("════════════════════════════════════════════════════════\n")
	builder.WriteString(fmt.Sprintf("  %s\n", t.GameTitle))
	builder.WriteString(fmt.Sprintf("  Started: %s\n", t.StartTime.Format("2006-01-02 15:04:05")))
	builder.WriteString("════════════════════════════════════════════════════════\n\n")

	for _, entry := range t.Entries {
		timestamp := entry.Timestamp.Sub(t.StartTime).Round(time.Millisecond)

		// Format based on entry type
		if strings.HasPrefix(entry.Action, "═══") {
			builder.WriteString("\n")
			builder.WriteString(entry.Action)
			builder.WriteString("\n")
		} else if strings.HasPrefix(entry.Action, "▶") {
			builder.WriteString(fmt.Sprintf("  %s\n", entry.Action))
		} else if strings.HasPrefix(entry.Action, "⚔") {
			builder.WriteString(fmt.Sprintf("    %s\n", entry.Action))
		} else if strings.HasPrefix(entry.Action, "  ") {
			// Battle result (already indented)
			builder.WriteString(fmt.Sprintf("    %s\n", entry.Action))
		} else if strings.HasPrefix(entry.Action, "🏆") {
			builder.WriteString("\n")
			builder.WriteString("════════════════════════════════════════════════════════\n")
			builder.WriteString(fmt.Sprintf("  %s\n", entry.Action))
			builder.WriteString("════════════════════════════════════════════════════════\n")
		} else if strings.HasPrefix(entry.Action, "Game State:") {
			builder.WriteString(fmt.Sprintf("    [%v] %s\n", timestamp, entry.Action))
		} else {
			// Regular action
			builder.WriteString(fmt.Sprintf("    • %s\n", entry.Action))
		}
	}

	duration := time.Since(t.StartTime)
	builder.WriteString(fmt.Sprintf("\nGame Duration: %v\n", duration.Round(time.Millisecond)))
	builder.WriteString(fmt.Sprintf("Total Entries: %d\n", len(t.Entries)))

	return builder.String()
}

// SaveToFile writes the transcript to a file.
//
// This used to return nil without writing anything, so every caller believed it
// had saved a transcript that was never on disk.
func (t *GameTranscript) SaveToFile(filename string) error {
	if t == nil {
		return fmt.Errorf("no transcript to save")
	}
	if err := os.WriteFile(filename, []byte(t.String()), 0o644); err != nil {
		return fmt.Errorf("writing transcript to %s: %w", filename, err)
	}
	return nil
}

// GetPhaseEntries returns all entries for a specific phase
func (t *GameTranscript) GetPhaseEntries(phase models.Phase) []TranscriptEntry {
	entries := make([]TranscriptEntry, 0)
	for _, entry := range t.Entries {
		if entry.Phase == phase {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetPlayerEntries returns all entries for a specific player
func (t *GameTranscript) GetPlayerEntries(player string) []TranscriptEntry {
	entries := make([]TranscriptEntry, 0)
	for _, entry := range t.Entries {
		if entry.Player == player {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetTurnEntries returns all entries for a specific turn
func (t *GameTranscript) GetTurnEntries(turn int) []TranscriptEntry {
	entries := make([]TranscriptEntry, 0)
	for _, entry := range t.Entries {
		if entry.Turn == turn {
			entries = append(entries, entry)
		}
	}
	return entries
}

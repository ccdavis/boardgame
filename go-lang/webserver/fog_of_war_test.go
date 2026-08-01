package webserver

import (
	"strings"
	"testing"

	"boardgame/game"
)

// An enemy viewer must not read operation details; an allied viewer must.
func TestTranscriptLines_RedactsSecretsFromTheEnemy(t *testing.T) {
	transcript := game.NewGameTranscript("UK's turn")
	transcript.LogAction("UK", "Purchased: 3x infantry (Cost: 9 IPCs)")
	transcript.LogSecretAction("UK", "new Operation OVERLORD (plan 1): take Western Europe")
	transcript.LogSecretAction("UK", "new Operation TORCH (plan 2): take Algeria")
	transcript.LogAction("UK", "Collected 30 IPCs (Total: 42 IPCs)")

	enemy := transcriptLines(transcript.Entries, false, "UK")
	joined := strings.Join(enemy, "\n")
	if strings.Contains(joined, "OVERLORD") || strings.Contains(joined, "Algeria") {
		t.Errorf("enemy transcript leaks operation details:\n%s", joined)
	}
	if !strings.Contains(joined, "2 secret operation(s)") {
		t.Errorf("enemy transcript does not count the secret operations:\n%s", joined)
	}
	if !strings.Contains(joined, "Purchased") || !strings.Contains(joined, "Collected") {
		t.Errorf("enemy transcript lost public entries:\n%s", joined)
	}

	ally := transcriptLines(transcript.Entries, true, "UK")
	joined = strings.Join(ally, "\n")
	if !strings.Contains(joined, "OVERLORD") || !strings.Contains(joined, "TORCH") {
		t.Errorf("allied transcript must show the side's own operations:\n%s", joined)
	}
	if strings.Contains(joined, "secret operation(s)") {
		t.Errorf("allied transcript should not be redacted:\n%s", joined)
	}
}

// A transcript with no secrets passes through untouched.
func TestTranscriptLines_NoSecretsNoNotice(t *testing.T) {
	transcript := game.NewGameTranscript("test")
	transcript.LogAction("Japan", "No purchases made")

	lines := transcriptLines(transcript.Entries, false, "Japan")
	if len(lines) != 1 || strings.Contains(lines[0], "secret") {
		t.Errorf("unexpected redaction of a public transcript: %v", lines)
	}
}

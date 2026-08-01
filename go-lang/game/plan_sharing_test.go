package game

import (
	"testing"

	"boardgame/models"
)

func sharingBook() *PlanBook {
	book := NewPlanBook()
	book.sideOf = func(power string) string {
		switch power {
		case "UK", "USA", "USSR":
			return "Allies"
		case "Germany", "Japan":
			return "Axis"
		}
		return ""
	}
	return book
}

// Powers on one side share their operational claims; the enemy sees nothing.
func TestSideTargets_SharedWithinASide(t *testing.T) {
	book := sharingBook()
	book.Add(&AmphibiousPlan{Power: "UK", Target: "Midway Island", State: PlanForming})

	if !book.SideTargets("USA")["Midway Island"] {
		t.Error("the USA does not see its ally's claim on Midway Island")
	}
	if book.SideTargets("Germany")["Midway Island"] {
		t.Error("Germany can read the Allied plan book")
	}
	if !book.SideTargets("UK")["Midway Island"] {
		t.Error("a power lost sight of its own claim")
	}
}

// A target judged hopeless cools off, then comes back on the table -- the
// grand invasion is throttled, never forbidden.
func TestHopelessTargetsCoolOffThenReturn(t *testing.T) {
	book := sharingBook()
	book.RecordHopeless("Germany", "Eastern US", 10)

	if !book.CoolingOff("Germany", "Eastern US", 11) {
		t.Error("freshly hopeless target is not cooling off")
	}
	if book.CoolingOff("Germany", "Western US", 11) {
		t.Error("cooldown bled onto a target never judged")
	}
	if book.CoolingOff("Japan", "Eastern US", 11) {
		t.Error("cooldown bled onto another power")
	}
	if book.CoolingOff("Germany", "Eastern US", 10+hopelessRetryCooldown) {
		t.Error("the cooling-off never ends; hopeless targets must return eventually")
	}
}

// Only REVEALED enemy operations inform garrisons, and only for their target.
func TestRevealedThreatsAgainst(t *testing.T) {
	g := models.NewGame()
	g.AddTerritory("Midway Island", models.Land, "USA", 2)
	g.AddTerritory("Tokyo", models.Land, "Japan", 8)
	usa := g.Players["USA"]
	japan := g.Players["Japan"]
	usa.Side = "Allies"
	japan.Side = "Axis"

	book := sharingBook()
	hidden := book.Add(&AmphibiousPlan{
		Power: "Japan", Target: "Midway Island", State: PlanForming, WantTroops: 6,
	})

	if threats := book.RevealedThreatsAgainst(g, usa); len(threats) != 0 {
		t.Errorf("unrevealed plan produced threats %v; the fog of war is gone", threats)
	}

	hidden.Revealed = true
	threats := book.RevealedThreatsAgainst(g, usa)
	if threats["Midway Island"] == 0 {
		t.Error("revealed plan produced no threat against its target")
	}
	if book.RevealedThreatsAgainst(g, japan)["Midway Island"] != 0 {
		t.Error("a power is threatened by its own operation")
	}
}

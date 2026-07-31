package game

import (
	"testing"

	"boardgame/models"
)

// A board where the Axis power is heavily outproduced: Germany holds 6
// production against the Allies' 24.
func pressureBoard(t *testing.T) (*models.Game, *GameController) {
	t.Helper()

	g := models.NewGame()
	g.PlayerOrder = []string{"Germany", "USSR"}
	germany := g.GetOrCreatePlayer("Germany")
	ussr := g.GetOrCreatePlayer("USSR")
	germany.Side, ussr.Side = "Axis", "Allies"
	germany.TakesTurns, ussr.TakesTurns = true, true

	g.AddTerritory("Reich", models.Land, "Germany", 6)
	// Padding so Germany is not "losing badly" (a separate desperation bonus
	// fires below five territories and would mask the clock's effect).
	for _, name := range []string{"Marsh", "Forest", "Hills", "Coastline"} {
		g.AddTerritory(name, models.Land, "Germany", 0)
		g.ConnectTerritories("Reich", name)
	}
	g.AddTerritory("Border", models.Land, "USSR", 4)
	g.AddTerritory("Steppe", models.Land, "USSR", 10)
	g.AddTerritory("Urals", models.Land, "USSR", 10)
	g.ConnectTerritories("Reich", "Border")
	g.ConnectTerritories("Border", "Steppe")
	g.ConnectTerritories("Steppe", "Urals")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("armor", models.Land, 2, 3, 2, 5)

	gc := NewGameController(g)
	if err := gc.StartGame(); err != nil {
		t.Fatalf("starting game: %v", err)
	}
	return g, gc
}

func TestTimePressure_MeasuresTheProductionRace(t *testing.T) {
	g, _ := pressureBoard(t)

	// Germany: 6 production against 24 -> pressure 4.0. USSR: the mirror.
	if p := timePressure(g, g.Players["Germany"]); p < 3.9 || p > 4.1 {
		t.Errorf("Germany's pressure = %.2f, want 4.0", p)
	}
	if p := timePressure(g, g.Players["USSR"]); p < 0.2 || p > 0.3 {
		t.Errorf("USSR's pressure = %.2f, want 0.25", p)
	}
}

func TestPressureThreshold_ShiftsTheBarBothWays(t *testing.T) {
	near := func(a, b float64) bool { return a-b < 0.001 && b-a < 0.001 }

	// Outproduced: the bar drops, but never below the floor.
	if got := pressureThreshold(0.6, 2.0); !near(got, 0.4) {
		t.Errorf("outproduced threshold = %.2f, want 0.40", got)
	}
	if got := pressureThreshold(0.4, 4.0); !near(got, 0.35) {
		t.Errorf("floored threshold = %.2f, want 0.35", got)
	}
	// Winning the race: the bar rises.
	if got := pressureThreshold(0.6, 0.5); !near(got, 0.65) {
		t.Errorf("favoured threshold = %.2f, want 0.65", got)
	}
	// An even race changes nothing.
	if got := pressureThreshold(0.6, 1.0); !near(got, 0.6) {
		t.Errorf("even-race threshold = %.2f, want 0.60", got)
	}
}

// The outproduced side takes a fight the favoured side declines.
//
// The chosen force here estimates at ~0.52: below the normal 0.6 bar, above
// the outproduced side's lowered one. Waiting would only let the defenders
// multiply, so the clock says go.
func TestPressure_OutproducedSideAcceptsThinnerOdds(t *testing.T) {
	attack := func(t *testing.T, germanProduction int) int {
		t.Helper()
		g, gc := pressureBoard(t)
		g.Board["Reich"].Production = germanProduction

		g.PlacePieces("Reich", "armor", 10)    // attack 30; the chosen 70% is 21
		g.PlacePieces("Border", "infantry", 10) // defence 20; chosen ratio 1.05 -> ~0.52

		g.CurrentPower = "Germany"
		g.CurrentPhase = models.CombatMovePhase
		npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
		if err := npc.CombatMovePhase(gc, NewGameTranscript("t")); err != nil {
			t.Fatalf("combat move: %v", err)
		}
		if battle, ok := gc.PendingBattles["Border"]; ok {
			return len(battle.AttackingPieceIDs)
		}
		return 0
	}

	// Outproduced (6 vs 24): the attack goes in.
	if n := attack(t, 6); n == 0 {
		t.Error("an outproduced Germany declined the attack the clock demands")
	}
	// Comfortably out-producing (production 60 vs 24): the same fight is
	// declined -- patience will make it a sure one.
	if n := attack(t, 60); n != 0 {
		t.Errorf("a favoured Germany took a marginal fight with %d units; patience was free", n)
	}
}

// The favoured side fortifies its contact front past mere equality.
func TestPressure_FavouredSideHoldsFrontsAboveEquality(t *testing.T) {
	g, _ := pressureBoard(t)

	// USSR's Border faces German attack strength; USSR wins the race.
	g.PlacePieces("Reich", "armor", 4) // attack 12 next door

	favoured := findFronts(g, g.Players["USSR"], pressureFrontMargin(timePressure(g, g.Players["USSR"])))
	if len(favoured) == 0 {
		t.Fatal("no front found for the USSR")
	}
	if favoured[0].want <= favoured[0].enemy {
		t.Errorf("favoured front holds to %d against enemy %d; want a margin above equality",
			favoured[0].want, favoured[0].enemy)
	}

	// Germany, outproduced, holds its own front at equality and no higher --
	// the difference goes into the attack instead.
	g.PlacePieces("Border", "infantry", 2)
	pressed := findFronts(g, g.Players["Germany"], pressureFrontMargin(timePressure(g, g.Players["Germany"])))
	if len(pressed) == 0 {
		t.Fatal("no front found for Germany")
	}
	if pressed[0].want != pressed[0].enemy {
		t.Errorf("outproduced front holds to %d against enemy %d; want equality exactly",
			pressed[0].want, pressed[0].enemy)
	}
}

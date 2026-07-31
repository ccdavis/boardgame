package game

import (
	"sync"
	"testing"

	"boardgame/models"
)

// Two independent games resolving battles at the same time, as two web
// sessions do. Run with -race.
//
// Combat used to keep two package-level mutables: the unit registry (which
// every NewGameController overwrote, so concurrent games on different boards
// used each other's unit rules) and the artillery boost map (written during
// every land battle with no lock). The per-session locking never covered
// either. Capabilities are now derived statelessly from the piece and the
// boost record is local to the battle, so there is nothing left to race.
func TestConcurrentGamesDoNotShareCombatState(t *testing.T) {
	battleGame := func() (*GameController, string) {
		g := models.NewGame()
		g.PlayerOrder = []string{"A", "B"}
		attacker := g.GetOrCreatePlayer("A")
		defender := g.GetOrCreatePlayer("B")
		attacker.Side, defender.Side = "Axis", "Allies"
		attacker.TakesTurns, defender.TakesTurns = true, true

		g.AddTerritory("Front", models.Land, "B", 2)
		g.AddTerritory("Home", models.Land, "A", 2)
		g.ConnectTerritories("Home", "Front")

		g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
		g.AddPieceTemplate("artillery", models.Land, 1, 2, 2, 4)
		g.PlacePieces("Home", "infantry", 3)
		g.PlacePieces("Home", "artillery", 1)
		g.PlacePieces("Front", "infantry", 2)

		gc := NewGameController(g)
		gc.StartGame()
		g.CurrentPower = "A"
		g.CurrentPhase = models.CombatMovePhase
		for _, id := range append([]int{}, g.Board["Home"].Pieces...) {
			gc.PlanMove(id, "Home", "Front")
		}
		if err := gc.ExecuteCombatMoves(); err != nil {
			t.Fatalf("combat moves: %v", err)
		}
		g.CurrentPhase = models.ConductCombatPhase
		return gc, "Front"
	}

	const games = 8
	var wg sync.WaitGroup
	for i := 0; i < games; i++ {
		gc, where := battleGame()
		roller := NewSeededDiceRoller(int64(i + 1))
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := gc.ResolveBattleWithRetreat(where, roller, nil); err != nil {
				t.Errorf("resolving battle: %v", err)
			}
		}()
	}
	wg.Wait()
}

package game

import (
	"fmt"
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

// The second rung of the ladder: an outproduced power with no good direct
// attack takes production from a neutral instead -- priced honestly, with
// the toll, the garrison it will raise, and the neutrals the chain would
// hand to the enemy.
func TestPressure_OutproducedSideTurnsOnNeutrals(t *testing.T) {
	targets := func(t *testing.T, germanProduction int, chainCost int) map[string]bool {
		t.Helper()
		g, gc := pressureBoard(t)
		g.Board["Reich"].Production = germanProduction

		// The direct front is hopeless: a Soviet wall.
		g.PlacePieces("Border", "infantry", 30)

		// A strict neutral next door worth 4, and optionally another strict
		// neutral elsewhere whose defection the chain would gift the enemy.
		g.AddTerritory("Turkey", models.Land, "Neutral", 4)
		g.ConnectTerritories("Reich", "Turkey")
		if chainCost > 0 {
			g.AddTerritory("Helvetia", models.Land, "Neutral", chainCost)
		}
		g.Players["Germany"].IPCs = 20

		out := make(map[string]bool)
		for _, target := range NewSeededNPCAIPlayer("Germany", "normal", 1).
			findAttackTargets(g, g.Players["Germany"]) {
			out[target.Name] = true
		}
		_ = gc
		return out
	}

	// Outproduced, no other strict neutrals: Turkey is on the table.
	if got := targets(t, 6, 0); !got["Turkey"] {
		t.Error("an outproduced Germany with a hopeless front left Turkey alone")
	}
	// Out-producing the enemy: the neutrals are left in peace.
	if got := targets(t, 60, 0); got["Turkey"] {
		t.Error("a favoured Germany considered violating a neutral")
	}
	// Outproduced, but the chain would hand the enemy more than Turkey yields:
	// the diplomacy costs too much.
	if got := targets(t, 6, 10); got["Turkey"] {
		t.Error("violating Turkey gifts the enemy a 10-production neutral bloc; not worth it")
	}
}

// The estimator prices the garrison a neutral WILL raise, not the empty
// province on the board: a token force is refused the attack a mobilised
// Turkey would slaughter.
func TestPressure_NeutralAttackEstimatesTheLatentGarrison(t *testing.T) {
	g, _ := pressureBoard(t)
	g.AddTerritory("Turkey", models.Land, "Neutral", 4)
	g.ConnectTerritories("Reich", "Turkey")

	phantoms := latentDefenders(g, g.Board["Turkey"])
	if len(phantoms) != 4 {
		t.Fatalf("Turkey's latent garrison = %d, want 4 (one per production)", len(phantoms))
	}

	// One armor (attack 3) against the latent garrison (defence 8): the
	// estimator must see a bad fight, not a walkover.
	attacker := []*models.Piece{{Name: "armor", Attack: 3, Defend: 2}}
	if prob := EstimateAttackSuccess(attacker, expectedDefenders(g, g.Board["Turkey"])); prob > 0.45 {
		t.Errorf("attack on an 'empty' neutral estimated at %.2f; the garrison it raises makes it %d defence",
			prob, 8)
	}

	// Once mobilised, the real garrison is on the board and the phantoms go.
	g.PlacePieces("Turkey", "infantry", 4)
	if extra := latentDefenders(g, g.Board["Turkey"]); extra != nil {
		t.Errorf("a mobilised neutral still projects %d phantom defenders", len(extra))
	}
}

// The third rung: an outproduced power whose combat phase found nothing to
// hit digs in like the favoured side does.
func TestPressure_NothingToHitMeansDigIn(t *testing.T) {
	g, _ := pressureBoard(t)
	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	germany := g.Players["Germany"]

	npc.attacksThisTurn = 2 // striking: hold fronts at equality, spend on attack
	if m := npc.frontMargin(g, germany); m != 1.0 {
		t.Errorf("attacking under pressure: margin %.2f, want 1.0", m)
	}
	npc.attacksThisTurn = 0 // nothing worth hitting: fortify and outlast
	if m := npc.frontMargin(g, germany); m != 1.25 {
		t.Errorf("idle under pressure: margin %.2f, want 1.25", m)
	}
}

// A landing pays the neutral's price like an overland attack: the garrison
// rises and the toll is levied. The sea route used to dodge all of it.
func TestNeutral_AmphibiousViolationPaysTheSamePrice(t *testing.T) {
	g, gc := invasionBoard(t)
	player := g.Players["Germany"]
	player.IPCs = 20

	// The island is a strict neutral, production 3.
	models.ChangeOwnership(g.Board["Island"], g.GetOrCreatePlayer("Neutral"))
	g.Board["Island"].Production = 3
	g.Board["Island"].NeutralType = models.StrictNeutral

	// A second strict neutral to watch the chain.
	g.AddTerritory("Mongolia", models.Land, "Neutral", 1)
	g.Board["Mongolia"].NeutralType = models.StrictNeutral

	// A loaded transport in the drop zone, ready to land.
	if err := g.PlacePieces("Home", "infantry", 1); err != nil {
		t.Fatalf("troops: %v", err)
	}
	troopID := g.Board["Home"].Pieces[0]
	if err := g.PlacePieces("Island Sea", "transport", 1); err != nil {
		t.Fatalf("transport: %v", err)
	}
	transportID := g.Board["Island Sea"].Pieces[0]
	g.Pieces[transportID].Owner = player
	g.Board["Home"].Pieces = nil
	g.Pieces[transportID].Holding = []int{troopID}

	plan := gc.Plans.Add(&AmphibiousPlan{
		Power: "Germany", Target: "Island", Staging: "Home",
		Embark: "Home Sea", DropZone: "Island Sea", State: PlanReady,
	})
	plan.Ships = []int{transportID}
	plan.pendingLanding = []int{transportID}

	g.CurrentPhase = models.CombatMovePhase
	if landed := gc.LandAssaultTroops("Germany", NewGameTranscript("t")); landed != 1 {
		t.Fatalf("landed %d, want 1", landed)
	}

	if player.IPCs != 20-NeutralViolationCost {
		t.Errorf("treasury %d after the landing; the sea route dodged the toll", player.IPCs)
	}
	garrison := 0
	for _, id := range g.Board["Island"].Pieces {
		if piece := g.Pieces[id]; piece != nil && piece.Owner != nil && piece.Owner.Name == "Neutral" {
			garrison++
		}
	}
	if garrison != 3 {
		t.Errorf("the island raised %d defenders, want 3", garrison)
	}
	if owner := g.Board["Mongolia"].Owner.Name; owner == "Neutral" {
		t.Error("the chain did not fire for an amphibious violation")
	}
}

// The scoreboard clock: a side may be winning the economy and losing the
// game. Whichever race is going worse governs.
func TestVictoryRacePressure_TrailingSideFeelsTheClock(t *testing.T) {
	g, _ := pressureBoard(t)
	g.VictoryCitiesEnabled = true

	// Axis holds 2 victory cities (needs 9: 7 to go); Allies hold 7
	// (need 9: 2 to go). The Allies are five cities closer.
	mark := func(name, owner string, vc bool) {
		g.AddTerritory(name, models.Land, owner, 1)
		g.Board[name].IsVictoryCity = vc
	}
	for i, owner := range []string{"Germany", "Germany"} {
		mark(fmt.Sprintf("AxisCity%d", i), owner, true)
	}
	for i := 0; i < 7; i++ {
		mark(fmt.Sprintf("AlliedCity%d", i), "USSR", true)
	}

	germany, ussr := g.Players["Germany"], g.Players["USSR"]
	if p := victoryRacePressure(g, germany); p < 1.9 || p > 2.1 {
		t.Errorf("Germany trails by five cities: scoreboard pressure %.2f, want 2.0", p)
	}
	if p := victoryRacePressure(g, ussr); p != 1 {
		t.Errorf("the USSR leads the victory race: scoreboard pressure %.2f, want calm 1.0", p)
	}

	// With the switch off, the scoreboard says nothing.
	g.VictoryCitiesEnabled = false
	if p := victoryRacePressure(g, germany); p != 1 {
		t.Errorf("victory cities disabled but scoreboard pressure %.2f", p)
	}
}

// strategicPressure is governed by whichever clock is worse: a production
// lead does not excuse losing the victory race.
func TestStrategicPressure_WorseClockGoverns(t *testing.T) {
	g, _ := pressureBoard(t)
	g.VictoryCitiesEnabled = true

	// The USSR out-produces Germany fourfold (economic pressure 0.25 for it),
	// but Germany is two cities from winning while the USSR needs ten.
	for i := 0; i < 7; i++ {
		g.AddTerritory(fmt.Sprintf("AxisCity%d", i), models.Land, "Germany", 0)
		g.Board[fmt.Sprintf("AxisCity%d", i)].IsVictoryCity = true
	}

	ussr := g.Players["USSR"]
	economic := timePressure(g, ussr)
	if economic >= 1 {
		t.Fatalf("fixture: USSR should be winning the economy, pressure %.2f", economic)
	}
	combined := strategicPressure(g, ussr)
	if !outproduced(combined) {
		t.Errorf("USSR wins the economy but trails the victory race by eight cities; "+
			"strategic pressure %.2f should demand action", combined)
	}
}

// A power with the clock against it spends nearly everything; a comfortable
// one keeps its cushion.
func TestPressure_UrgentPowersSpendDown(t *testing.T) {
	spend := func(t *testing.T, germanProduction int) int {
		t.Helper()
		g, gc := pressureBoard(t)
		g.Board["Reich"].Production = germanProduction
		g.Players["Germany"].IPCs = 100

		g.CurrentPower = "Germany"
		g.CurrentPhase = models.PurchasePhase
		npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
		if err := npc.PurchasePhase(gc, NewGameTranscript("t")); err != nil {
			t.Fatalf("purchase: %v", err)
		}
		return 100 - g.Players["Germany"].IPCs
	}

	urgent := spend(t, 6)     // outproduced 4:1
	comfortable := spend(t, 60) // out-producing

	if urgent <= comfortable {
		t.Errorf("urgent power spent %d, comfortable spent %d; the clock should open the purse",
			urgent, comfortable)
	}
	if urgent < 90 {
		t.Errorf("urgent power spent only %d of 100; want nearly all of it", urgent)
	}
}

// The clock scales the expedition machinery: bigger landings, more of them,
// and no waiting on naval gunfire.
func TestPressure_ScalesInvasions(t *testing.T) {
	// Force caps grow with pressure and stop at the ceiling.
	if got := maxPlanTroopsFor(1.0); got != maxPlanTroopsCalm {
		t.Errorf("calm troop cap = %d, want %d", got, maxPlanTroopsCalm)
	}
	if got := maxPlanTroopsFor(1.35); got <= maxPlanTroopsCalm {
		t.Errorf("pressed troop cap = %d, want above the calm %d", got, maxPlanTroopsCalm)
	}
	if got := maxPlanTroopsFor(9.9); got != maxPlanTroopsUrgent {
		t.Errorf("desperate troop cap = %d, want ceiling %d", got, maxPlanTroopsUrgent)
	}

	// Concurrency: one front calm, two pressed, three desperate.
	if got := concurrentPlansFor(1.0); got != concurrentPlansCalm {
		t.Errorf("calm concurrency = %d, want %d", got, concurrentPlansCalm)
	}
	if got := concurrentPlansFor(1.2); got != concurrentPlansUrgent {
		t.Errorf("pressed concurrency = %d, want %d", got, concurrentPlansUrgent)
	}
	if got := concurrentPlansFor(2.0); got != concurrentPlansDesperate {
		t.Errorf("desperate concurrency = %d, want %d", got, concurrentPlansDesperate)
	}

	// A hopeless fortress for a calm power is a valid target for a desperate one.
	calm, desperate := hopelessDefenceFor(maxPlanTroopsFor(1.0)), hopelessDefenceFor(maxPlanTroopsFor(2.0))
	if desperate <= calm {
		t.Errorf("hopeless threshold calm %d, desperate %d; pressure should extend the reach", calm, desperate)
	}
}

// An outproduced power opens a second front when there is somewhere to open it.
func TestPressure_OpensASecondFrontUnderPressure(t *testing.T) {
	g, gc := pressureBoard(t)
	germany := g.Players["Germany"]

	addIsles(t, g)
	g.PlacePieces("Reich", "infantry", 6)

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(gc, germany, NewGameTranscript("t"))
	if got := len(gc.Plans.Active("Germany")); got < 2 {
		t.Errorf("outproduced Germany runs %d operation(s); the clock calls for a second front", got)
	}

	// The same board with the economy reversed: one operation at a time.
	g2, gc2 := pressureBoard(t)
	g2.Board["Reich"].Production = 60
	addIsles(t, g2)
	g2.PlacePieces("Reich", "infantry", 6)

	npc2 := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc2.ReviewPlans(gc2, g2.Players["Germany"], NewGameTranscript("t"))
	if got := len(gc2.Plans.Active("Germany")); got != 1 {
		t.Errorf("a favoured Germany runs %d operations; patience wants one at a time", got)
	}
}

// addIsles gives a pressure board a small archipelago to covet: two Soviet
// islands off the Reich coast, shipping templates, and a coastal factory so
// the plans may buy ships.
func addIsles(t *testing.T, g *models.Game) {
	t.Helper()
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	g.GlobalPieceTemplates["transport"].Capacity = 2
	g.GlobalPieceTemplates["transport"].CanCarry = []string{"infantry"}
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)

	g.AddTerritory("Reich Sea", models.Water, "Neutral", 0)
	g.ConnectTerritories("Reich", "Reich Sea")
	for _, name := range []string{"Isle One", "Isle Two"} {
		g.AddTerritory(name, models.Land, "USSR", 3)
		sea := name + " Sea"
		g.AddTerritory(sea, models.Water, "Neutral", 0)
		g.ConnectTerritories(name, sea)
		g.ConnectTerritories(sea, "Reich Sea")
	}
	if err := g.PlacePieces("Reich", "factory", 1); err != nil {
		t.Fatalf("factory: %v", err)
	}
}

// A power in a hurry does not wait out the price of a battleship.
func TestPressure_SkipsTheBombardierInAHurry(t *testing.T) {
	build := func(t *testing.T, production int) (*models.Game, *GameController) {
		t.Helper()
		g, gc := pressureBoard(t)
		g.Board["Reich"].Production = production
		addIsles(t, g)
		g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)
		g.PlacePieces("Reich", "infantry", 6)
		return g, gc
	}

	wantsGun := func(t *testing.T, production int) bool {
		g, gc := build(t, production)
		player := g.Players["Germany"]
		npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
		npc.ReviewPlans(gc, player, NewGameTranscript("t"))
		plans := gc.Plans.Active("Germany")
		if len(plans) == 0 {
			t.Fatal("no plan formed")
		}
		plan := plans[0]
		// Hand the plan its full lift, so only the gun question remains.
		for len(plan.Ships) < plan.WantTransports {
			if err := g.PlacePieces("Reich Sea", "transport", 1); err != nil {
				t.Fatalf("transport: %v", err)
			}
			id := g.Board["Reich Sea"].Pieces[len(g.Board["Reich Sea"].Pieces)-1]
			g.Pieces[id].Owner = player
			plan.Ships = append(plan.Ships, id)
		}
		return npc.PlanPurchases(gc, player)["battleship"] > 0
	}

	if wantsGun(t, 6) {
		t.Error("an outproduced power waits on a battleship; it should strike with what it has")
	}
	if !wantsGun(t, 60) {
		t.Error("a favoured power skips the bombardier; with time to spare it should want the gun")
	}
}

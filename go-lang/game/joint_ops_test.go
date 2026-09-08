package game

import (
	"testing"

	"boardgame/models"
)

// jointBoard: two Allied powers (UK, USA) each with a coastal factory and an
// army, one sea between them and a German-held island fortress too strong
// for either to lift alone.
func jointBoard(t *testing.T, garrison int) (*models.Game, *GameController) {
	t.Helper()
	g := models.NewGame()
	for _, name := range []string{"UK", "USA", "Germany"} {
		p := g.GetOrCreatePlayer(name)
		p.TakesTurns = true
		p.IPCs = 40
	}
	g.Players["UK"].Side, g.Players["USA"].Side, g.Players["Germany"].Side = "Allies", "Allies", "Axis"
	g.Players["UK"].Capital, g.Players["USA"].Capital, g.Players["Germany"].Capital = "Britain", "America", "Fortress"
	g.PlayerOrder = []string{"UK", "USA", "Germany"}

	g.AddTerritory("Britain", models.Land, "UK", 8)
	g.AddTerritory("Shire", models.Land, "UK", 2)
	g.AddTerritory("America", models.Land, "USA", 10)
	g.AddTerritory("Plains", models.Land, "USA", 2)
	g.AddTerritory("Fortress", models.Land, "Germany", 10)
	g.AddTerritory("Channel", models.Water, "Neutral", 0)
	g.AddTerritory("Ocean", models.Water, "Neutral", 0)
	g.ConnectTerritories("Britain", "Shire")
	g.ConnectTerritories("Britain", "Channel")
	g.ConnectTerritories("America", "Plains")
	g.ConnectTerritories("America", "Ocean")
	g.ConnectTerritories("Channel", "Ocean")
	g.ConnectTerritories("Channel", "Fortress")
	g.ConnectTerritories("Ocean", "Fortress")
	g.Board["Fortress"].IsVictoryCity = true

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	g.GlobalPieceTemplates["transport"].Capacity = 2
	g.GlobalPieceTemplates["transport"].CanCarry = []string{"infantry"}
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 32)

	g.PlacePieces("Britain", "factory", 1)
	g.PlacePieces("America", "factory", 1)
	g.PlacePieces("Britain", "infantry", 10)
	g.PlacePieces("America", "infantry", 10)
	g.PlacePieces("Fortress", "infantry", garrison)
	g.PlacePieces("Fortress", "factory", 1)

	c := NewGameController(g)
	c.StartGame()
	return g, c
}

// A fortress beyond one power's lift is proposed as a joint operation, and
// the ally's planner joins it rather than looking elsewhere.
func TestJoint_TwoPowersPlanTheSameFortress(t *testing.T) {
	g, c := jointBoard(t, 40)
	uk := g.Players["UK"]
	usa := g.Players["USA"]
	troopCap := maxPlanTroopsFor(strategicPressure(g, uk))
	if defenderStrength(g, "Fortress") <= hopelessDefenceFor(troopCap) {
		t.Skipf("fixture: fortress (defence %d) is not beyond one power's lift (%d)",
			defenderStrength(g, "Fortress"), hopelessDefenceFor(troopCap))
	}
	if defenderStrength(g, "Fortress") > hopelessDefenceFor(2*troopCap) {
		t.Skipf("fixture: fortress is beyond even two powers' lift")
	}

	ukNPC := NewSeededNPCAIPlayer("UK", "normal", 1)
	first := ukNPC.ProposePlan(c, uk)
	if first == nil || first.Target != "Fortress" {
		t.Fatalf("UK should open the joint operation against Fortress, got %+v", first)
	}
	if !first.Joint || first.Partner != "" {
		t.Errorf("the first half should be marked joint and partnerless, got joint=%v partner=%q", first.Joint, first.Partner)
	}
	c.Plans.Add(first)

	usaNPC := NewSeededNPCAIPlayer("USA", "normal", 2)
	second := usaNPC.ProposePlan(c, usa)
	if second == nil || second.Target != "Fortress" {
		t.Fatalf("USA should join the operation against Fortress, got %+v", second)
	}
	if !second.Joint || second.Partner != "UK" {
		t.Errorf("USA's plan should be joint with UK, got joint=%v partner=%q", second.Joint, second.Partner)
	}
	if first.Partner != "USA" {
		t.Errorf("UK's plan should now name USA as partner, got %q", first.Partner)
	}
	if second.WantTroops < 2 || second.WantTroops > troopCap {
		t.Errorf("USA's half wants %d troops; want between 2 and %d", second.WantTroops, troopCap)
	}
	c.Plans.Add(second)

	// A third proposal from the UK must not double up on the same target.
	if again := ukNPC.ProposePlan(c, uk); again != nil && again.Target == "Fortress" {
		t.Error("the UK proposed Fortress a second time while its plan stands")
	}
}

// One half of a joint operation waits at the drop zone until the other is
// beside the target too.
func TestJoint_HalfWaitsForItsPartner(t *testing.T) {
	g, c := jointBoard(t, 40)
	uk := g.Players["UK"]

	ukPlan := c.Plans.Add(&AmphibiousPlan{
		Power: "UK", Target: "Fortress", Staging: "Britain", Embark: "Channel", DropZone: "Channel",
		State: PlanReady, Joint: true, Partner: "USA", WantTroops: 4, CreatedTurn: 1, LastProgress: 1,
	})
	c.Plans.Add(&AmphibiousPlan{
		Power: "USA", Target: "Fortress", Staging: "America", Embark: "Ocean", DropZone: "Ocean",
		State: PlanForming, Joint: true, Partner: "UK", WantTroops: 4, CreatedTurn: 1, LastProgress: 1,
	})
	// A loaded UK transport in the Channel, beside the fortress.
	g.PlacePieces("Channel", "transport", 1)
	transport := g.NextPieceID - 1
	g.Pieces[transport].Owner = uk
	ukPlan.Ships = []int{transport}
	for i := 0; i < 2; i++ {
		id := g.Board["Britain"].Pieces[len(g.Board["Britain"].Pieces)-1]
		if err := g.LoadPiece(transport, id); err != nil {
			t.Fatal(err)
		}
		ukPlan.Troops = append(ukPlan.Troops, id)
	}

	g.CurrentPower = "UK"
	g.CurrentPhase = models.CombatMovePhase
	npc := NewSeededNPCAIPlayer("UK", "normal", 1)
	if launched := npc.ExecuteReadyPlans(c, uk, NewGameTranscript("t")); launched != 0 {
		t.Errorf("UK launched alone (%d) while the USA's half was still forming", launched)
	}
	// Once the partner is beside the target, the UK goes in.
	c.Plans.planOf("USA", "Fortress").State = PlanReady
	if launched := npc.ExecuteReadyPlans(c, uk, NewGameTranscript("t")); launched != 1 {
		t.Errorf("UK should launch with its partner ready, launched %d", launched)
	}
	if ukPlan.LaunchedTurn != g.Turn {
		t.Errorf("LaunchedTurn %d, want %d", ukPlan.LaunchedTurn, g.Turn)
	}
}

// Troops afloat beside a target an ally has just taken land there as
// reinforcements instead of sailing home.
func TestJoint_AfloatTroopsReinforceAnAllyCapture(t *testing.T) {
	g, c := jointBoard(t, 2)
	usa := g.Players["USA"]

	plan := c.Plans.Add(&AmphibiousPlan{
		Power: "USA", Target: "Fortress", Staging: "America", Embark: "Ocean", DropZone: "Ocean",
		State: PlanReady, WantTroops: 2, CreatedTurn: 1, LastProgress: 1,
	})
	g.PlacePieces("Ocean", "transport", 1)
	transport := g.NextPieceID - 1
	g.Pieces[transport].Owner = usa
	plan.Ships = []int{transport}
	for i := 0; i < 2; i++ {
		id := g.Board["America"].Pieces[len(g.Board["America"].Pieces)-1]
		if err := g.LoadPiece(transport, id); err != nil {
			t.Fatal(err)
		}
		plan.Troops = append(plan.Troops, id)
	}

	// The UK takes the fortress first.
	for _, id := range append([]int{}, g.Board["Fortress"].Pieces...) {
		if g.Pieces[id].Name == "infantry" {
			c.removePieceFromBoard(g.Pieces[id], "Fortress")
		}
	}
	if err := c.CaptureTerritory("Fortress", "UK"); err != nil {
		t.Fatal(err)
	}

	g.CurrentPower = "USA"
	g.CurrentPhase = models.NoncombatMovePhase
	npc := NewSeededNPCAIPlayer("USA", "normal", 3)
	npc.ReviewPlans(c, usa, NewGameTranscript("t"))
	if plan.State != PlanAbandoned {
		t.Fatalf("plan should end when an ally holds the target, state %v", plan.State)
	}
	if moved := npc.GatherForPlans(c, usa, NewGameTranscript("t")); moved != 2 {
		t.Errorf("expected 2 troops to reinforce the ally's beachhead, got %d", moved)
	}
	ashore := 0
	for _, id := range g.Board["Fortress"].Pieces {
		if g.Pieces[id].Owner == usa && g.Pieces[id].Name == "infantry" {
			ashore++
		}
	}
	if ashore != 2 {
		t.Errorf("%d American infantry ashore in Fortress, want 2", ashore)
	}
	if len(g.Pieces[transport].Holding) != 0 {
		t.Error("the transport still has troops aboard")
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("board invalid: %v", problems)
	}
}

// A human's booked landing is a claim the computer allies plan around.
func TestJoint_HumanLandingClaimsTheTarget(t *testing.T) {
	g, c := jointBoard(t, 2)
	uk := g.Players["UK"]
	g.PlacePieces("Channel", "transport", 1)
	transport := g.NextPieceID - 1
	g.Pieces[transport].Owner = uk
	troop := g.Board["Britain"].Pieces[len(g.Board["Britain"].Pieces)-1]
	if err := g.LoadPiece(transport, troop); err != nil {
		t.Fatal(err)
	}
	g.CurrentPower = "UK"
	g.CurrentPhase = models.CombatMovePhase
	if err := c.PlanLanding([]int{troop}, "Fortress"); err != nil {
		t.Fatal(err)
	}
	if !c.Plans.SideTargets("USA")["Fortress"] {
		t.Error("the USA's side targets do not include the human UK landing on Fortress")
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	if c.Plans.SideTargets("USA")["Fortress"] {
		t.Error("the claim outlived the landing that made it")
	}
}

// A leaked garrison plan is reported openly; an unleaked one is withheld.
func TestFog_GarrisonPlansLeakLikeOperations(t *testing.T) {
	g, c := jointBoard(t, 2)
	germany := g.Players["Germany"]
	npc := NewSeededNPCAIPlayer("Germany", "normal", 4)
	g.CurrentPower = "Germany"
	transcript := NewGameTranscript("t")
	npc.ReviewDefences(c, germany, transcript)
	plans := c.Plans.Defences("Germany")
	if len(plans) == 0 {
		t.Fatal("Germany opened no defence plan for its fortress")
	}
	secret := 0
	for _, entry := range transcript.Entries {
		if entry.Secret {
			secret++
		}
	}
	if secret == 0 {
		t.Error("a fresh garrison plan should be logged as secret")
	}

	// Force a leak and re-log: the line is public.
	plans[0].Revealed = true
	leaked := NewGameTranscript("t")
	logPlanLine(leaked, "Germany", plans[0].Revealed, plans[0].Describe(g))
	if len(leaked.Entries) != 1 || leaked.Entries[0].Secret {
		t.Error("a revealed garrison plan should be logged openly")
	}
	lines := leaked.LinesFor(false, "Germany")
	if len(lines) != 1 || lines[0] != plans[0].Describe(g) {
		t.Errorf("enemy viewer should read the leaked line, got %v", lines)
	}
}

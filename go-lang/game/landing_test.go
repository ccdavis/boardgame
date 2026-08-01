package game

import (
	"testing"

	"boardgame/models"
)

// setupLandingTest: Germany holds France with troops and a loaded transport
// plus a battleship in the North Sea; the UK holds its island with defenders.
func setupLandingTest(t *testing.T) (*GameController, []int, int) {
	t.Helper()
	g := models.NewGame()

	g.AddTerritory("France", models.Land, "Germany", 3)
	g.AddTerritory("North Sea", models.Water, "Germany", 0)
	g.AddTerritory("UK", models.Land, "UK", 8)
	g.ConnectTerritories("France", "North Sea")
	g.ConnectTerritories("North Sea", "France")
	g.ConnectTerritories("North Sea", "UK")
	g.ConnectTerritories("UK", "North Sea")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)
	g.SetContainerCapacity("transport", 2, []string{"infantry"})

	g.PlacePieces("France", "infantry", 2)
	g.PlacePieces("North Sea", "transport", 1)
	g.PlacePieces("North Sea", "battleship", 1)
	g.PlacePieces("UK", "infantry", 2)

	g.PlayerOrder = []string{"Germany", "UK"}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	if germany, ok := g.Players["Germany"]; ok {
		germany.Side = "Axis"
	}
	if uk, ok := g.Players["UK"]; ok {
		uk.Side = "Allies"
	}
	gc := NewGameController(g)

	// Identify the pieces: the two German infantry and the transport.
	france := g.Board["France"]
	cargo := append([]int{}, france.Pieces...)
	var transportID int
	for _, id := range g.Board["North Sea"].Pieces {
		if g.Pieces[id].Name == "transport" {
			transportID = id
		}
	}

	for _, id := range cargo {
		if err := gc.LoadUnit(transportID, id); err != nil {
			t.Fatalf("loading infantry %d: %v", id, err)
		}
	}
	return gc, cargo, transportID
}

func TestPlanLanding_CreatesAmphibiousBattle(t *testing.T) {
	gc, cargo, _ := setupLandingTest(t)

	if err := gc.PlanLanding(cargo, "UK"); err != nil {
		t.Fatalf("PlanLanding: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("ExecuteCombatMoves: %v", err)
	}

	battle, ok := gc.PendingBattles["UK"]
	if !ok {
		t.Fatal("no battle created in UK after the landing")
	}
	if battle.AttackerID != "Germany" || battle.DefenderID != "UK" {
		t.Errorf("battle sides = %s vs %s, want Germany vs UK", battle.AttackerID, battle.DefenderID)
	}
	if len(battle.AttackingPieceIDs) != 2 {
		t.Errorf("attacking pieces = %d, want 2", len(battle.AttackingPieceIDs))
	}
	if battle.AmphibiousUnits != 2 {
		t.Errorf("AmphibiousUnits = %d, want 2", battle.AmphibiousUnits)
	}
	// The battleship in the drop zone supports the landing, exactly once.
	if len(battle.Bombarding) != 1 {
		t.Errorf("bombarding ships = %d, want 1 (the battleship)", len(battle.Bombarding))
	}
	// Amphibious troops cannot retreat: no origin recorded means
	// withdrawAttackers leaves them ashore.
	for _, id := range cargo {
		if origin := battle.AttackerOrigins[id]; origin != "" {
			t.Errorf("cargo %d has retreat origin %q, want none", id, origin)
		}
	}
	// The troops are physically ashore, out of the hold.
	uk := gc.Game.Board["UK"]
	for _, id := range cargo {
		if !contains(uk.Pieces, id) {
			t.Errorf("cargo %d not in UK after landing", id)
		}
		if gc.Game.IsLoaded(id) {
			t.Errorf("cargo %d still aboard after landing", id)
		}
	}
	if len(gc.PlannedLandings) != 0 {
		t.Errorf("planned landings not cleared after execution")
	}
}

func TestPlanLanding_RefusesFriendlyShoreAndWrongPhase(t *testing.T) {
	gc, cargo, _ := setupLandingTest(t)

	if err := gc.PlanLanding(cargo, "France"); err == nil {
		t.Error("landing on one's own shore should be refused")
	}
	gc.Game.CurrentPhase = models.NoncombatMovePhase
	if err := gc.PlanLanding(cargo, "UK"); err == nil {
		t.Error("an assault landing outside the combat-move phase should be refused")
	}
}

func TestPlanLanding_TransportSailsThenLands(t *testing.T) {
	// The transport starts one zone away and is PLANNED to arrive: booking the
	// landing must accept the planned position, and the landing must execute
	// after the sail.
	gc, cargo, transportID := setupLandingTest(t)
	g := gc.Game

	// Add a second sea zone between: move transport there first.
	g.AddTerritory("Mid Sea", models.Water, "Germany", 0)
	g.ConnectTerritories("North Sea", "Mid Sea")
	g.ConnectTerritories("Mid Sea", "North Sea")
	// Physically relocate the transport (with cargo aboard) to Mid Sea, which
	// is NOT adjacent to UK.
	if err := g.MovePiece(transportID, "North Sea", "Mid Sea"); err != nil {
		t.Fatalf("staging the transport: %v", err)
	}

	// Booking without a planned move must fail -- the transport cannot reach.
	if err := gc.PlanLanding(cargo, "UK"); err == nil {
		t.Fatal("landing booked although the transport is out of range")
	}

	// Plan the sail to the drop zone, then book the landing.
	if err := gc.PlanMove(transportID, "Mid Sea", "North Sea"); err != nil {
		t.Fatalf("planning the sail: %v", err)
	}
	if err := gc.PlanLanding(cargo, "UK"); err != nil {
		t.Fatalf("PlanLanding with planned sail: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("ExecuteCombatMoves: %v", err)
	}

	if _, ok := gc.PendingBattles["UK"]; !ok {
		t.Fatal("no battle in UK: the landing did not follow the sail")
	}
}

func TestCancelLanding(t *testing.T) {
	gc, cargo, _ := setupLandingTest(t)

	if err := gc.PlanLanding(cargo, "UK"); err != nil {
		t.Fatalf("PlanLanding: %v", err)
	}
	if err := gc.CancelLanding(cargo[0]); err != nil {
		t.Fatalf("CancelLanding: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("ExecuteCombatMoves: %v", err)
	}

	battle, ok := gc.PendingBattles["UK"]
	if !ok {
		t.Fatal("the remaining booked unit should still have landed")
	}
	if len(battle.AttackingPieceIDs) != 1 {
		t.Errorf("attackers = %d, want 1 after one cancellation", len(battle.AttackingPieceIDs))
	}
	if !gc.Game.IsLoaded(cargo[0]) {
		t.Error("cancelled unit should still be aboard")
	}
}

func TestPlanLanding_StrictNeutralPaysTheToll(t *testing.T) {
	gc, cargo, _ := setupLandingTest(t)
	g := gc.Game

	// Make the shore a strict neutral with a small garrison, Turkey-style.
	g.AddTerritory("Turkey", models.Land, "Neutral", 4)
	g.ConnectTerritories("North Sea", "Turkey")
	g.ConnectTerritories("Turkey", "North Sea")
	turkey := g.Board["Turkey"]
	turkey.NeutralType = models.StrictNeutral
	g.PlayerOrder = append(g.PlayerOrder, "Neutral")

	germany := g.Players["Germany"]

	// Too poor to violate a strict neutral: refused, with the reason.
	germany.IPCs = 2
	if err := gc.PlanLanding(cargo, "Turkey"); err == nil {
		t.Fatal("landing on a strict neutral should be refused when the toll cannot be paid")
	}

	// Rich enough: the landing goes in, the toll is paid, the garrison rises.
	germany.IPCs = 10
	if err := gc.PlanLanding(cargo, "Turkey"); err != nil {
		t.Fatalf("PlanLanding on strict neutral with funds: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("ExecuteCombatMoves: %v", err)
	}

	if germany.IPCs != 10-NeutralViolationCost {
		t.Errorf("Germany has %d IPCs, want %d after paying the violation toll",
			germany.IPCs, 10-NeutralViolationCost)
	}
	battle, ok := gc.PendingBattles["Turkey"]
	if !ok {
		t.Fatal("no battle created in Turkey")
	}
	if battle.AmphibiousUnits != 2 {
		t.Errorf("AmphibiousUnits = %d, want 2", battle.AmphibiousUnits)
	}
	// The violated neutral mobilises one defender per point of production.
	defenders := 0
	attacking := make(map[int]bool)
	for _, id := range battle.AttackingPieceIDs {
		attacking[id] = true
	}
	for _, id := range turkey.Pieces {
		if !attacking[id] {
			defenders++
		}
	}
	if defenders != 4 {
		t.Errorf("garrison = %d defenders, want 4 (production value)", defenders)
	}
}

// setupContestedLanding: Germany's loaded transport must fight its way into
// the drop zone -- the North Sea is held by UK destroyers -- so executing the
// combat moves stages BOTH a sea battle there and the landing battle on UK
// soil, linked by AmphibiousFrom.
func setupContestedLanding(t *testing.T) (*GameController, []int) {
	t.Helper()
	g := models.NewGame()

	g.AddTerritory("France", models.Land, "Germany", 3)
	g.AddTerritory("Home Sea", models.Water, "Neutral", 0)
	g.AddTerritory("North Sea", models.Water, "Neutral", 0)
	g.AddTerritory("UK", models.Land, "UK", 8)
	g.ConnectTerritories("France", "Home Sea")
	g.ConnectTerritories("Home Sea", "France")
	g.ConnectTerritories("Home Sea", "North Sea")
	g.ConnectTerritories("North Sea", "Home Sea")
	g.ConnectTerritories("North Sea", "UK")
	g.ConnectTerritories("UK", "North Sea")

	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 8)
	g.AddPieceTemplate("destroyer", models.Water, 2, 2, 2, 8)
	g.SetContainerCapacity("transport", 2, []string{"infantry"})

	g.PlacePieces("France", "infantry", 2)
	g.PlacePieces("Home Sea", "transport", 1)
	g.PlacePieces("North Sea", "destroyer", 2)
	g.PlacePieces("UK", "infantry", 1)

	g.PlayerOrder = []string{"Germany", "UK"}
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	germany := g.Players["Germany"]
	uk := g.Players["UK"]
	germany.Side = "Axis"
	uk.Side = "Allies"

	// Pieces placed in Neutral water belong to Neutral; hand them to their
	// real owners. The destroyers guard the crossing for the UK, and the
	// transport is Germany's.
	for _, id := range g.Board["North Sea"].Pieces {
		g.Pieces[id].Owner = uk
	}
	for _, id := range g.Board["Home Sea"].Pieces {
		g.Pieces[id].Owner = germany
	}

	gc := NewGameController(g)

	cargo := append([]int{}, g.Board["France"].Pieces...)
	var transportID int
	for _, id := range g.Board["Home Sea"].Pieces {
		if g.Pieces[id].Name == "transport" {
			transportID = id
		}
	}
	for _, id := range cargo {
		if err := gc.LoadUnit(transportID, id); err != nil {
			t.Fatalf("loading infantry %d: %v", id, err)
		}
	}

	// The transport fights its way into the drop zone and the troops land.
	if err := gc.PlanMove(transportID, "Home Sea", "North Sea"); err != nil {
		t.Fatalf("planning transport into contested water: %v", err)
	}
	if err := gc.PlanLanding(cargo, "UK"); err != nil {
		t.Fatalf("planning landing: %v", err)
	}
	if err := gc.ExecuteCombatMoves(); err != nil {
		t.Fatalf("executing combat moves: %v", err)
	}
	gc.Game.CurrentPhase = models.ConductCombatPhase
	return gc, cargo
}

// The landing may not be fought while the sea battle covering it is pending.
func TestLanding_SeaBattleResolvesFirst(t *testing.T) {
	gc, _ := setupContestedLanding(t)

	if _, ok := gc.PendingBattles["North Sea"]; !ok {
		t.Fatal("no sea battle staged in the contested drop zone")
	}
	if _, ok := gc.PendingBattles["UK"]; !ok {
		t.Fatal("no landing battle staged on UK soil")
	}

	if _, err := gc.ResolveBattle("UK", NewSeededDiceRoller(1)); err == nil {
		t.Fatal("the landing resolved before the sea battle covering it")
	}

	// BattleOrder puts the sea fight first for the automatic resolvers.
	order := gc.BattleOrder()
	if len(order) != 2 || order[0] != "North Sea" {
		t.Errorf("battle order = %v, want the North Sea first", order)
	}
}

// Losing the fight off the beach drowns the landing force.
func TestLanding_LostDropZoneDrownsTheTroops(t *testing.T) {
	gc, cargo := setupContestedLanding(t)
	g := gc.Game

	// A lone transport (attack 0) against two destroyers: the covering
	// action can only be lost.
	seaResult, err := gc.ResolveBattle("North Sea", NewSeededDiceRoller(1))
	if err != nil {
		t.Fatalf("resolving sea battle: %v", err)
	}
	if seaResult.AttackerWins {
		t.Fatal("fixture broke: a transport with attack 0 won a sea battle")
	}

	landResult, err := gc.ResolveBattle("UK", NewSeededDiceRoller(1))
	if err != nil {
		t.Fatalf("resolving landing: %v", err)
	}
	if !landResult.DefenderWins {
		t.Error("the cut-off landing should be a defender victory")
	}
	if len(landResult.AttackerCasualties) != len(cargo) {
		t.Errorf("attacker casualties = %d, want the whole landing force (%d) drowned",
			len(landResult.AttackerCasualties), len(cargo))
	}
	for _, id := range cargo {
		if _, alive := g.Pieces[id]; alive {
			t.Errorf("landed infantry %d survived a drop zone in enemy hands", id)
		}
	}
	if owner := g.Board["UK"].Owner; owner == nil || owner.Name != "UK" {
		t.Error("UK changed hands despite the landing drowning")
	}
}

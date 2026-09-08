package game

import (
	"testing"

	"boardgame/models"
)

// raidGame: a German bomber in Germany, a Soviet factory in Moscow next door
// with an AA gun, and a Soviet territory beyond it for range checks.
func raidGame(t *testing.T) (*models.Game, *GameController) {
	t.Helper()
	g := createTestGame()
	g.AddPieceTemplate("bomber", models.Air, 6, 4, 2, 16)
	g.AddPieceTemplate("AAA", models.Land, 1, 0, 1, 5)
	g.ConnectTerritories("Germany", "Moscow")
	g.ConnectTerritories("Moscow", "Germany")
	for name, capital := range map[string]string{
		"USSR": "Moscow", "Germany": "Germany", "UK": "London", "Japan": "Tokyo", "USA": "Washington",
	} {
		g.Players[name].Capital = capital
	}
	c := NewGameController(g)
	c.StartGame()
	c.Dice = NewSeededDiceRoller(11)
	g.PlacePieces("Moscow", "factory", 1)
	g.PlacePieces("Moscow", "infantry", 4)
	g.PlacePieces("Moscow", "AAA", 1)
	g.PlacePieces("Germany", "bomber", 2)
	g.PlacePieces("Germany", "factory", 1)
	g.CurrentPower = "Germany"
	g.CurrentPhase = models.CombatMovePhase
	return g, c
}

func bombersIn(g *models.Game, territory string) []int {
	var ids []int
	for _, id := range g.Board[territory].Pieces {
		if g.Pieces[id].Name == "bomber" {
			ids = append(ids, id)
		}
	}
	return ids
}

// A raid is booked as a combat move, flies at execution without staging a
// battle against the garrison, and does its damage when resolved.
func TestRaid_BooksFliesAndDamages(t *testing.T) {
	g, c := raidGame(t)
	bombers := bombersIn(g, "Germany")
	for _, id := range bombers {
		if err := c.PlanBombingRaid(id, "Germany", "Moscow"); err != nil {
			t.Fatalf("planning raid: %v", err)
		}
	}
	if attacks := c.GetPlannedAttacks(); len(attacks) != 1 || attacks[0] != "Moscow" {
		t.Errorf("a raid should count as an attack on Moscow, got %v", attacks)
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	if _, fight := c.PendingBattles["Moscow"]; fight {
		t.Error("a raid must not stage a battle against the garrison")
	}
	raid, ok := c.PendingRaids["Moscow"]
	if !ok || len(raid.BomberIDs) != 2 {
		t.Fatalf("raid not booked with both bombers: %+v", raid)
	}
	for _, id := range bombers {
		if !contains(g.Board["Moscow"].Pieces, id) {
			t.Errorf("bomber %d is not over Moscow", id)
		}
	}

	g.CurrentPhase = models.ConductCombatPhase
	result, err := c.ResolveRaid("Moscow", c.Dice)
	if err != nil {
		t.Fatal(err)
	}
	if _, still := c.PendingRaids["Moscow"]; still {
		t.Error("raid still pending after resolution")
	}
	survivors := result.Bombers - result.BombersLost
	if got := len(bombersIn(g, "Moscow")); got != survivors {
		t.Errorf("%d bombers over Moscow after the raid, want %d", got, survivors)
	}
	if survivors > 0 && result.Damage == 0 {
		t.Error("surviving bombers rolled no damage at all")
	}
	if g.Board["Moscow"].ICDamage != result.Damage {
		t.Errorf("Moscow damage %d, raid reported %d", g.Board["Moscow"].ICDamage, result.Damage)
	}
	if g.Board["Moscow"].ICDamage > 2*g.Board["Moscow"].Production {
		t.Error("damage exceeds twice production")
	}
	if g.Board["Moscow"].Owner.Name != "USSR" {
		t.Error("a raid must not change ownership")
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("board invalid: %v", problems)
	}
	// The complex builds less until repaired.
	if c.FactoryCapacity(g.Board["Moscow"]) != GetEffectiveProduction(g.Board["Moscow"]) {
		t.Error("factory capacity ignores bomb damage")
	}
}

// Only bombers raid, only enemy complexes are targets, and a raid is a
// combat-phase move.
func TestRaid_Rules(t *testing.T) {
	g, c := raidGame(t)
	g.PlacePieces("Germany", "fighter", 1)
	var fighter int
	for _, id := range g.Board["Germany"].Pieces {
		if g.Pieces[id].Name == "fighter" {
			fighter = id
		}
	}
	bomber := bombersIn(g, "Germany")[0]

	if err := c.PlanBombingRaid(fighter, "Germany", "Moscow"); err == nil {
		t.Error("a fighter was allowed to fly a bombing raid")
	}
	if err := c.PlanBombingRaid(bomber, "Germany", "Germany"); err == nil {
		t.Error("a raid on our own complex was allowed")
	}
	g.AddTerritory("Steppe", models.Land, "USSR", 2)
	g.ConnectTerritories("Moscow", "Steppe")
	g.ConnectTerritories("Steppe", "Moscow")
	if err := c.PlanBombingRaid(bomber, "Germany", "Steppe"); err == nil {
		t.Error("a raid on a territory with no complex was allowed")
	}
	g.CurrentPhase = models.NoncombatMovePhase
	if err := c.PlanBombingRaid(bomber, "Germany", "Moscow"); err == nil {
		t.Error("a raid was booked outside the combat phase")
	}
}

// The computer sends an idle bomber against a worthwhile complex it can
// reach and return from, and sends a fighter two zones away to a fight.
func TestNPC_AirReachesBeyondNextDoor(t *testing.T) {
	g, c := raidGame(t)
	// A Soviet outpost two zones from Germany, lightly held, with a friendly
	// landing zone on the way.
	g.AddTerritory("Marsh", models.Land, "Germany", 1)
	g.AddTerritory("Outpost", models.Land, "USSR", 3)
	g.ConnectTerritories("Germany", "Marsh")
	g.ConnectTerritories("Marsh", "Germany")
	g.ConnectTerritories("Marsh", "Outpost")
	g.ConnectTerritories("Outpost", "Marsh")
	g.PlacePieces("Outpost", "infantry", 1)
	g.PlacePieces("Marsh", "infantry", 1)
	g.PlacePieces("Germany", "fighter", 3)
	g.PlacePieces("Germany", "armor", 2)

	npc := NewSeededNPCAIPlayer("Germany", "normal", 5)
	player := g.Players["Germany"]
	target := g.Board["Outpost"]
	attackers := npc.findAttackersFor(c, player, target)
	planes := 0
	for _, piece := range attackers["Germany"] {
		if piece.Terrain == models.Air {
			planes++
		}
	}
	if planes == 0 {
		t.Error("no aircraft from Germany, two zones off, were offered for the attack on Outpost")
	}

	// Idle bombers raid Moscow's factory (production 8, in range, home next door).
	transcript := NewGameTranscript("t")
	raids := npc.PlanBombingRaids(c, player, transcript)
	if raids == 0 {
		t.Fatal("the NPC booked no bombing raid against Moscow's factory")
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.PendingRaids["Moscow"]; !ok {
		t.Error("the booked raid did not arrive over Moscow")
	}
}

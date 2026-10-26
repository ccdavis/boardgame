package game_test

import (
	"testing"

	"boardgame/engine"
	"boardgame/game"
	"boardgame/models"
)

func gameplayRegressionGame() (*models.Game, *game.GameController) {
	g := models.NewGame()
	for _, p := range []struct{ name, side string }{{"Germany", "Axis"}, {"UK", "Allies"}} {
		player := g.GetOrCreatePlayer(p.name)
		player.Side, player.TakesTurns, player.IPCs = p.side, true, 50
	}
	g.PlayerOrder = []string{"Germany", "UK"}
	g.AddTerritory("Home", models.Land, "Germany", 1)
	g.AddTerritory("Enemy", models.Land, "UK", 3)
	g.Players["Germany"].Capital = "Home"
	g.Players["UK"].Capital = "Enemy"
	g.ConnectTerritories("Home", "Enemy")
	g.AddPieceTemplate("infantry", models.Land, 1, 1, 2, 3)
	g.AddPieceTemplate("factory", models.Land, 0, 0, 0, 15)
	g.AddPieceTemplate("fighter", models.Air, 4, 3, 4, 12)
	g.AddPieceTemplate("bomber", models.Air, 6, 4, 1, 15)
	g.AddPieceTemplate("sub", models.Water, 2, 2, 2, 8)
	g.AddPieceTemplate("battleship", models.Water, 2, 4, 4, 24)
	g.AddPieceTemplate("carrier", models.Water, 2, 0, 1, 24)
	g.SetContainerCapacity("carrier", 2, []string{"fighter"})
	c := game.NewGameController(g)
	c.StartGame()
	return g, c
}

func placeRegressionUnit(t *testing.T, g *models.Game, where, kind, owner string) int {
	t.Helper()
	if err := g.PlacePieces(where, kind, 1); err != nil {
		t.Fatal(err)
	}
	id := g.NextPieceID - 1
	g.Pieces[id].Owner = g.Players[owner]
	return id
}

func TestGameplay_SeaCombatInNominallyOwnZone(t *testing.T) {
	g, c := gameplayRegressionGame()
	g.AddTerritory("Sea A", models.Water, "UK", 0)
	g.AddTerritory("Sea B", models.Water, "Germany", 0)
	g.ConnectTerritories("Sea A", "Sea B")
	id := placeRegressionUnit(t, g, "Sea A", "battleship", "Germany")
	placeRegressionUnit(t, g, "Sea B", "battleship", "UK")
	g.CurrentPhase = models.CombatMovePhase
	if err := c.PlanMove(id, "Sea A", "Sea B"); err != nil {
		t.Fatal(err)
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	if c.PendingBattles["Sea B"] == nil {
		t.Fatal("opposing battleships now share Sea B, but no battle was created")
	}
	if c.PendingBattles["Sea B"].DefenderID != "UK" {
		t.Fatal("sea battle names the zone's nominal owner instead of the enemy fleet")
	}
}

func TestGameplay_BombingAircraftMustNotDefendAgainstOwnArmy(t *testing.T) {
	g, c := gameplayRegressionGame()
	bomber := placeRegressionUnit(t, g, "Home", "bomber", "Germany")
	infantry := placeRegressionUnit(t, g, "Home", "infantry", "Germany")
	placeRegressionUnit(t, g, "Enemy", "infantry", "UK")
	placeRegressionUnit(t, g, "Enemy", "factory", "UK")
	g.CurrentPhase = models.CombatMovePhase
	if err := c.PlanBombingRaid(bomber, "Home", "Enemy"); err != nil {
		t.Fatal(err)
	}
	if err := c.PlanMove(infantry, "Home", "Enemy"); err != nil {
		t.Fatal(err)
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	g.CurrentPhase = models.ConductCombatPhase
	live, err := c.BeginBattle("Enemy")
	if err != nil {
		t.Fatal(err)
	}
	for _, defender := range live.Defenders {
		if defender.ID == bomber {
			t.Fatal("German bombing aircraft is a defender against German infantry")
		}
	}
}

func TestGameplay_PurchasesMustNotPermanentlyBlockTurn(t *testing.T) {
	g, c := gameplayRegressionGame()
	placeRegressionUnit(t, g, "Home", "factory", "Germany")
	if err := c.PurchaseUnit("infantry", 2); err != nil {
		t.Fatal(err)
	}
	d := engine.New(c)
	for g.CurrentPhase != models.MobilizePhase {
		if _, err := d.AdvancePhase(); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.MobilizeUnit("Home", "infantry"); err != nil {
		t.Fatal(err)
	}
	if len(c.PlacementTargets(g.Players["Germany"], "infantry")) != 0 {
		t.Fatal("setup still has placement capacity")
	}
	if _, err := d.AdvancePhase(); err != nil {
		t.Fatalf("turn is permanently blocked after legal purchases: %v", err)
	}
	if got := g.Players["Germany"].IPCs; got != 47 {
		t.Fatalf("IPCs = %d, want 47 after refunding unplaceable infantry", got)
	}
	if len(g.PurchasedUnits["Germany"]) != 0 {
		t.Fatal("refunded units remain pending")
	}
}

func TestGameplay_NegativePurchasesMustNotMintMoney(t *testing.T) {
	g, c := gameplayRegressionGame()
	before := g.Players["Germany"].IPCs
	if err := c.PurchaseUnit("infantry", -10); err == nil {
		t.Fatalf("negative purchase accepted: IPCs %d -> %d, pending units %d", before, g.Players["Germany"].IPCs, len(g.PurchasedUnits["Germany"]))
	}
}

func TestGameplay_AirAndSubmarineCannotHitEachOther(t *testing.T) {
	for seed := int64(1); seed < 100; seed++ {
		b := game.NewBattle("Sea", game.SeaBattle, "Germany", "UK")
		b.AddAttacker(&models.Piece{Name: "fighter", Terrain: models.Air, Movement: 4, Attack: 3, Defend: 4, Cost: 12})
		b.AddDefender(&models.Piece{Name: "sub", Terrain: models.Water, Movement: 2, Attack: 2, Defend: 2, Cost: 8})
		result, err := game.ResolveCombat(b, game.NewSeededDiceRoller(seed), 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.AttackerCasualties)+len(result.DefenderCasualties) > 0 {
			t.Fatalf("seed %d: unescorted fighter vs sub caused %d fighter and %d submarine losses", seed, len(result.AttackerCasualties), len(result.DefenderCasualties))
		}
	}
}

func TestGameplay_SubmarineSurpriseStrikesAreSimultaneous(t *testing.T) {
	for seed := int64(1); seed < 1000; seed++ {
		probe := game.NewSeededDiceRoller(seed)
		if probe.Roll() > 2 || probe.Roll() > 2 {
			continue
		}
		b := game.NewBattle("Sea", game.SeaBattle, "Germany", "UK")
		b.AddAttacker(&models.Piece{Name: "sub", Terrain: models.Water, Movement: 2, Attack: 2, Defend: 2, Cost: 8})
		b.AddDefender(&models.Piece{Name: "sub", Terrain: models.Water, Movement: 2, Attack: 2, Defend: 2, Cost: 8})
		_, _, surprise := game.NewSeededDiceRoller(seed).CombatRound(b)
		if len(surprise.AttackerHits) != 1 || len(surprise.DefenderHits) != 1 {
			t.Fatalf("seed %d has two hit rolls, but attacker got %d hits and defender got %d", seed, len(surprise.AttackerHits), len(surprise.DefenderHits))
		}
		return
	}
	t.Fatal("could not find seed")
}

func TestGameplay_OutgoingAircraftFreesCarrierSeat(t *testing.T) {
	g, c := gameplayRegressionGame()
	g.AddTerritory("Sea", models.Water, "Germany", 0)
	g.ConnectTerritories("Sea", "Home")
	placeRegressionUnit(t, g, "Sea", "carrier", "Germany")
	outgoing := placeRegressionUnit(t, g, "Sea", "fighter", "Germany")
	placeRegressionUnit(t, g, "Sea", "fighter", "Germany")
	incoming := placeRegressionUnit(t, g, "Home", "fighter", "Germany")
	g.CurrentPhase = models.NoncombatMovePhase
	if err := c.PlanMove(outgoing, "Sea", "Home"); err != nil {
		t.Fatal(err)
	}
	if err := c.PlanMove(incoming, "Home", "Sea"); err != nil {
		t.Fatalf("fighter cannot use seat vacated by planned outgoing fighter: %v", err)
	}
}

func TestGameplay_LostSeaBattleMustPreventCaptureOfEmptyBeach(t *testing.T) {
	g, c := gameplayRegressionGame()
	g.AddTerritory("Sea A", models.Water, "Germany", 0)
	g.AddTerritory("Sea B", models.Water, "UK", 0)
	g.ConnectTerritories("Home", "Sea A")
	g.ConnectTerritories("Sea A", "Sea B")
	g.ConnectTerritories("Sea B", "Enemy")
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 10)
	g.SetContainerCapacity("transport", 2, []string{"infantry"})
	transport := placeRegressionUnit(t, g, "Sea A", "transport", "Germany")
	infantry := placeRegressionUnit(t, g, "Home", "infantry", "Germany")
	placeRegressionUnit(t, g, "Sea B", "battleship", "UK")
	g.CurrentPhase = models.CombatMovePhase
	if err := c.LoadUnit(transport, infantry); err != nil {
		t.Fatal(err)
	}
	if err := c.PlanMove(transport, "Sea A", "Sea B"); err != nil {
		t.Fatal(err)
	}
	if err := c.PlanLanding([]int{infantry}, "Enemy"); err != nil {
		t.Fatal(err)
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	g.CurrentPhase = models.ConductCombatPhase
	result, err := c.ResolveBattle("Sea B", game.NewSeededDiceRoller(1))
	if err != nil {
		t.Fatal(err)
	}
	if !result.DefenderWins {
		t.Fatal("setup did not lose sea battle")
	}
	if g.Board["Enemy"].Owner.Name != "UK" {
		t.Fatal("beach captured before sea battle was won")
	}
	if _, err := c.ResolveBattle("Enemy", game.NewSeededDiceRoller(1)); err != nil {
		t.Fatal(err)
	}
	if g.Board["Enemy"].Owner.Name != "UK" || g.Pieces[infantry] != nil {
		t.Fatalf("transport sunk, but empty beach is owned by %s and landed infantry survives=%v", g.Board["Enemy"].Owner.Name, g.Pieces[infantry] != nil)
	}
}

func TestGameplay_TransportCannotFerryUnlimitedTroopsInOnePhase(t *testing.T) {
	g, c := gameplayRegressionGame()
	g.AddTerritory("Sea", models.Water, "Germany", 0)
	g.AddTerritory("Friendly shore", models.Land, "Germany", 1)
	g.ConnectTerritories("Home", "Sea")
	g.ConnectTerritories("Sea", "Friendly shore")
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 10)
	g.SetContainerCapacity("transport", 2, []string{"infantry"})
	transport := placeRegressionUnit(t, g, "Sea", "transport", "Germany")
	var infantry []int
	for i := 0; i < 6; i++ {
		infantry = append(infantry, placeRegressionUnit(t, g, "Home", "infantry", "Germany"))
	}
	g.CurrentPhase = models.NoncombatMovePhase
	moved := 0
	for _, id := range infantry {
		if err := c.LoadUnit(transport, id); err != nil {
			break
		}
		if err := c.UnloadUnit(transport, id, "Friendly shore"); err != nil {
			break
		}
		moved++
	}
	if moved > 2 {
		t.Fatalf("capacity-2 transport ferried %d infantry in a single phase without moving", moved)
	}
}

func TestGameplay_InvalidPurchaseDoesNotChangeTreasury(t *testing.T) {
	for _, quantity := range []int{0, -1, int(^uint(0) >> 1)} {
		g, c := gameplayRegressionGame()
		if err := c.PurchaseUnit("infantry", quantity); err == nil {
			t.Fatalf("accepted invalid/unaffordable quantity %d", quantity)
		}
		if g.Players["Germany"].IPCs != 50 || len(g.PurchasedUnits["Germany"]) != 0 {
			t.Fatal("refused purchase changed money or pending units")
		}
	}
}

func TestGameplay_BombingAircraftSitsOutAutomaticLandCombat(t *testing.T) {
	g, c := gameplayRegressionGame()
	bomber := placeRegressionUnit(t, g, "Home", "bomber", "Germany")
	infantry := placeRegressionUnit(t, g, "Home", "infantry", "Germany")
	g.Pieces[infantry].Attack = 6
	defender := placeRegressionUnit(t, g, "Enemy", "infantry", "UK")
	g.Pieces[defender].Defend = 0
	placeRegressionUnit(t, g, "Enemy", "factory", "UK")
	g.CurrentPhase = models.CombatMovePhase
	if err := c.PlanBombingRaid(bomber, "Home", "Enemy"); err != nil {
		t.Fatal(err)
	}
	if err := c.PlanMove(infantry, "Home", "Enemy"); err != nil {
		t.Fatal(err)
	}
	if err := c.ExecuteCombatMoves(); err != nil {
		t.Fatal(err)
	}
	g.CurrentPhase = models.ConductCombatPhase
	result, err := c.ResolveBattle("Enemy", game.NewSeededDiceRoller(1))
	if err != nil {
		t.Fatal(err)
	}
	if !result.AttackerWins || len(result.DefenderCasualties) != 1 || result.DefenderCasualties[0].ID != defender {
		t.Fatalf("land battle fought the wrong units: %+v", result)
	}
	if g.Pieces[bomber] == nil || g.Pieces[bomber].Owner.Name != "Germany" {
		t.Fatal("raiding bomber was lost or changed sides during land combat")
	}
	if len(c.PendingRaids["Enemy"].BomberIDs) != 1 {
		t.Fatal("ground combat consumed the raid")
	}
}

func TestGameplay_TransportFinishesCargoButCannotReloadOrSail(t *testing.T) {
	g, c := gameplayRegressionGame()
	g.AddTerritory("Sea", models.Water, "Germany", 0)
	g.AddTerritory("Other sea", models.Water, "Germany", 0)
	g.AddTerritory("Shore", models.Land, "Germany", 1)
	g.ConnectTerritories("Home", "Sea")
	g.ConnectTerritories("Sea", "Shore")
	g.ConnectTerritories("Sea", "Other sea")
	g.ConnectTerritories("Shore", "Home")
	g.AddPieceTemplate("transport", models.Water, 2, 0, 1, 10)
	g.SetContainerCapacity("transport", 2, []string{"infantry"})
	ship := placeRegressionUnit(t, g, "Sea", "transport", "Germany")
	first := placeRegressionUnit(t, g, "Home", "infantry", "Germany")
	second := placeRegressionUnit(t, g, "Home", "infantry", "Germany")
	third := placeRegressionUnit(t, g, "Home", "infantry", "Germany")
	g.CurrentPhase = models.NoncombatMovePhase
	for _, id := range []int{first, second} {
		if err := c.LoadUnit(ship, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.UnloadUnit(ship, first, "Shore"); err != nil {
		t.Fatal(err)
	}
	if err := c.UnloadUnit(ship, second, "Home"); err == nil {
		t.Fatal("unloaded at two shores")
	}
	if err := c.UnloadUnit(ship, second, "Shore"); err != nil {
		t.Fatalf("could not finish unloading original cargo: %v", err)
	}
	if err := c.LoadUnit(ship, third); err == nil {
		t.Fatal("reloaded after unloading")
	}
	if err := c.PlanMove(ship, "Sea", "Other sea"); err == nil {
		t.Fatal("sailed after unloading")
	}
	if err := c.PlanMove(first, "Shore", "Home"); err == nil {
		t.Fatal("land unit moved after unloading")
	}
	if err := c.AdvanceTurn(); err != nil {
		t.Fatal(err)
	}
	if err := c.AdvanceTurn(); err != nil {
		t.Fatal(err)
	}
	g.CurrentPhase = models.NoncombatMovePhase
	if err := c.LoadUnit(ship, third); err != nil {
		t.Fatalf("transport restriction leaked into next turn: %v", err)
	}
}

func TestGameplay_EmptyBeachWaitsForSeaBattle(t *testing.T) {
	for _, mode := range []string{"automatic", "live", "transport lost"} {
		t.Run(mode, func(t *testing.T) {
			g, c := gameplayRegressionGame()
			g.AddTerritory("Sea A", models.Water, "Germany", 0)
			g.AddTerritory("Sea B", models.Water, "UK", 0)
			g.ConnectTerritories("Home", "Sea A")
			g.ConnectTerritories("Sea A", "Sea B")
			g.ConnectTerritories("Sea B", "Enemy")
			// Guaranteed hits make the sea outcome deterministic. In the last
			// case both ships sink, so an empty drop zone still loses its cargo.
			defence := 0
			if mode == "transport lost" {
				defence = 6
			}
			g.AddPieceTemplate("transport", models.Water, 2, 6, 1, 10)
			g.SetContainerCapacity("transport", 2, []string{"infantry"})
			g.AddPieceTemplate("destroyer", models.Water, 2, 0, defence, 8)
			ship := placeRegressionUnit(t, g, "Sea A", "transport", "Germany")
			cargo := placeRegressionUnit(t, g, "Home", "infantry", "Germany")
			placeRegressionUnit(t, g, "Sea B", "destroyer", "UK")
			g.CurrentPhase = models.CombatMovePhase
			if err := c.LoadUnit(ship, cargo); err != nil {
				t.Fatal(err)
			}
			if err := c.PlanMove(ship, "Sea A", "Sea B"); err != nil {
				t.Fatal(err)
			}
			if err := c.PlanLanding([]int{cargo}, "Enemy"); err != nil {
				t.Fatal(err)
			}
			if err := c.ExecuteCombatMoves(); err != nil {
				t.Fatal(err)
			}
			if g.Board["Enemy"].Owner.Name != "UK" {
				t.Fatal("captured beach before the sea battle")
			}
			g.CurrentPhase = models.ConductCombatPhase
			if _, err := c.BeginBattle("Enemy"); err == nil {
				t.Fatal("began landing before sea battle was resolved")
			}
			if _, err := c.ResolveBattle("Sea B", game.NewSeededDiceRoller(1)); err != nil {
				t.Fatal(err)
			}
			if mode == "live" {
				if _, err := c.BeginBattle("Enemy"); err != nil {
					t.Fatal(err)
				}
			} else if _, err := c.ResolveBattle("Enemy", game.NewSeededDiceRoller(1)); err != nil {
				t.Fatal(err)
			}
			want := "Germany"
			if mode == "transport lost" {
				want = "UK"
			}
			if g.Board["Enemy"].Owner.Name != want {
				t.Fatalf("beach owner = %s, want %s", g.Board["Enemy"].Owner.Name, want)
			}
			if mode == "transport lost" && g.Pieces[cargo] != nil {
				t.Fatal("cargo survived its sunken transport")
			}
			if problems := g.Validate(); len(problems) != 0 {
				t.Fatal(problems)
			}
		})
	}
}

func TestGameplay_AirTargetsNeedTheirOwnDestroyer(t *testing.T) {
	for _, friendlyDestroyer := range []bool{false, true} {
		b := game.NewBattle("Sea", game.SeaBattle, "Germany", "UK")
		b.AddAttacker(&models.Piece{Name: "fighter", Terrain: models.Air, Attack: 6, Cost: 12})
		b.AddDefender(&models.Piece{Name: "sub", Terrain: models.Water, Movement: 2, Defend: 0, Cost: 8})
		destroyer := &models.Piece{Name: "destroyer", Terrain: models.Water, Attack: 0, Defend: 0, Movement: 2, Cost: 10}
		if friendlyDestroyer {
			b.AddAttacker(destroyer)
		} else {
			b.AddDefender(destroyer)
		}
		result, err := game.ResolveCombat(b, game.NewSeededDiceRoller(1), 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.DefenderCasualties) != 1 {
			t.Fatal("guaranteed air hit was not applied")
		}
		want := "destroyer"
		if friendlyDestroyer {
			want = "sub"
		}
		if result.DefenderCasualties[0].Name != want {
			t.Fatalf("friendly destroyer=%v: killed %s, want %s", friendlyDestroyer, result.DefenderCasualties[0].Name, want)
		}
	}
}

func TestGameplay_MixedHitsDoNotWasteLegalDamage(t *testing.T) {
	surface := &models.Piece{Name: "transport", Terrain: models.Water, Cost: 1, Movement: 2}
	sub := &models.Piece{Name: "sub", Terrain: models.Water, Cost: 8, Movement: 2}
	lost := game.SelectCombatCasualties([]*models.Piece{surface, sub}, []game.Hit{{}, {CannotHitSubmarines: true}})
	if len(lost) != 2 {
		t.Fatal("unrestricted hit consumed the only target the aircraft could hit")
	}
	ship := &models.Piece{Name: "battleship", Terrain: models.Water, Cost: 24, Movement: 2}
	lost = game.SelectCombatCasualties([]*models.Piece{ship}, []game.Hit{{CannotHitAir: true}})
	if len(lost) != 0 || ship.Hits != 1 {
		t.Fatal("restricted hit did not damage battleship correctly")
	}
}

func TestGameplay_ManualCasualtiesRespectAircraftTargetRestrictions(t *testing.T) {
	g, c := gameplayRegressionGame()
	g.AddTerritory("Sea", models.Water, "UK", 0)
	sub := placeRegressionUnit(t, g, "Sea", "sub", "Germany")
	fighter := placeRegressionUnit(t, g, "Sea", "fighter", "Germany")
	carrier := placeRegressionUnit(t, g, "Sea", "carrier", "Germany")
	defender := placeRegressionUnit(t, g, "Sea", "fighter", "UK")
	deck := placeRegressionUnit(t, g, "Sea", "carrier", "UK")
	for _, id := range []int{sub, fighter, carrier} {
		g.Pieces[id].Attack = 0
	}
	g.Pieces[defender].Defend = 6
	g.Pieces[deck].Defend = 0
	b := game.NewBattle("Sea", game.SeaBattle, "Germany", "UK")
	b.AttackingPieceIDs = []int{sub, fighter, carrier}
	c.PendingBattles["Sea"] = b
	c.Dice = game.NewSeededDiceRoller(1)
	if _, err := c.BeginBattle("Sea"); err != nil {
		t.Fatal(err)
	}
	live, err := c.FightRound("Sea")
	if err != nil {
		t.Fatal(err)
	}
	if live.PendingAttackerHits != 1 {
		t.Fatalf("owed %d hits, want 1", live.PendingAttackerHits)
	}
	if _, err := c.AssignCasualties("Sea", map[int]int{sub: 1}); err == nil {
		t.Fatal("assigned an aircraft hit to a submarine without a destroyer")
	}
	if live.PendingAttackerHits != 1 || len(live.Attackers) != 3 {
		t.Fatal("invalid choice mutated the battle")
	}
	if _, err := c.AssignCasualties("Sea", map[int]int{carrier: 1}); err != nil {
		t.Fatal(err)
	}
	if live.PendingAttackerHits != 0 || len(live.Attackers) != 2 {
		t.Fatal("valid choice was not applied")
	}
}

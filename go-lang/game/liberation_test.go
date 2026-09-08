package game

import (
	"testing"

	"boardgame/models"
)

// liberationGame: three Allied powers and Germany, with capitals, so the
// liberation rule has an original owner and a capital to consult.
func liberationGame(t *testing.T) (*models.Game, *GameController) {
	t.Helper()
	g := createTestGame()
	for name, capital := range map[string]string{
		"USSR": "Moscow", "Germany": "Germany", "UK": "London", "Japan": "Tokyo", "USA": "Washington",
	} {
		g.Players[name].Capital = capital
	}
	g.AddTerritory("Egypt", models.Land, "UK", 2)
	g.AddTerritory("Ukraine", models.Land, "USSR", 2)
	g.AddPieceTemplate("AAA", models.Land, 0, 0, 1, 5)
	c := NewGameController(g)
	c.StartGame()
	return g, c
}

// An ally that retakes a lost province hands it back. Its own troops stay
// its own; what the enemy left behind goes to the province's owner.
func TestLiberation_ReturnsProvinceToOriginalOwner(t *testing.T) {
	g, c := liberationGame(t)
	uk := g.Players["UK"]
	usa := g.Players["USA"]

	if err := c.CaptureTerritory("Egypt", "Germany"); err != nil {
		t.Fatal(err)
	}
	if g.Board["Egypt"].Owner.Name != "Germany" {
		t.Fatalf("Germany failed to take Egypt")
	}
	// Germany leaves an AA gun; the USA arrives with infantry.
	g.PlacePieces("Egypt", "AAA", 1)
	aaa := g.NextPieceID - 1
	g.PlacePieces("Egypt", "infantry", 2)
	for _, id := range g.Board["Egypt"].Pieces {
		if g.Pieces[id].Name == "infantry" {
			g.Pieces[id].Owner = usa
		}
	}

	if err := c.CaptureTerritory("Egypt", "USA"); err != nil {
		t.Fatal(err)
	}
	if g.Board["Egypt"].Owner != uk {
		t.Errorf("liberated Egypt belongs to %s, want UK", g.Board["Egypt"].Owner.Name)
	}
	if g.Pieces[aaa].Owner != uk {
		t.Errorf("the captured AA gun belongs to %s, want UK", g.Pieces[aaa].Owner.Name)
	}
	for _, id := range g.Board["Egypt"].Pieces {
		if g.Pieces[id].Name == "infantry" && g.Pieces[id].Owner != usa {
			t.Errorf("American infantry changed allegiance to %s", g.Pieces[id].Owner.Name)
		}
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("state invalid after liberation: %v", problems)
	}
}

// While the owner's capital is enemy-held, the liberator keeps what it takes.
// Freeing the capital returns it, and every province held in trust with it.
func TestLiberation_WaitsForTheCapital(t *testing.T) {
	g, c := liberationGame(t)
	ussr := g.Players["USSR"]
	usa := g.Players["USA"]

	for _, name := range []string{"Moscow", "Ukraine"} {
		if err := c.CaptureTerritory(name, "Germany"); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.CaptureTerritory("Ukraine", "USA"); err != nil {
		t.Fatal(err)
	}
	if g.Board["Ukraine"].Owner != usa {
		t.Fatalf("with Moscow in German hands, Ukraine should stay with the USA, got %s",
			g.Board["Ukraine"].Owner.Name)
	}

	if err := c.CaptureTerritory("Moscow", "USA"); err != nil {
		t.Fatal(err)
	}
	if g.Board["Moscow"].Owner != ussr {
		t.Errorf("a liberated capital belongs to %s, want USSR", g.Board["Moscow"].Owner.Name)
	}
	if g.Board["Ukraine"].Owner != ussr {
		t.Errorf("Ukraine should come home with the capital, got %s", g.Board["Ukraine"].Owner.Name)
	}
	if problems := g.Validate(); len(problems) > 0 {
		t.Errorf("state invalid: %v", problems)
	}
}

// Enemy conquest is conquest: nothing is handed to anyone.
func TestLiberation_EnemyCaptureKeepsTheGround(t *testing.T) {
	g, c := liberationGame(t)
	if err := c.CaptureTerritory("Egypt", "Japan"); err != nil {
		t.Fatal(err)
	}
	if g.Board["Egypt"].Owner.Name != "Japan" {
		t.Errorf("Japan's conquest went to %s", g.Board["Egypt"].Owner.Name)
	}
	if g.Board["Egypt"].OriginalOwner.Name != "UK" {
		t.Errorf("original owner rewritten to %s", g.Board["Egypt"].OriginalOwner.Name)
	}
}

// A neutral country belongs to whoever first brings it into the war; an ally
// who later frees it hands it back to that power.
func TestLiberation_NeutralAdoptsItsFirstConqueror(t *testing.T) {
	g, c := liberationGame(t)
	g.AddTerritory("Turkey", models.Land, "Neutral", 1)
	if err := c.CaptureTerritory("Turkey", "Germany"); err != nil {
		t.Fatal(err)
	}
	if g.Board["Turkey"].OriginalOwner.Name != "Germany" {
		t.Fatalf("Turkey's original owner is %s, want Germany", g.Board["Turkey"].OriginalOwner.Name)
	}
	if err := c.CaptureTerritory("Turkey", "UK"); err != nil {
		t.Fatal(err)
	}
	if err := c.CaptureTerritory("Turkey", "Japan"); err != nil {
		t.Fatal(err)
	}
	if g.Board["Turkey"].Owner.Name != "Germany" {
		t.Errorf("Japan retaking Turkey should return it to Germany, got %s", g.Board["Turkey"].Owner.Name)
	}
}

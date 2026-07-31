package game

import (
	"strings"
	"testing"
)

// Every operation gets a codename from its side's vendored list, assigned
// deterministically so a replayed game christens identically.
func TestCodenames_SideFlavouredAndDeterministic(t *testing.T) {
	if len(alliedNames) < 20 || len(axisNames) < 20 {
		t.Fatalf("wordlists too thin: allied %d, axis %d", len(alliedNames), len(axisNames))
	}

	// Same side, same id, same name -- and consecutive ids differ.
	if operationName("Axis", 3) != operationName("Axis", 3) {
		t.Error("codenames are not deterministic")
	}
	if operationName("Axis", 3) == operationName("Axis", 4) {
		t.Error("consecutive operations share a codename")
	}

	// Flavour: the two sides draw from different lists.
	if operationName("Axis", 1) == operationName("Allies", 1) {
		t.Error("both sides drew the same first codename; the lists should differ")
	}
	for _, name := range axisNames {
		for _, other := range alliedNames {
			if name == other {
				t.Fatalf("codename %q appears in both lists", name)
			}
		}
	}
}

// Plans made through the controller carry their codename into the transcript.
func TestCodenames_AppearOnPlans(t *testing.T) {
	g, gc := invasionBoard(t)
	player := g.Players["Germany"]
	if err := g.PlacePieces("Home", "infantry", 2); err != nil {
		t.Fatalf("troops: %v", err)
	}

	npc := NewSeededNPCAIPlayer("Germany", "normal", 1)
	npc.ReviewPlans(gc, player, NewGameTranscript("t"))
	npc.ReviewDefences(gc, player, NewGameTranscript("t"))

	plans := gc.Plans.Active("Germany")
	if len(plans) == 0 {
		t.Fatal("no plan formed")
	}
	if plans[0].Codename == "" {
		t.Error("an invasion plan went unchristened")
	}
	if !strings.Contains(plans[0].Describe(), "Operation "+plans[0].Codename) {
		t.Errorf("transcript line %q does not carry the codename", plans[0].Describe())
	}
	// Germany is Axis: its operations draw from the Axis list.
	found := false
	for _, name := range axisNames {
		if name == plans[0].Codename {
			found = true
		}
	}
	if !found {
		t.Errorf("Axis operation named %q, which is not on the Axis list", plans[0].Codename)
	}

	for _, defence := range gc.Plans.Defences("Germany") {
		if defence.Codename == "" {
			t.Errorf("defence of %s went unchristened", defence.Territory)
		}
	}

	// Naval plans are christened too.
	squadron := gc.Plans.AddNaval(&NavalPlan{Power: "Germany", Mission: NavalReturn, Station: "Home Sea"})
	if squadron.Codename == "" {
		t.Error("a squadron went unchristened")
	}
}

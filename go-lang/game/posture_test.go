package game

import "testing"

// The posture table is the dial that makes the computer players behave unlike
// one another. These pin the relationships that were asked for rather than the
// exact numbers, so the weights can be retuned without rewriting the tests.
func TestPosture_RelativeTemperaments(t *testing.T) {
	germany := PostureFor("Germany")
	italy := PostureFor("Italy")
	usa := PostureFor("USA")
	ussr := PostureFor("USSR")
	uk := PostureFor("UK")
	japan := PostureFor("Japan")

	if germany.Share("offence") <= italy.Share("offence") {
		t.Error("Germany should press harder than Italy")
	}
	if italy.Share("defence") <= germany.Share("defence") {
		t.Error("Italy should spend more on defence than Germany")
	}
	if italy.Garrison <= germany.Garrison {
		t.Error("Italy should garrison more deeply than Germany")
	}
	if usa.Share("buildup") <= germany.Share("buildup") {
		t.Error("the United States should build more than Germany")
	}
	if ussr.Share("buildup") <= germany.Share("buildup") {
		t.Error("the Soviet Union should build more than Germany")
	}
	// Japan and the UK sit between the extremes.
	for _, middle := range []Posture{japan, uk} {
		if middle.Share("offence") >= germany.Share("offence") {
			t.Errorf("%s should be less aggressive than Germany", middle.Name)
		}
		if middle.Share("defence") >= italy.Share("defence") {
			t.Errorf("%s should be less defensive than Italy", middle.Name)
		}
	}
}

// A board may field powers this build has never heard of.
func TestPosture_UnknownPowerGetsSomethingSensible(t *testing.T) {
	posture := PostureFor("Atlantis")

	if posture.Share("defence") <= 0 || posture.Share("offence") <= 0 {
		t.Error("an unknown power should still have a workable balance")
	}
	if posture.Garrison <= 0 {
		t.Error("an unknown power should still garrison")
	}
}

func TestPosture_BudgetSplitsWholeAmount(t *testing.T) {
	for _, name := range []string{"Germany", "Italy", "USA", "Atlantis"} {
		defence, buildup, offence := PostureFor(name).Budget(100)
		if defence+buildup+offence != 100 {
			t.Errorf("%s: split %d+%d+%d does not account for the whole budget",
				name, defence, buildup, offence)
		}
		if defence < 0 || buildup < 0 || offence < 0 {
			t.Errorf("%s: negative share in %d/%d/%d", name, defence, buildup, offence)
		}
	}
}

func TestPosture_CaseAndSpacingDoNotMatter(t *testing.T) {
	if PostureFor("  germany  ").Name != PostureFor("Germany").Name {
		t.Error("power lookup should ignore case and surrounding space")
	}
}

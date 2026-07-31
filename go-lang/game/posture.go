package game

import "strings"

// How a power divides its effort between holding what it has, massing force,
// and mounting operations overseas.
//
// This is the dial that makes the computer players behave differently from one
// another rather than all playing the same game with different coloured pieces.
// A defensive power garrisons heavily and rarely sails; an aggressive one keeps
// a thin garrison and spends on expeditions.

// Posture is one power's balance of priorities. The three shares are relative
// weights on the production budget, not percentages, and are normalised on use.
type Posture struct {
	Name string

	// Defence is spending on garrisons: anti-aircraft over factories, infantry
	// in the cities that matter.
	Defence float64
	// Buildup is the general-purpose army, spent wherever the front needs it.
	Buildup float64
	// Offence is expeditionary work -- shipping, escorts and landing forces.
	Offence float64

	// Garrison scales how strongly a territory worth holding is garrisoned. A
	// cautious power wants a deeper garrison for the same territory.
	Garrison float64
}

// postures gives each power its temperament.
//
// Germany presses hardest and accepts a thin defence. The United States and the
// Soviet Union build the largest forces and then use them. Japan and the United
// Kingdom sit in the middle, holding what they have while still mounting
// operations. Italy is the most cautious, spending most of its production on
// staying where it is.
var postures = map[string]Posture{
	"germany": {Name: "aggressive", Defence: 0.15, Buildup: 0.35, Offence: 0.50, Garrison: 0.6},
	"usa":     {Name: "industrial", Defence: 0.20, Buildup: 0.45, Offence: 0.35, Garrison: 0.9},
	"ussr":    {Name: "industrial", Defence: 0.25, Buildup: 0.50, Offence: 0.25, Garrison: 1.0},
	"japan":   {Name: "balanced", Defence: 0.25, Buildup: 0.35, Offence: 0.40, Garrison: 0.9},
	"uk":      {Name: "balanced", Defence: 0.30, Buildup: 0.35, Offence: 0.35, Garrison: 1.0},
	"italy":   {Name: "defensive", Defence: 0.50, Buildup: 0.30, Offence: 0.20, Garrison: 1.4},
}

// balancedPosture is what an unrecognised power plays: no strong opinion.
var balancedPosture = Posture{
	Name: "balanced", Defence: 0.30, Buildup: 0.40, Offence: 0.30, Garrison: 1.0,
}

// PostureFor returns a power's temperament, falling back to something balanced
// so a board with powers this build has never heard of still plays sensibly.
func PostureFor(power string) Posture {
	if posture, ok := postures[strings.ToLower(strings.TrimSpace(power))]; ok {
		return posture
	}
	return balancedPosture
}

// Share returns the fraction of a budget a category should receive.
func (p Posture) Share(category string) float64 {
	total := p.Defence + p.Buildup + p.Offence
	if total <= 0 {
		return 0
	}
	switch category {
	case "defence":
		return p.Defence / total
	case "buildup":
		return p.Buildup / total
	case "offence":
		return p.Offence / total
	}
	return 0
}

// Budget splits a production budget three ways.
func (p Posture) Budget(total int) (defence, buildup, offence int) {
	defence = int(float64(total) * p.Share("defence"))
	offence = int(float64(total) * p.Share("offence"))
	buildup = total - defence - offence
	if buildup < 0 {
		buildup = 0
	}
	return defence, buildup, offence
}

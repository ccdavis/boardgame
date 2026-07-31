package game

// The numbers behind the computer players' judgement, named and in one place.
//
// These used to be scattered through the phase code as bare literals -- the
// odds a "normal" player demands before attacking, how much of a stack is
// committed, when a fight is abandoned. Collecting them here makes the
// temperaments visible and tunable, and keeps the phase code about *flow*
// rather than arithmetic.

// Doctrine is how one difficulty level fights.
type Doctrine struct {
	// MinOdds is the estimated success probability an attack must clear
	// before the situational modifiers (value, desperation, the clock).
	MinOdds float64

	// Stubborn fighters retreat only when losses are extreme; the rest follow
	// the general retreat evaluation.
	Stubborn bool
}

// doctrines maps the difficulty names accepted by the constructors.
var doctrines = map[string]Doctrine{
	"simple":     {MinOdds: 0.70},
	"normal":     {MinOdds: 0.60},
	"aggressive": {MinOdds: 0.40, Stubborn: true},
}

// defaultDoctrine is what an unknown difficulty string plays.
var defaultDoctrine = Doctrine{MinOdds: 0.60}

// DefaultDifficulty is the difficulty callers get when they have no opinion.
const DefaultDifficulty = "normal"

// DoctrineFor returns the fighting temperament for a difficulty name.
func DoctrineFor(difficulty string) Doctrine {
	if d, ok := doctrines[difficulty]; ok {
		return d
	}
	return defaultDoctrine
}

// Attack shaping: how the required odds bend to the situation, and how much
// of a stack an approved attack commits.
const (
	// richTargetValue is the territory value (production plus victory-city
	// weight) at which a target is worth extra risk, and richTargetDiscount
	// is how much the required odds drop for it.
	richTargetValue    = 5
	richTargetDiscount = 0.15

	// desperationTerritories is the holding below which a power fights like
	// it has nothing left to lose, dropping its required odds by
	// desperationDiscount.
	desperationTerritories = 5
	desperationDiscount    = 0.2

	// commitDefault is the fraction of a stack an attack commits;
	// commitPressing when the estimate is shaky (below pressingOdds), because
	// a marginal attack needs weight; commitCautious when it is comfortable
	// (above cautiousOdds), because a sure thing should not strip the source.
	commitDefault  = 0.5
	commitPressing = 0.7
	pressingOdds   = 0.7
	commitCautious = 0.4
	cautiousOdds   = 0.85

	// minViableOdds is the estimate below which not even a single probing
	// unit is sent from a stack too small for the fraction to round to one.
	minViableOdds = 0.5

	// victoryCityGarrison is the least units left behind in an attacker's own
	// victory city, whatever the attack wants.
	victoryCityGarrison = 3

	// vulnerableDefenceRatio: a source territory is left vulnerable if its
	// remaining defence falls below this fraction of the worst neighbouring
	// threat.
	vulnerableDefenceRatio = 0.6

	// defendedTargetPenalty is docked from a target's score once its expected
	// defence exceeds defendedTargetStrength -- still attackable, but no
	// longer a bargain.
	defendedTargetStrength = 10
	defendedTargetPenalty  = 2

	// attackTargetVCBonus is how much a victory city adds to a target's
	// score when ranking what to attack this turn. (Amphibious planning and
	// the combat estimate weigh victory cities separately, at their own
	// scales: planVCBonus and CalculateTerritoryValue.)
	attackTargetVCBonus = 15

	// maxAttacksPerTurn bounds how many separate battles one power opens in
	// a single combat phase.
	maxAttacksPerTurn = 5
)

// Purchasing: how the treasury is spent.
const (
	// treasurySpendPercent is how much of the treasury a power is willing to
	// spend in one purchase phase; the remainder rides as a cushion. A power
	// with the clock against it spends urgentSpendPercent instead -- money in
	// the bank wins nothing.
	treasurySpendPercent = 80
	urgentSpendPercent   = 95

	// factoryCashCushion is the money a power wants left over after buying a
	// factory, so the new works does not bankrupt the army; a factory is only
	// worth building somewhere producing at least factoryMinProduction.
	factoryCashCushion   = 20
	factoryMinProduction = 3

	// The general buildup's target shares: a line unit (best defence per
	// IPC), a punch unit (best attack per IPC), an air unit (best combat
	// value per IPC).
	lineShare  = 0.50
	punchShare = 0.30
	airShare   = 0.20
)

// stubbornLossRatio is the casualty fraction past which even a Stubborn
// fighter breaks off a losing battle.
const stubbornLossRatio = 0.8

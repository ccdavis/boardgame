package game

import "boardgame/models"

// Time pressure: whether the production race favours you.
//
// A war between economies is decided by rate, not by the armies of the moment.
// A side being outproduced faces odds that get worse every round it waits --
// the enemy's factories will eventually field a force that no caution can
// beat -- so its least bad move is to attack soon and aggressively, trading
// today's slightly unfavourable battle against tomorrow's hopeless one, and
// above all to knock production away from the enemy while that is still
// possible. The side winning the race wants the opposite: build as fast as
// possible, reinforce the threatened front until an attack looks dreadful,
// and let time do the fighting.
//
// Concretely: Germany presses the Soviet front early, because waiting only
// deepens the Allied production advantage; the Soviets fortify and build,
// because every quiet round is a round won; Japan strikes the Americans while
// a strike still has a chance of mattering.

// productionOfSide sums the production of every territory held by a side.
func productionOfSide(g *models.Game, side string) int {
	if side == "" {
		return 0
	}
	total := 0
	for _, name := range g.PlayerOrder {
		player := g.Players[name]
		if player == nil || player.Side != side {
			continue
		}
		for _, territory := range player.Territories {
			total += territory.Production
		}
	}
	return total
}

// timePressure returns the enemy side's production rate over the player's
// own side's. Above 1: we are being outproduced and time works against us.
// Below 1: the race is ours and patience pays.
func timePressure(g *models.Game, player *models.Player) float64 {
	if player == nil || player.Side == "" {
		return 1
	}
	ours := productionOfSide(g, player.Side)

	theirs := 0
	counted := make(map[string]bool)
	for _, name := range g.PlayerOrder {
		other := g.Players[name]
		if other == nil || other.Side == "" || other.Side == player.Side || counted[other.Side] {
			continue
		}
		counted[other.Side] = true
		theirs += productionOfSide(g, other.Side)
	}

	if ours <= 0 {
		return 2 // no economy at all: as desperate as the scale goes
	}
	return float64(theirs) / float64(ours)
}

// pressureThreshold adjusts an attack's required success odds for the clock.
//
// Outproduced, the bar drops -- a fight at slightly unfavourable odds today
// beats the same fight at terrible odds in five rounds -- but never below a
// floor: desperation is not an argument for suicide. Winning the race, the
// bar rises a little; there is no reason to take a marginal fight that
// patience will turn into a sure one.
func pressureThreshold(base, pressure float64) float64 {
	switch {
	case pressure > 1.05:
		urgency := (pressure - 1) * 0.4
		if urgency > 0.2 {
			urgency = 0.2
		}
		base -= urgency
	case pressure < 0.95:
		base += 0.05
	}
	if base < 0.35 {
		base = 0.35
	}
	return base
}

// pressureFrontMargin is how far above mere equality a front should be held.
//
// The side winning the production race garrisons its contact fronts past
// equality -- the point is to make the outproduced enemy's necessary attack
// as difficult as possible. The side losing the race holds at equality and
// puts the difference into the attacks it cannot afford to postpone.
func pressureFrontMargin(pressure float64) float64 {
	if pressure < 0.95 {
		return 1.25
	}
	return 1.0
}

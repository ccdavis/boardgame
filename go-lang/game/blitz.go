package game

import (
	"boardgame/models"
)

// Blitzing: a tank may drive through an enemy territory that nobody is
// defending and keep going, taking it on the way.
//
// This replaces a 250-line "blitz subsystem" that no production code ever
// called, whose ExecuteTankBlitz flipped a territory's owner without moving the
// tank and whose enemy-unit check counted *every* piece present, friendly ones
// included. Blitzing is a property of movement, not a subsystem, so it lives
// with movement and is consulted by the pathfinder.

// canBlitzThrough reports whether a piece may pass through a territory it does
// not own, taking it as it goes.
//
// The territory must be enemy-held land with nobody in it. An empty enemy
// territory offers no resistance; anything garrisoned has to be fought for,
// which is a combat move ending there rather than a blitz through it.
func canBlitzThrough(g *models.Game, piece *models.Piece, territory *models.Territory, mover *models.Player) bool {
	if piece == nil || territory == nil || mover == nil {
		return false
	}
	if !g.Units().For(piece).CanBlitz {
		return false
	}
	if territory.Terrain != models.Land {
		return false
	}
	// Our own and allied territory is passable anyway; this is about enemy land.
	if territory.Owner == mover || areAllies(territory.Owner, mover) {
		return false
	}
	// Neutrals are not blitzed; entering one is its own decision.
	if territory.NeutralType != models.NotNeutral {
		return false
	}

	// Defended means defended by an enemy. Counting friendly units here -- as
	// the old implementation did, by counting every piece -- meant an overflying
	// friendly aircraft was enough to forbid the blitz.
	for _, pieceID := range territory.Pieces {
		occupant := g.Pieces[pieceID]
		if occupant == nil {
			continue
		}
		if occupant.Owner == mover || areAllies(occupant.Owner, mover) {
			continue
		}
		if g.Units().For(occupant).IsStructure {
			continue // an undefended factory does not hold a territory
		}
		return false
	}
	return true
}

// BlitzedTerritories returns the enemy territories a piece would take by
// passing through them along a path, in order.
//
// The destination is excluded: arriving somewhere is a normal attack or move,
// not a blitz.
func BlitzedTerritories(g *models.Game, piece *models.Piece, path []string, mover *models.Player) []string {
	if len(path) < 3 {
		return nil // no intermediate steps
	}

	blitzed := make([]string, 0)
	for _, name := range path[1 : len(path)-1] {
		territory := g.Board[name]
		if canBlitzThrough(g, piece, territory, mover) {
			blitzed = append(blitzed, name)
		}
	}
	return blitzed
}

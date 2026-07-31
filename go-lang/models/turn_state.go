package models

import "fmt"

// GetCurrentPlayer returns the player whose turn it is
func (g *Game) GetCurrentPlayer() (*Player, error) {
	player, exists := g.Players[g.CurrentPower]
	if !exists {
		return nil, fmt.Errorf("current power %s not found", g.CurrentPower)
	}
	return player, nil
}

// CountVictoryCities returns the number of victory cities controlled by each side
func (g *Game) CountVictoryCities() (axisCount, alliesCount int) {
	for _, territory := range g.Board {
		if territory.IsVictoryCity && territory.Owner != nil {
			if territory.Owner.Side == "Axis" {
				axisCount++
			} else if territory.Owner.Side == "Allies" {
				alliesCount++
			}
		}
	}
	return axisCount, alliesCount
}

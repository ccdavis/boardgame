package game

func ChangeOwnership(territory *Territory, to *Player) {
	from := territory.Owner
	territory.Owner = to
	to.Territories = append(to.Territories, territory)

	newTerritories := make([]*Territory, 0, len(from.Territories)-1)
	for _, t := range from.Territories {
		if t != territory {
			newTerritories = append(newTerritories, t)
		}
	}
	from.Territories = newTerritories
}
package parser

import (
	"boardgame/models"
	"fmt"
	"io"
	"sort"
	"strings"
)

// WriteGDF writes a game state to a .gdf format file
func WriteGDF(w io.Writer, game *models.Game) error {
	// Players section
	if err := writePlayers(w, game); err != nil {
		return err
	}

	// Turn section
	if err := writeTurn(w, game); err != nil {
		return err
	}

	// Territories section
	if err := writeTerritories(w, game); err != nil {
		return err
	}

	// Map section
	if err := writeMap(w, game); err != nil {
		return err
	}

	// Units section
	if err := writeUnits(w, game); err != nil {
		return err
	}

	// Containers section
	if err := writeContainers(w, game); err != nil {
		return err
	}

	// Placement section
	if err := writePlacement(w, game); err != nil {
		return err
	}

	return nil
}

func writePlayers(w io.Writer, game *models.Game) error {
	fmt.Fprintf(w, "Players\n")

	if len(game.PlayerOrder) > 0 {
		fmt.Fprintf(w, "%s", game.PlayerOrder[0])
		for i := 1; i < len(game.PlayerOrder); i++ {
			fmt.Fprintf(w, ", %s", game.PlayerOrder[i])
		}
	}
	fmt.Fprintf(w, ";\n\n")

	return nil
}

func writeTurn(w io.Writer, game *models.Game) error {
	fmt.Fprintf(w, "Turn %d;\n\n", game.Turn)
	return nil
}

func writeTerritories(w io.Writer, game *models.Game) error {
	fmt.Fprintf(w, "Territories\n")

	// Sort territories by name for consistent output
	var names []string
	for name := range game.Board {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		territory := game.Board[name]
		ownerName := "Neutral"
		if territory.Owner != nil {
			ownerName = territory.Owner.Name
		}

		fmt.Fprintf(w, "\t%s :%s, %s, %d;\n",
			territory.Name,
			territory.Terrain,
			ownerName,
			territory.Production)
	}

	fmt.Fprintf(w, "\n")
	return nil
}

func writeMap(w io.Writer, game *models.Game) error {
	fmt.Fprintf(w, "Map\n\n")

	// Sort territories by name for consistent output
	var names []string
	for name := range game.Board {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		territory := game.Board[name]

		if len(territory.ConnectedTo) == 0 {
			continue
		}

		fmt.Fprintf(w, "%s:", territory.Name)

		// Sort connected territories for consistent output
		var connectedNames []string
		for _, conn := range territory.ConnectedTo {
			connectedNames = append(connectedNames, conn.Name)
		}
		sort.Strings(connectedNames)

		for i, connName := range connectedNames {
			if i == 0 {
				fmt.Fprintf(w, " %s", connName)
			} else {
				fmt.Fprintf(w, ", %s", connName)
			}
		}

		fmt.Fprintf(w, ";\n")
	}

	fmt.Fprintf(w, "\n")
	return nil
}

func writeUnits(w io.Writer, game *models.Game) error {
	fmt.Fprintf(w, "Units\n\n")

	// Sort unit templates by name for consistent output
	var names []string
	for name := range game.GlobalPieceTemplates {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		piece := game.GlobalPieceTemplates[name]
		fmt.Fprintf(w, "%s: %s, %d movement, %d attack, %d defend, %d cost;\n",
			piece.Name,
			piece.Terrain,
			piece.Movement,
			piece.Attack,
			piece.Defend,
			piece.Cost)
	}

	fmt.Fprintf(w, "\n")
	return nil
}

func writeContainers(w io.Writer, game *models.Game) error {
	fmt.Fprintf(w, "Containers\n\n")

	// Find all containers (pieces with non-zero capacity)
	var containerNames []string
	for name, piece := range game.GlobalPieceTemplates {
		if piece.Capacity > 0 {
			containerNames = append(containerNames, name)
		}
	}
	sort.Strings(containerNames)

	for _, name := range containerNames {
		piece := game.GlobalPieceTemplates[name]
		fmt.Fprintf(w, "%s: %d", piece.Name, piece.Capacity)

		if len(piece.CanCarry) > 0 {
			for _, canCarry := range piece.CanCarry {
				fmt.Fprintf(w, ", %s", canCarry)
			}
		}

		fmt.Fprintf(w, ";\n")
	}

	fmt.Fprintf(w, "\n")
	return nil
}

func writePlacement(w io.Writer, game *models.Game) error {
	fmt.Fprintf(w, "Placement\n\n")

	// Sort territories by name for consistent output
	var names []string
	for name := range game.Board {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		territory := game.Board[name]

		if len(territory.Pieces) == 0 {
			continue
		}

		// Count pieces by type
		pieceCounts := make(map[string]int)
		for _, pieceID := range territory.Pieces {
			piece := game.Pieces[pieceID]
			pieceCounts[piece.Name]++
		}

		// Sort piece names for consistent output
		var pieceNames []string
		for pieceName := range pieceCounts {
			pieceNames = append(pieceNames, pieceName)
		}
		sort.Strings(pieceNames)

		fmt.Fprintf(w, "%s:", territory.Name)

		var parts []string
		for _, pieceName := range pieceNames {
			count := pieceCounts[pieceName]
			parts = append(parts, fmt.Sprintf("%d %s", count, pieceName))
		}

		fmt.Fprintf(w, " %s;\n", strings.Join(parts, ", "))
	}

	return nil
}

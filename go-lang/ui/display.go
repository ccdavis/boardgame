package ui

import (
	"boardgame/game"
	"boardgame/models"
	"fmt"
	"sort"
	"strings"
)

// DisplayGameStatus shows the current game state
func DisplayGameStatus(g *models.Game) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("Turn %d - %s - %s\n", g.Turn, g.CurrentPower, g.CurrentPhase)
	fmt.Println(strings.Repeat("=", 60))
}

// DisplayPlayerStatus shows a player's current state
func DisplayPlayerStatus(player *models.Player) {
	fmt.Printf("\n%s:\n", player.Name)
	fmt.Printf("  IPCs: %d\n", player.IPCs)
	fmt.Printf("  Territories: %d\n", len(player.Territories))
	fmt.Printf("  Type: ")
	if player.NPC {
		fmt.Println("NPC")
	} else {
		fmt.Println("Human")
	}
}

// DisplayAllPlayers shows all players' status
func DisplayAllPlayers(g *models.Game) {
	fmt.Println("\n=== Players ===")
	for _, playerName := range g.PlayerOrder {
		player := g.Players[playerName]
		DisplayPlayerStatus(player)
	}
}

// DisplayTerritory shows detailed information about a territory
func DisplayTerritory(g *models.Game, territoryName string) {
	territory, exists := g.Board[territoryName]
	if !exists {
		fmt.Printf("Territory '%s' not found\n", territoryName)
		return
	}

	fmt.Printf("\n=== %s ===\n", territory.Name)
	fmt.Printf("Owner: %s\n", territory.Owner.Name)
	fmt.Printf("Terrain: %s\n", territory.Terrain)
	fmt.Printf("Production: %d", territory.Production)
	if territory.ICDamage > 0 {
		effectiveProduction := territory.Production - territory.ICDamage
		if effectiveProduction < 0 {
			effectiveProduction = 0
		}
		fmt.Printf(" (Damaged: %d, Effective: %d)", territory.ICDamage, effectiveProduction)
	}
	fmt.Println()

	if territory.IsVictoryCity {
		fmt.Println("*** VICTORY CITY ***")
	}

	// Display pieces
	pieces := g.GetPiecesInTerritory(territoryName)
	if len(pieces) > 0 {
		fmt.Println("\nUnits:")

		// Group pieces by type but keep IDs
		piecesByType := make(map[string][]int)
		for pieceID, piece := range g.Pieces {
			// Check if piece is in this territory
			for _, territoryPieceID := range territory.Pieces {
				if territoryPieceID == pieceID {
					piecesByType[piece.Name] = append(piecesByType[piece.Name], pieceID)
					break
				}
			}
		}

		// Sort by unit name for consistent display
		unitNames := make([]string, 0, len(piecesByType))
		for name := range piecesByType {
			unitNames = append(unitNames, name)
		}
		sort.Strings(unitNames)

		for _, name := range unitNames {
			ids := piecesByType[name]
			sort.Ints(ids)
			fmt.Printf("  %d x %s [IDs: ", len(ids), name)
			for i, id := range ids {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(id)
				// Show max 10 IDs per line
				if i >= 9 && len(ids) > 10 {
					fmt.Printf("... +%d more", len(ids)-10)
					break
				}
			}
			fmt.Println("]")
		}
	} else {
		fmt.Println("\nNo units present")
	}

	// Display connections
	if len(territory.ConnectedTo) > 0 {
		fmt.Println("\nConnected to:")
		connections := make([]string, len(territory.ConnectedTo))
		for i, conn := range territory.ConnectedTo {
			connections[i] = conn.Name
		}
		sort.Strings(connections)
		for _, name := range connections {
			fmt.Printf("  - %s\n", name)
		}
	}
}

// DisplayBattle shows battle information
func DisplayBattle(battle *game.Battle) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println(battle.String())
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\nAttackers:")
	displayUnitList(battle.Attackers)

	fmt.Println("\nDefenders:")
	displayUnitList(battle.Defenders)

	fmt.Printf("\nRound: %d\n", battle.Round)
}

// displayUnitList shows a list of units with their stats
func displayUnitList(units []*models.Piece) {
	if len(units) == 0 {
		fmt.Println("  None")
		return
	}

	// Group by unit type
	unitCounts := make(map[string]struct {
		count  int
		attack int16
		defend int16
	})

	for _, unit := range units {
		info := unitCounts[unit.Name]
		info.count++
		info.attack = unit.Attack
		info.defend = unit.Defend
		unitCounts[unit.Name] = info
	}

	// Sort by unit name
	unitNames := make([]string, 0, len(unitCounts))
	for name := range unitCounts {
		unitNames = append(unitNames, name)
	}
	sort.Strings(unitNames)

	for _, name := range unitNames {
		info := unitCounts[name]
		fmt.Printf("  %d x %s (Attack: %d, Defend: %d)\n",
			info.count, name, info.attack, info.defend)
	}
}

// DisplayBattleResult shows the outcome of a battle
func DisplayBattleResult(result *game.BattleResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("BATTLE RESULT")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("Rounds: %d\n", result.Rounds)

	if result.AttackerWins {
		fmt.Println("*** ATTACKER WINS ***")
	} else if result.DefenderWins {
		fmt.Println("*** DEFENDER WINS ***")
	} else if result.AttackerRetreated {
		fmt.Println("*** ATTACKER RETREATED ***")
	} else {
		fmt.Println("Battle incomplete")
	}

	fmt.Println("\nCasualties:")
	fmt.Printf("  Attacker: %d units lost\n", len(result.AttackerCasualties))
	displayUnitList(result.AttackerCasualties)

	fmt.Printf("  Defender: %d units lost\n", len(result.DefenderCasualties))
	displayUnitList(result.DefenderCasualties)

	fmt.Println("\nSurvivors:")
	fmt.Printf("  Attackers: %d units\n", len(result.AttackersRemaining))
	displayUnitList(result.AttackersRemaining)

	fmt.Printf("  Defenders: %d units\n", len(result.DefendersRemaining))
	displayUnitList(result.DefendersRemaining)
}

// DisplayIncome shows all players' income
func DisplayIncome(controller *game.GameController) {
	fmt.Println("\n=== Player Income ===")

	for _, playerName := range controller.Game.PlayerOrder {
		player := controller.Game.Players[playerName]
		income, err := controller.CalculateIncome(playerName)
		if err != nil {
			fmt.Printf("%s: Error calculating income\n", playerName)
			continue
		}
		fmt.Printf("%s: %d IPCs (current: %d)\n", playerName, income, player.IPCs)
	}
}

// DisplayVictoryCities shows victory city control
func DisplayVictoryCities(controller *game.GameController) {
	fmt.Println("\n=== Victory Cities ===")

	axisCities, alliedCities := controller.GetVictoryCityCounts()

	fmt.Printf("Axis: %d cities\n", axisCities)
	fmt.Printf("Allies: %d cities\n", alliedCities)

	fmt.Println("\nVictory conditions:")
	fmt.Println("  - Immediate victory: 13+ cities")
	fmt.Println("  - Sustained victory: Axis 9+, Allies 10+ (held for full round)")

	winner, hasWon, _ := controller.CheckVictoryCondition()
	if hasWon {
		fmt.Printf("\n*** %s HAS WON THE GAME! ***\n", strings.ToUpper(winner))
	} else if winner != "" {
		fmt.Printf("\n%s is approaching victory...\n", winner)
	}
}

// DisplayPhaseHelp shows available commands for the current phase
func DisplayPhaseHelp(phase models.Phase) {
	fmt.Println("\n=== Available Commands ===")

	switch phase {
	case models.PurchasePhase:
		fmt.Println("  buy <unit> <quantity>    - Purchase units")
		fmt.Println("  repair <territory> <amt> - Repair industrial complex")
		fmt.Println("  done                     - Proceed to next phase")

	case models.CombatMovePhase:
		fmt.Println("  move <unit-id> <from> <to> - Move unit")
		fmt.Println("  attack <territory>         - View planned attack")
		fmt.Println("  cancel <unit-id>           - Cancel unit's move")
		fmt.Println("  done                       - Lock in moves, proceed")

	case models.ConductCombatPhase:
		fmt.Println("  view <territory>     - Show battle details")
		fmt.Println("  roll                 - Roll attack/defense dice")
		fmt.Println("  casualty <unit-type> - Select casualty")
		fmt.Println("  retreat [territory]  - Retreat from combat")
		fmt.Println("  continue             - Next round of combat")

	case models.NoncombatMovePhase:
		fmt.Println("  move <unit-id> <from> <to> - Move non-combatants")
		fmt.Println("  done                       - Proceed")

	case models.MobilizePhase:
		fmt.Println("  place <unit> <territory> <qty> - Place purchased units")
		fmt.Println("  done                           - Proceed")

	case models.CollectIncomePhase:
		fmt.Println("  (Income collected automatically)")
		fmt.Println("  done - Proceed to next player")
	}

	fmt.Println("\nGeneral commands:")
	fmt.Println("  status            - Show game state")
	fmt.Println("  board [territory] - Show board/territory info")
	fmt.Println("  units <territory> - List units in territory")
	fmt.Println("  income            - Show all players' income")
	fmt.Println("  cities            - Show victory city control")
	fmt.Println("  help              - Show this help")
	fmt.Println("  quit              - Exit game")
}

// DisplayBoard shows a summary of the board state
func DisplayBoard(g *models.Game) {
	fmt.Println("\n=== Board Summary ===")
	fmt.Printf("Total territories: %d\n", len(g.Board))
	fmt.Printf("Total pieces: %d\n", len(g.Pieces))

	// Group territories by owner
	ownershipMap := make(map[string][]*models.Territory)
	for _, territory := range g.Board {
		ownerName := territory.Owner.Name
		ownershipMap[ownerName] = append(ownershipMap[ownerName], territory)
	}

	fmt.Println("\nTerritories by owner:")
	for _, playerName := range g.PlayerOrder {
		territories := ownershipMap[playerName]
		fmt.Printf("  %s: %d territories\n", playerName, len(territories))
	}

	// Show neutral territories
	if neutralTerr, exists := ownershipMap["Neutral"]; exists {
		fmt.Printf("  Neutral: %d territories\n", len(neutralTerr))
	}
}

// DisplayPurchasedUnits shows units purchased but not yet placed
func DisplayPurchasedUnits(g *models.Game, playerName string) {
	units := g.PurchasedUnits[playerName]

	if len(units) == 0 {
		fmt.Println("\nNo purchased units to place")
		return
	}

	fmt.Println("\n=== Purchased Units (not yet placed) ===")

	// Group by type
	unitCounts := make(map[string]int)
	for _, unit := range units {
		unitCounts[unit.Type]++
	}

	// Sort by unit type
	unitTypes := make([]string, 0, len(unitCounts))
	for unitType := range unitCounts {
		unitTypes = append(unitTypes, unitType)
	}
	sort.Strings(unitTypes)

	for _, unitType := range unitTypes {
		count := unitCounts[unitType]
		fmt.Printf("  %d x %s\n", count, unitType)
	}
}

// ClearScreen clears the terminal (basic implementation)
func ClearScreen() {
	fmt.Print("\033[2J\033[H")
}

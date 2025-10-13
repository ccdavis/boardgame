package ui

import (
	"boardgame/models"
	"fmt"
)

// AIController handles NPC decision-making
type AIController struct {
	Terminal *Terminal
}

// ExecuteNPCTurn runs through all phases of an NPC turn
func (ai *AIController) ExecuteNPCTurn() error {
	player, err := ai.Terminal.Controller.GetCurrentPlayer()
	if err != nil {
		return err
	}

	fmt.Printf("\n")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  %s (Computer) is taking their turn...\n", player.Name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Execute all 6 phases
	for ai.Terminal.Controller.Game.CurrentPower == player.Name {
		phase := ai.Terminal.Controller.Game.CurrentPhase

		switch phase {
		case models.PurchasePhase:
			err = ai.executePurchasePhase()
		case models.CombatMovePhase:
			err = ai.executeCombatMovePhase()
		case models.ConductCombatPhase:
			err = ai.executeConductCombatPhase()
		case models.NoncombatMovePhase:
			err = ai.executeNoncombatMovePhase()
		case models.MobilizePhase:
			err = ai.executeMobilizePhase()
		case models.CollectIncomePhase:
			err = ai.executeCollectIncomePhase()
		}

		if err != nil {
			return fmt.Errorf("%s AI error in %s: %v", player.Name, phase, err)
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  %s has completed their turn\n", player.Name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return nil
}

// executePurchasePhase handles NPC purchasing
func (ai *AIController) executePurchasePhase() error {
	player, _ := ai.Terminal.Controller.GetCurrentPlayer()

	fmt.Printf("📦 Purchase Phase\n")
	fmt.Printf("   IPCs available: %d\n", player.IPCs)

	// Simple AI: spend about half IPCs on infantry (3 IPCs each)
	infantryCount := player.IPCs / 6
	if infantryCount > 0 {
		err := ai.Terminal.Controller.PurchaseUnit("infantry", infantryCount)
		if err == nil {
			fmt.Printf("   → Purchased %d infantry (%d IPCs)\n", infantryCount, infantryCount*3)
		}
	} else {
		fmt.Println("   → No purchases")
	}

	// Advance phase
	return ai.Terminal.Controller.AdvancePhase()
}

// executeCombatMovePhase handles NPC combat moves
func (ai *AIController) executeCombatMovePhase() error {
	fmt.Println("⚔️  Combat Move Phase")
	fmt.Println("   → No attacks planned")

	// For now, AI doesn't attack
	// TODO: Implement basic combat AI
	err := ai.Terminal.Controller.ExecuteCombatMoves()
	if err != nil {
		return err
	}

	return ai.Terminal.Controller.AdvancePhase()
}

// executeConductCombatPhase handles NPC combat resolution
func (ai *AIController) executeConductCombatPhase() error {
	fmt.Println("💥 Conduct Combat Phase")

	// Check if there are battles (the AI might be defending or attacking human player)
	if len(ai.Terminal.Controller.PendingBattles) > 0 {
		fmt.Printf("   → %d battle(s) to resolve\n", len(ai.Terminal.Controller.PendingBattles))

		// Auto-resolve all battles
		territories := make([]string, 0, len(ai.Terminal.Controller.PendingBattles))
		for territory := range ai.Terminal.Controller.PendingBattles {
			territories = append(territories, territory)
		}

		for _, territory := range territories {
			battle := ai.Terminal.Controller.PendingBattles[territory]

			// Get player objects to check if human is involved
			attacker := ai.Terminal.Controller.Game.Players[battle.AttackerID]
			defender := ai.Terminal.Controller.Game.Players[battle.DefenderID]

			// Check if human player is involved
			humanInvolved := !attacker.NPC || !defender.NPC

			if humanInvolved {
				fmt.Println()
				fmt.Println("   ⚠️  ═══════════════════════════════════════════════════")
				fmt.Printf("   ⚠️  BATTLE INVOLVING YOUR FORCES AT %s!\n", territory)
				fmt.Println("   ⚠️  ═══════════════════════════════════════════════════")
			}

			fmt.Printf("   → Battle at %s: %s vs %s\n",
				territory, attacker.Name, defender.Name)

			// Show battle details if human is involved
			if humanInvolved {
				DisplayBattle(battle)
				fmt.Println()
				fmt.Println("   Press Enter to resolve battle...")
				ai.Terminal.Reader.ReadString('\n')
			}

			result, err := ai.Terminal.Controller.ResolveBattle(territory, nil)
			if err != nil {
				fmt.Printf("      Error: %v\n", err)
				continue
			}

			if result.AttackerWins {
				fmt.Printf("      Attacker wins! (Rounds: %d)\n", result.Rounds)
				if !attacker.NPC {
					fmt.Println("      🎉 Victory! You captured the territory!")
				} else if !defender.NPC {
					fmt.Println("      💀 Defeat! You lost the territory!")
				}
			} else {
				fmt.Printf("      Defender wins! (Rounds: %d)\n", result.Rounds)
				if !defender.NPC {
					fmt.Println("      🛡️  Victory! You defended successfully!")
				} else if !attacker.NPC {
					fmt.Println("      ⚔️  Defeat! Your attack failed!")
				}
			}

			if humanInvolved {
				DisplayBattleResult(result)
				fmt.Println()
			}
		}
	} else {
		fmt.Println("   → No battles")
	}

	return ai.Terminal.Controller.AdvancePhase()
}

// executeNoncombatMovePhase handles NPC noncombat moves
func (ai *AIController) executeNoncombatMovePhase() error {
	fmt.Println("🚚 Noncombat Move Phase")
	fmt.Println("   → No moves")

	// For now, AI doesn't make noncombat moves
	err := ai.Terminal.Controller.ExecuteNoncombatMoves()
	if err != nil {
		return err
	}

	return ai.Terminal.Controller.AdvancePhase()
}

// executeMobilizePhase handles NPC unit placement
func (ai *AIController) executeMobilizePhase() error {
	player, _ := ai.Terminal.Controller.GetCurrentPlayer()

	fmt.Println("🏭 Mobilize Phase")

	purchased := ai.Terminal.Controller.Game.PurchasedUnits[player.Name]
	if len(purchased) == 0 {
		fmt.Println("   → No units to place")
		return ai.Terminal.Controller.AdvancePhase()
	}

	fmt.Printf("   → Placing %d unit(s)\n", len(purchased))

	// Find a territory with production capacity (simple AI: use first available)
	var placementTerritory string
	for _, territory := range player.Territories {
		if territory.Production > 0 {
			placementTerritory = territory.Name
			break
		}
	}

	if placementTerritory == "" {
		fmt.Println("   → Warning: No territory with production capacity found")
		return ai.Terminal.Controller.AdvancePhase()
	}

	// Place all purchased units
	for _, unit := range purchased {
		err := ai.Terminal.Controller.MobilizeUnit(placementTerritory, unit.Type)
		if err != nil {
			// Skip this unit if placement fails
			continue
		}
	}

	fmt.Printf("   → Placed units at %s\n", placementTerritory)

	return ai.Terminal.Controller.AdvancePhase()
}

// executeCollectIncomePhase handles NPC income collection
func (ai *AIController) executeCollectIncomePhase() error {
	player, _ := ai.Terminal.Controller.GetCurrentPlayer()

	fmt.Println("💰 Collect Income Phase")

	// Collect income
	err := ai.Terminal.Controller.CollectIncome()
	if err != nil {
		return err
	}

	fmt.Printf("   → Collected income (Total IPCs: %d)\n", player.IPCs)

	// Advance to next turn
	return ai.Terminal.Controller.AdvancePhase()
}

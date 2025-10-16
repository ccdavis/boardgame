package ui

import (
	"boardgame/game"
	"boardgame/models"
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// UIMode represents the interface mode
type UIMode int

const (
	TutorialMode UIMode = iota
	ExpertMode
)

func (m UIMode) String() string {
	if m == TutorialMode {
		return "Tutorial"
	}
	return "Expert"
}

// Terminal handles user interaction
type Terminal struct {
	Controller   *game.GameController
	Reader       *bufio.Reader
	Running      bool
	Mode         UIMode
	AIController *AIController
}

// NewTerminal creates a new terminal interface
func NewTerminal(controller *game.GameController) *Terminal {
	terminal := &Terminal{
		Controller: controller,
		Reader:     bufio.NewReader(os.Stdin),
		Running:    true,
		Mode:       TutorialMode, // Default to tutorial mode
	}
	terminal.AIController = &AIController{Terminal: terminal}
	return terminal
}

// Run starts the main terminal loop
func (t *Terminal) Run() error {
	fmt.Println("=== Axis & Allies 1942 ===")
	fmt.Printf("Mode: %s (type 'mode' to toggle)\n", t.Mode)
	fmt.Println("Type 'help' for available commands")

	// Ask which nation to play
	err := t.selectPlayerNation()
	if err != nil {
		return err
	}

	// Start the game
	err = t.Controller.StartGame()
	if err != nil {
		return fmt.Errorf("failed to start game: %v", err)
	}

	for t.Running {
		t.displayPhaseHeader()

		// Get current player
		player, err := t.Controller.GetCurrentPlayer()
		if err != nil {
			return err
		}

		// If NPC, prompt then execute AI turn
		if player.NPC {
			// Prompt human to start watching NPC turn
			fmt.Println()
			fmt.Println("┌─────────────────────────────────────────────────────────┐")
			fmt.Printf("│   Next up: %s (Computer)\n", player.Name)
			fmt.Println("└─────────────────────────────────────────────────────────┘")
			fmt.Print("Press Enter to watch their turn... ")
			t.Reader.ReadString('\n')

			err = t.AIController.ExecuteNPCTurn()
			if err != nil {
				return fmt.Errorf("NPC turn error: %v", err)
			}

			// Pause after NPC turn completes
			fmt.Println()
			fmt.Print("Press Enter to continue...")
			t.Reader.ReadString('\n')
			fmt.Println()

			continue
		}

		// Human player turn
		fmt.Printf("\n%s > ", player.Name)
		command, err := t.Reader.ReadString('\n')
		if err != nil {
			return err
		}

		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		err = t.ProcessCommand(command)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}

// ProcessCommand parses and executes a command
func (t *Terminal) ProcessCommand(command string) error {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil
	}

	cmd := strings.ToLower(parts[0])

	// General commands (available in any phase)
	switch cmd {
	case "quit", "exit":
		fmt.Println("Exiting game...")
		t.Running = false
		return nil

	case "mode":
		t.toggleMode()
		return nil

	case "help":
		t.displayPhaseHelp()
		return nil

	case "status":
		DisplayAllPlayers(t.Controller.Game)
		return nil

	case "board":
		if len(parts) > 1 {
			territoryName := strings.Join(parts[1:], " ")
			DisplayTerritory(t.Controller.Game, territoryName)
		} else {
			DisplayBoard(t.Controller.Game)
		}
		return nil

	case "units":
		if len(parts) < 2 {
			return fmt.Errorf("usage: units <territory>")
		}
		territoryName := strings.Join(parts[1:], " ")
		DisplayTerritory(t.Controller.Game, territoryName)
		return nil

	case "income":
		DisplayIncome(t.Controller)
		return nil

	case "cities":
		DisplayVictoryCities(t.Controller)
		return nil

	case "done":
		return t.advancePhase()
	}

	// Phase-specific commands
	switch t.Controller.Game.CurrentPhase {
	case models.PurchasePhase:
		return t.processPurchaseCommand(parts)

	case models.CombatMovePhase:
		return t.processCombatMoveCommand(parts)

	case models.ConductCombatPhase:
		return t.processConductCombatCommand(parts)

	case models.NoncombatMovePhase:
		return t.processNoncombatMoveCommand(parts)

	case models.MobilizePhase:
		return t.processMobilizeCommand(parts)

	case models.CollectIncomePhase:
		return t.processCollectIncomeCommand(parts)
	}

	return fmt.Errorf("unknown command: %s", cmd)
}

// advancePhase advances to the next phase
func (t *Terminal) advancePhase() error {
	phase := t.Controller.Game.CurrentPhase

	// Execute phase-specific actions before advancing
	switch phase {
	case models.CombatMovePhase:
		// Execute all combat moves and set up battles
		err := t.Controller.ExecuteCombatMoves()
		if err != nil {
			return fmt.Errorf("failed to execute combat moves: %v", err)
		}

		if len(t.Controller.PendingBattles) > 0 {
			fmt.Printf("\n%d battle(s) created\n", len(t.Controller.PendingBattles))
			for territory := range t.Controller.PendingBattles {
				fmt.Printf("  - %s\n", territory)
			}
		}

	case models.NoncombatMovePhase:
		// Execute all noncombat moves
		err := t.Controller.ExecuteNoncombatMoves()
		if err != nil {
			return fmt.Errorf("failed to execute noncombat moves: %v", err)
		}

	case models.CollectIncomePhase:
		// Collect income before advancing
		err := t.Controller.CollectIncome()
		if err != nil {
			return fmt.Errorf("failed to collect income: %v", err)
		}

		player, _ := t.Controller.GetCurrentPlayer()
		fmt.Printf("\n%s collected income!\n", player.Name)
		DisplayPlayerStatus(player)
	}

	err := t.Controller.AdvancePhase()
	if err != nil {
		return err
	}

	fmt.Println("\nAdvanced to next phase")
	return nil
}

// processPurchaseCommand handles purchase phase commands
func (t *Terminal) processPurchaseCommand(parts []string) error {
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "buy":
		if len(parts) < 3 {
			return fmt.Errorf("usage: buy <unit> <quantity>")
		}
		unitType := parts[1]
		quantity, err := strconv.Atoi(parts[2])
		if err != nil {
			return fmt.Errorf("invalid quantity: %s", parts[2])
		}

		err = t.Controller.PurchaseUnit(unitType, quantity)
		if err != nil {
			return err
		}

		fmt.Printf("Purchased %d x %s\n", quantity, unitType)
		player, _ := t.Controller.GetCurrentPlayer()
		fmt.Printf("Remaining IPCs: %d\n", player.IPCs)

		return nil

	case "repair":
		if len(parts) < 3 {
			return fmt.Errorf("usage: repair <territory> <amount>")
		}
		territoryName := parts[1]
		amount, err := strconv.Atoi(parts[2])
		if err != nil {
			return fmt.Errorf("invalid amount: %s", parts[2])
		}

		err = t.Controller.RepairIndustrialComplex(territoryName, amount)
		if err != nil {
			return err
		}

		fmt.Printf("Repaired %d damage at %s\n", amount, territoryName)
		player, _ := t.Controller.GetCurrentPlayer()
		fmt.Printf("Remaining IPCs: %d\n", player.IPCs)

		return nil

	case "show":
		player, _ := t.Controller.GetCurrentPlayer()
		DisplayPurchasedUnits(t.Controller.Game, player.Name)
		return nil

	default:
		return fmt.Errorf("unknown purchase command: %s", cmd)
	}
}

// processCombatMoveCommand handles combat move phase commands
func (t *Terminal) processCombatMoveCommand(parts []string) error {
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "move":
		// If no arguments, use interactive mode
		if len(parts) == 1 {
			return t.promptInteractiveMove()
		}

		// Otherwise, use traditional syntax
		if len(parts) < 4 {
			return fmt.Errorf("usage: move <piece-id> <from> <to>, or just 'move' for interactive mode")
		}
		pieceID, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid piece ID: %s", parts[1])
		}
		from := parts[2]
		to := parts[3]

		err = t.Controller.PlanMove(pieceID, from, to)
		if err != nil {
			// Provide helpful context with the error
			piece, exists := t.Controller.Game.Pieces[pieceID]
			if exists {
				return fmt.Errorf("cannot move %s (ID:%d) from %s to %s: %v", piece.Name, pieceID, from, to, err)
			}
			return fmt.Errorf("move failed: %v", err)
		}

		piece := t.Controller.Game.Pieces[pieceID]
		fmt.Printf("✓ Planned move: %s (ID:%d) from %s to %s\n", piece.Name, pieceID, from, to)
		return nil

	case "attack":
		if len(parts) < 2 {
			// Show all planned attacks
			attacks := t.Controller.GetPlannedAttacks()
			if len(attacks) == 0 {
				fmt.Println("No attacks planned")
			} else {
				fmt.Println("Planned attacks:")
				for _, territory := range attacks {
					fmt.Printf("  - %s\n", territory)
				}
			}
			return nil
		}

		territoryName := parts[1]
		t.showPlannedAttack(territoryName)
		return nil

	case "cancel":
		if len(parts) < 2 {
			return fmt.Errorf("usage: cancel <piece-id>")
		}
		pieceID, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid piece ID: %s", parts[1])
		}

		err = t.Controller.CancelMove(pieceID)
		if err != nil {
			return err
		}

		fmt.Printf("Cancelled move for piece %d\n", pieceID)
		return nil

	case "show", "moves":
		moves := t.Controller.GetPlannedMoves()
		if len(moves) == 0 {
			fmt.Println("No moves planned")
			return nil
		}

		fmt.Println("\n=== Planned Moves ===")
		for _, move := range moves {
			piece := t.Controller.Game.Pieces[move.PieceID]
			fmt.Printf("  Piece %d (%s): %s -> %s\n",
				move.PieceID, piece.Name, move.From, move.To)
		}
		return nil

	default:
		return fmt.Errorf("unknown combat move command: %s", cmd)
	}
}

// processConductCombatCommand handles conduct combat phase commands
func (t *Terminal) processConductCombatCommand(parts []string) error {
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "battles":
		// List all pending battles
		if len(t.Controller.PendingBattles) == 0 {
			fmt.Println("No battles to resolve")
			return nil
		}

		fmt.Println("\n=== Pending Battles ===")
		for territory := range t.Controller.PendingBattles {
			fmt.Printf("  - %s\n", territory)
		}
		return nil

	case "view":
		if len(parts) < 2 {
			return fmt.Errorf("usage: view <territory>")
		}
		territoryName := parts[1]

		battle, exists := t.Controller.PendingBattles[territoryName]
		if !exists {
			return fmt.Errorf("no battle in %s", territoryName)
		}

		DisplayBattle(battle)
		return nil

	case "resolve":
		if len(parts) < 2 {
			return fmt.Errorf("usage: resolve <territory>")
		}
		territoryName := parts[1]

		battle, exists := t.Controller.PendingBattles[territoryName]
		if !exists {
			return fmt.Errorf("no battle in %s", territoryName)
		}

		fmt.Printf("\n=== Resolving Battle at %s ===\n", territoryName)
		DisplayBattle(battle)

		// Resolve the battle automatically
		result, err := t.Controller.ResolveBattle(territoryName, nil)
		if err != nil {
			return err
		}

		DisplayBattleResult(result)

		if result.AttackerWins {
			fmt.Printf("\n*** Territory %s captured! ***\n", territoryName)
		}

		return nil

	case "auto":
		// Auto-resolve all battles
		if len(t.Controller.PendingBattles) == 0 {
			fmt.Println("No battles to resolve")
			return nil
		}

		fmt.Println("\n=== Auto-Resolving All Battles ===")

		// Get list of territories (need to copy keys since we modify the map)
		territories := make([]string, 0, len(t.Controller.PendingBattles))
		for territory := range t.Controller.PendingBattles {
			territories = append(territories, territory)
		}

		for _, territory := range territories {
			fmt.Printf("\n--- Battle at %s ---\n", territory)
			result, err := t.Controller.ResolveBattle(territory, nil)
			if err != nil {
				fmt.Printf("Error resolving battle: %v\n", err)
				continue
			}

			if result.AttackerWins {
				fmt.Printf("Attacker wins! Territory captured.\n")
			} else if result.DefenderWins {
				fmt.Printf("Defender wins!\n")
			}
			fmt.Printf("Rounds: %d, Attacker casualties: %d, Defender casualties: %d\n",
				result.Rounds, len(result.AttackerCasualties), len(result.DefenderCasualties))
		}

		return nil

	default:
		return fmt.Errorf("unknown combat command: %s (try: battles, view, resolve, auto)", cmd)
	}
}

// processNoncombatMoveCommand handles noncombat move phase commands
func (t *Terminal) processNoncombatMoveCommand(parts []string) error {
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "move":
		// If no arguments, use interactive mode
		if len(parts) == 1 {
			return t.promptInteractiveMove()
		}

		// Otherwise, use traditional syntax
		if len(parts) < 4 {
			return fmt.Errorf("usage: move <piece-id> <from> <to>, or just 'move' for interactive mode")
		}
		pieceID, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid piece ID: %s", parts[1])
		}
		from := parts[2]
		to := parts[3]

		err = t.Controller.PlanMove(pieceID, from, to)
		if err != nil {
			// Provide helpful context with the error
			piece, exists := t.Controller.Game.Pieces[pieceID]
			if exists {
				return fmt.Errorf("cannot move %s (ID:%d) from %s to %s: %v\nHint: Noncombat moves cannot enter enemy territories", piece.Name, pieceID, from, to, err)
			}
			return fmt.Errorf("move failed: %v", err)
		}

		piece := t.Controller.Game.Pieces[pieceID]
		fmt.Printf("✓ Planned noncombat move: %s (ID:%d) from %s to %s\n", piece.Name, pieceID, from, to)
		return nil

	case "load":
		if len(parts) < 3 {
			return fmt.Errorf("usage: load <transport-id> <piece-id>")
		}
		transportID, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid transport ID: %s", parts[1])
		}
		pieceID, err := strconv.Atoi(parts[2])
		if err != nil {
			return fmt.Errorf("invalid piece ID: %s", parts[2])
		}

		err = t.Controller.LoadUnit(transportID, pieceID)
		if err != nil {
			return err
		}

		piece := t.Controller.Game.Pieces[pieceID]
		transport := t.Controller.Game.Pieces[transportID]
		fmt.Printf("Loaded %s (ID: %d) onto %s (ID: %d)\n", piece.Name, pieceID, transport.Name, transportID)

		// Show current cargo
		cargo, _ := t.Controller.GetTransportCargo(transportID)
		fmt.Printf("%s is now carrying %d unit(s)\n", transport.Name, len(cargo))

		return nil

	case "unload":
		if len(parts) < 4 {
			return fmt.Errorf("usage: unload <transport-id> <piece-id> <destination>")
		}
		transportID, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid transport ID: %s", parts[1])
		}
		pieceID, err := strconv.Atoi(parts[2])
		if err != nil {
			return fmt.Errorf("invalid piece ID: %s", parts[2])
		}
		destination := strings.Join(parts[3:], " ")

		err = t.Controller.UnloadUnit(transportID, pieceID, destination)
		if err != nil {
			return err
		}

		piece := t.Controller.Game.Pieces[pieceID]
		transport := t.Controller.Game.Pieces[transportID]
		fmt.Printf("Unloaded %s (ID: %d) from %s (ID: %d) to %s\n",
			piece.Name, pieceID, transport.Name, transportID, destination)

		return nil

	case "cargo":
		if len(parts) < 2 {
			return fmt.Errorf("usage: cargo <transport-id>")
		}
		transportID, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid transport ID: %s", parts[1])
		}

		transport, exists := t.Controller.Game.Pieces[transportID]
		if !exists {
			return fmt.Errorf("transport %d not found", transportID)
		}

		cargo, err := t.Controller.GetTransportCargo(transportID)
		if err != nil {
			return err
		}

		fmt.Printf("\n=== %s (ID: %d) Cargo ===\n", transport.Name, transportID)
		fmt.Printf("Capacity: %d/%d\n", len(cargo), transport.Capacity)

		if len(cargo) == 0 {
			fmt.Println("Empty")
		} else {
			fmt.Println("\nCarrying:")
			for _, pieceID := range cargo {
				piece := t.Controller.Game.Pieces[pieceID]
				fmt.Printf("  ID %d: %s\n", pieceID, piece.Name)
			}
		}

		return nil

	case "cancel":
		if len(parts) < 2 {
			return fmt.Errorf("usage: cancel <piece-id>")
		}
		pieceID, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid piece ID: %s", parts[1])
		}

		err = t.Controller.CancelMove(pieceID)
		if err != nil {
			return err
		}

		fmt.Printf("Cancelled move for piece %d\n", pieceID)
		return nil

	case "show", "moves":
		moves := t.Controller.GetPlannedMoves()
		if len(moves) == 0 {
			fmt.Println("No moves planned")
			return nil
		}

		fmt.Println("\n=== Planned Noncombat Moves ===")
		for _, move := range moves {
			piece := t.Controller.Game.Pieces[move.PieceID]
			fmt.Printf("  Piece %d (%s): %s -> %s\n",
				move.PieceID, piece.Name, move.From, move.To)
		}
		return nil

	default:
		return fmt.Errorf("unknown noncombat move command: %s", cmd)
	}
}

// processMobilizeCommand handles mobilize phase commands
func (t *Terminal) processMobilizeCommand(parts []string) error {
	cmd := strings.ToLower(parts[0])

	switch cmd {
	case "place":
		if len(parts) < 4 {
			return fmt.Errorf("usage: place <unit> <territory> <quantity>")
		}
		unitType := parts[1]
		territoryName := parts[2]
		quantity, err := strconv.Atoi(parts[3])
		if err != nil {
			return fmt.Errorf("invalid quantity: %s", parts[3])
		}

		// Place each unit
		for i := 0; i < quantity; i++ {
			err = t.Controller.MobilizeUnit(territoryName, unitType)
			if err != nil {
				return fmt.Errorf("failed to place unit %d: %v", i+1, err)
			}
		}

		fmt.Printf("Placed %d x %s at %s\n", quantity, unitType, territoryName)
		return nil

	case "show":
		player, _ := t.Controller.GetCurrentPlayer()
		DisplayPurchasedUnits(t.Controller.Game, player.Name)
		return nil

	default:
		return fmt.Errorf("unknown mobilize command: %s", cmd)
	}
}

// processCollectIncomeCommand handles collect income phase commands
func (t *Terminal) processCollectIncomeCommand(parts []string) error {
	// Only 'done' is valid in this phase (handled in ProcessCommand)
	return fmt.Errorf("unknown collect income command: %s", parts[0])
}

// toggleMode switches between tutorial and expert mode
func (t *Terminal) toggleMode() {
	if t.Mode == TutorialMode {
		t.Mode = ExpertMode
		fmt.Println("\n*** Switched to Expert Mode ***")
		fmt.Println("Streamlined interface - type 'help' for commands")
	} else {
		t.Mode = TutorialMode
		fmt.Println("\n*** Switched to Tutorial Mode ***")
		fmt.Println("Detailed explanations enabled - type 'help' for guidance")
	}
}

// displayPhaseHeader shows the current phase with appropriate detail level
func (t *Terminal) displayPhaseHeader() {
	player, _ := t.Controller.GetCurrentPlayer()

	// Add prominent message for human player's turn
	if !player.NPC {
		fmt.Println()
		fmt.Println("╔═══════════════════════════════════════════════════════════╗")
		fmt.Printf("║           ★ YOUR TURN - %s ★\n", player.Name)
		fmt.Println("╚═══════════════════════════════════════════════════════════╝")
		fmt.Println()
	}

	if t.Mode == ExpertMode {
		// Expert mode: minimal, clean display
		DisplayGameStatus(t.Controller.Game)
		return
	}

	// Tutorial mode: detailed with explanations
	DisplayGameStatus(t.Controller.Game)

	fmt.Printf("IPCs: %d\n", player.IPCs)

	// Show phase-specific tips
	switch t.Controller.Game.CurrentPhase {
	case models.PurchasePhase:
		fmt.Println("\n📦 PURCHASE PHASE")
		fmt.Println("Buy new units with your IPCs. Units will be placed later in the Mobilize phase.")
		fmt.Println("Commands: buy <unit> <qty>, repair <territory> <amt>, done")

	case models.CombatMovePhase:
		fmt.Println("\n⚔️  COMBAT MOVE PHASE")
		fmt.Println("Move units to attack enemy territories.")
		fmt.Println("Commands: move (interactive), attack, show, done")

		// Auto-display player's territories with units
		t.displayPlayerUnits()

	case models.ConductCombatPhase:
		fmt.Println("\n💥 CONDUCT COMBAT PHASE")
		fmt.Println("Resolve battles in attacked territories.")
		if len(t.Controller.PendingBattles) > 0 {
			fmt.Printf("%d battle(s) to resolve: ", len(t.Controller.PendingBattles))
			first := true
			for territory := range t.Controller.PendingBattles {
				if !first {
					fmt.Print(", ")
				}
				fmt.Print(territory)
				first = false
			}
			fmt.Println()
		} else {
			fmt.Println("No battles to resolve.")
		}
		fmt.Println("Commands: battles, view <territory>, resolve <territory>, auto, done")

	case models.NoncombatMovePhase:
		fmt.Println("\n🚚 NONCOMBAT MOVE PHASE")
		fmt.Println("Reposition units that didn't attack. Cannot move into enemy territories.")
		fmt.Println("Load/unload transports to move land units across water.")
		fmt.Println("Commands: move, load, unload, cargo, show, done")

		// Auto-display player's territories with units
		t.displayPlayerUnits()

	case models.MobilizePhase:
		fmt.Println("\n🏭 MOBILIZE PHASE")
		fmt.Println("Place units purchased earlier at your industrial complexes.")
		purchased := t.Controller.Game.PurchasedUnits[player.Name]
		if len(purchased) > 0 {
			fmt.Printf("Units to place: %d\n", len(purchased))
		} else {
			fmt.Println("No units to place.")
		}
		fmt.Println("Commands: place <unit> <territory> <qty>, show, done")

	case models.CollectIncomePhase:
		fmt.Println("\n💰 COLLECT INCOME PHASE")
		income, _ := t.Controller.CalculateIncome(player.Name)
		fmt.Printf("You will collect %d IPCs from your territories.\n", income)
		fmt.Println("Command: done (to collect and end turn)")
	}
}

// displayPhaseHelp shows help text based on current mode
func (t *Terminal) displayPhaseHelp() {
	if t.Mode == ExpertMode {
		// Expert mode: compact command list
		t.displayExpertHelp()
	} else {
		// Tutorial mode: detailed explanations
		t.displayTutorialHelp()
	}
}

// displayExpertHelp shows compact command list for experts
func (t *Terminal) displayExpertHelp() {
	fmt.Println("\n=== Commands ===")

	phase := t.Controller.Game.CurrentPhase
	switch phase {
	case models.PurchasePhase:
		fmt.Println("buy <unit> <qty> | repair <terr> <amt> | done")

	case models.CombatMovePhase:
		fmt.Println("move [id from to] | attack [terr] | cancel <id> | show | done")

	case models.ConductCombatPhase:
		fmt.Println("battles | view <terr> | resolve <terr> | auto | done")

	case models.NoncombatMovePhase:
		fmt.Println("move [id from to] | load <transport> <unit> | unload <transport> <unit> <dest> | cargo <transport> | cancel <id> | show | done")

	case models.MobilizePhase:
		fmt.Println("place <unit> <terr> <qty> | show | done")

	case models.CollectIncomePhase:
		fmt.Println("done")
	}

	fmt.Println("\nGeneral: status | board [terr] | units <terr> | income | cities | mode | help | quit")
}

// displayTutorialHelp shows detailed help for beginners
func (t *Terminal) displayTutorialHelp() {
	fmt.Println("\n╔═════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                        TUTORIAL HELP                            ║")
	fmt.Println("╚═════════════════════════════════════════════════════════════════╝")

	phase := t.Controller.Game.CurrentPhase

	switch phase {
	case models.PurchasePhase:
		fmt.Println("\n📦 PURCHASE PHASE - Buy units to build your forces")
		fmt.Println("\nCommands:")
		fmt.Println("  buy <unit> <quantity>")
		fmt.Println("    Example: buy infantry 5")
		fmt.Println("    Purchases units using your IPCs. Units are placed later in Mobilize phase.")
		fmt.Println()
		fmt.Println("  repair <territory> <amount>")
		fmt.Println("    Example: repair Berlin 3")
		fmt.Println("    Repair damaged industrial complexes (1 IPC per damage point)")
		fmt.Println()
		fmt.Println("  done - Proceed to Combat Move phase")

	case models.CombatMovePhase:
		fmt.Println("\n⚔️  COMBAT MOVE PHASE - Move units to attack enemies")
		fmt.Println("\nHow it works:")
		fmt.Println("  1. Your territories and units are listed automatically")
		fmt.Println("  2. Type 'move' to start interactive movement (recommended)")
		fmt.Println("     - Select territory by number")
		fmt.Println("     - Select unit by ID")
		fmt.Println("     - Select destination by letter (A, B, C...)")
		fmt.Println("  3. Review planned attacks with 'attack' or 'show'")
		fmt.Println("  4. Type 'done' when ready - battles will be set up automatically")
		fmt.Println("\nCommands:")
		fmt.Println("  move                        - Interactive move (easy!)")
		fmt.Println("  move <piece-id> <from> <to> - Direct move command")
		fmt.Println("  attack [territory]          - View planned attacks")
		fmt.Println("  show                        - Show all planned moves")
		fmt.Println("  cancel <piece-id>           - Cancel a unit's move")
		fmt.Println("  done                        - Execute moves and create battles")

	case models.ConductCombatPhase:
		fmt.Println("\n💥 CONDUCT COMBAT PHASE - Resolve battles")
		fmt.Println("\nHow it works:")
		fmt.Println("  Battles use dice rolls - units hit on their attack/defend value or less")
		fmt.Println("  Each hit destroys one enemy unit (cheapest units are lost first)")
		fmt.Println("  Battle continues until one side is eliminated")
		fmt.Println("\nCommands:")
		fmt.Println("  battles               - List all pending battles")
		fmt.Println("  view <territory>      - See battle details before resolving")
		fmt.Println("  resolve <territory>   - Resolve a specific battle")
		fmt.Println("  auto                  - Auto-resolve all battles (recommended)")
		fmt.Println("  done                  - Proceed to Noncombat Move phase")

	case models.NoncombatMovePhase:
		fmt.Println("\n🚚 NONCOMBAT MOVE PHASE - Reposition your forces")
		fmt.Println("\nHow it works:")
		fmt.Println("  Move units that didn't attack to better positions")
		fmt.Println("  Cannot move into enemy territories (attacks are done!)")
		fmt.Println("  Units that attacked cannot move again this turn")
		fmt.Println("  Load/unload transports to move land units across water")
		fmt.Println("\nCommands:")
		fmt.Println("  move                        - Interactive move (easy!)")
		fmt.Println("  move <piece-id> <from> <to> - Direct move command")
		fmt.Println("  load <transport-id> <unit-id>")
		fmt.Println("    Example: load 42 15       - Load unit 15 onto transport 42")
		fmt.Println("    Land units must be in same or adjacent territory to transport")
		fmt.Println("  unload <transport-id> <unit-id> <destination>")
		fmt.Println("    Example: unload 42 15 France")
		fmt.Println("    Unload unit to transport's location or adjacent territory")
		fmt.Println("  cargo <transport-id>        - View what a transport is carrying")
		fmt.Println("  show                        - Show planned moves")
		fmt.Println("  cancel <piece-id>           - Cancel a move")
		fmt.Println("  done                        - Execute moves and proceed")

	case models.MobilizePhase:
		fmt.Println("\n🏭 MOBILIZE PHASE - Place purchased units")
		fmt.Println("\nHow it works:")
		fmt.Println("  Place units you bought earlier at territories with industrial complexes")
		fmt.Println("  Can only place at ICs you controlled at the start of your turn")
		fmt.Println("  Limited by territory production value and IC damage")
		fmt.Println("\nCommands:")
		fmt.Println("  place <unit> <territory> <quantity>")
		fmt.Println("    Example: place infantry Berlin 5")
		fmt.Println("  show - View units you still need to place")
		fmt.Println("  done - Proceed to Collect Income phase")

	case models.CollectIncomePhase:
		fmt.Println("\n💰 COLLECT INCOME PHASE - Gain IPCs")
		fmt.Println("\nHow it works:")
		fmt.Println("  Automatically collect IPCs equal to the production value of")
		fmt.Println("  all territories you control")
		fmt.Println("  These IPCs will be available next turn for purchasing units")
		fmt.Println("\nCommands:")
		fmt.Println("  done - Collect income and end your turn")
	}

	fmt.Println("\n─────────────────────────────────────────────────────────────────")
	fmt.Println("GENERAL COMMANDS (available anytime)")
	fmt.Println("─────────────────────────────────────────────────────────────────")
	fmt.Println("  status            - Show all players' status")
	fmt.Println("  board [territory] - Show board summary or specific territory")
	fmt.Println("  units <territory> - List all units in a territory (shows IDs)")
	fmt.Println("  income            - Show all players' income potential")
	fmt.Println("  cities            - Show victory city control")
	fmt.Println("  mode              - Toggle between Tutorial and Expert mode")
	fmt.Println("  help              - Show this help")
	fmt.Println("  quit              - Exit game")
	fmt.Println()
}

// displayPlayerUnits shows a summary of the current player's territories with units
func (t *Terminal) displayPlayerUnits() {
	player, err := t.Controller.GetCurrentPlayer()
	if err != nil {
		return
	}

	fmt.Println("\n=== Your Units ===")

	// Group territories by those with units
	territoriesWithUnits := make([]*models.Territory, 0)
	for _, territory := range player.Territories {
		pieces := t.Controller.Game.GetPiecesInTerritory(territory.Name)
		if len(pieces) > 0 {
			territoriesWithUnits = append(territoriesWithUnits, territory)
		}
	}

	if len(territoriesWithUnits) == 0 {
		fmt.Println("No units to move")
		return
	}

	// Sort by name
	sort.Slice(territoriesWithUnits, func(i, j int) bool {
		return territoriesWithUnits[i].Name < territoriesWithUnits[j].Name
	})

	// Display each territory with unit summary
	for _, territory := range territoriesWithUnits {
		pieces := t.Controller.Game.GetPiecesInTerritory(territory.Name)

		// Count units by type
		unitCounts := make(map[string]int)
		for _, piece := range pieces {
			unitCounts[piece.Name]++
		}

		// Build summary string
		unitTypes := make([]string, 0, len(unitCounts))
		for unitType := range unitCounts {
			unitTypes = append(unitTypes, unitType)
		}
		sort.Strings(unitTypes)

		summary := make([]string, 0, len(unitTypes))
		for _, unitType := range unitTypes {
			count := unitCounts[unitType]
			summary = append(summary, fmt.Sprintf("%dx%s", count, unitType))
		}

		fmt.Printf("  %s: %s\n", territory.Name, strings.Join(summary, ", "))
	}

	fmt.Println("\nType 'move' to start interactive movement, or use the traditional command format.")
}

// selectPlayerNation prompts the player to select which nation to play
func (t *Terminal) selectPlayerNation() error {
	fmt.Println("\n=== Select Your Nation ===")
	fmt.Println("Available nations:")

	// Display all players in turn order
	if len(t.Controller.Game.PlayerOrder) == 0 {
		return fmt.Errorf("no players available")
	}

	for i, name := range t.Controller.Game.PlayerOrder {
		fmt.Printf("  %d. %s\n", i+1, name)
	}

	fmt.Print("\nEnter the number of your choice: ")
	input, err := t.Reader.ReadString('\n')
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(t.Controller.Game.PlayerOrder) {
		return fmt.Errorf("invalid choice")
	}

	selectedNation := t.Controller.Game.PlayerOrder[choice-1]

	// Mark selected player as human, all others as NPCs
	for _, playerName := range t.Controller.Game.PlayerOrder {
		player := t.Controller.Game.Players[playerName]
		if playerName == selectedNation {
			player.NPC = false
			fmt.Printf("\nYou are playing as %s\n", selectedNation)
		} else {
			player.NPC = true
		}
	}

	fmt.Println("All other nations are controlled by the computer.")

	return nil
}

// getTerritoryConnectionMap returns a map of letter IDs to territory names for connections
func getTerritoryConnectionMap(territory *models.Territory) (map[string]string, []string) {
	connectionMap := make(map[string]string)
	letters := make([]string, 0, len(territory.ConnectedTo))

	// Sort connections alphabetically for consistency
	connections := make([]string, len(territory.ConnectedTo))
	for i, conn := range territory.ConnectedTo {
		connections[i] = conn.Name
	}
	sort.Strings(connections)

	// Assign letters A, B, C, etc.
	for i, name := range connections {
		letter := string(rune('A' + i))
		connectionMap[letter] = name
		letters = append(letters, letter)
	}

	return connectionMap, letters
}

// displayTerritoryWithLetterIDs shows a territory with letter-coded connections
func (t *Terminal) displayTerritoryWithLetterIDs(territoryName string) (map[string]string, error) {
	territory, exists := t.Controller.Game.Board[territoryName]
	if !exists {
		return nil, fmt.Errorf("territory '%s' not found", territoryName)
	}

	fmt.Printf("\n=== %s ===\n", territory.Name)

	// Display connections with letter IDs
	if len(territory.ConnectedTo) > 0 {
		connectionMap, letters := getTerritoryConnectionMap(territory)
		fmt.Println("\nConnected territories:")
		for _, letter := range letters {
			name := connectionMap[letter]
			owner := t.Controller.Game.Board[name].Owner.Name
			fmt.Printf("  %s. %s (Owner: %s)\n", letter, name, owner)
		}
		return connectionMap, nil
	}

	return nil, nil
}

// displayReachableTerritoriesForPiece shows only territories a specific piece can reach
func (t *Terminal) displayReachableTerritoriesForPiece(pieceID int, fromTerritory string) (map[string]string, error) {
	piece, exists := t.Controller.Game.Pieces[pieceID]
	if !exists {
		return nil, fmt.Errorf("piece %d not found", pieceID)
	}

	// Get reachable territories using the game's pathfinding
	reachable, err := game.GetReachableTerritories(t.Controller.Game, pieceID, fromTerritory)
	if err != nil {
		return nil, err
	}

	if len(reachable) == 0 {
		return make(map[string]string), nil
	}

	fmt.Printf("\n=== Destinations for %s (Movement: %d) ===\n", piece.Name, piece.Movement)

	// Sort territories alphabetically
	sort.Slice(reachable, func(i, j int) bool {
		return reachable[i].Name < reachable[j].Name
	})

	// Create letter map
	connectionMap := make(map[string]string)
	letters := make([]string, 0, len(reachable))

	for i, territory := range reachable {
		letter := string(rune('A' + i))
		connectionMap[letter] = territory.Name
		letters = append(letters, letter)
	}

	// Get current player for context
	player, _ := t.Controller.GetCurrentPlayer()
	currentPhase := t.Controller.Game.CurrentPhase

	// Display with helpful annotations
	fmt.Println("\nReachable territories:")
	for _, letter := range letters {
		name := connectionMap[letter]
		territory := t.Controller.Game.Board[name]
		owner := territory.Owner.Name

		// Add helpful context about the territory
		annotation := ""
		if owner != player.Name {
			pieces := t.Controller.Game.GetPiecesInTerritory(name)
			if len(pieces) > 0 {
				if currentPhase == models.CombatMovePhase {
					annotation = " [ATTACK]"
				} else {
					annotation = " [BLOCKED - has enemy units]"
				}
			} else {
				if currentPhase == models.CombatMovePhase {
					annotation = " [empty, can capture]"
				} else {
					annotation = " [empty neutral]"
				}
			}
		} else {
			annotation = " [friendly]"
		}

		fmt.Printf("  %s. %s (Owner: %s)%s\n", letter, name, owner, annotation)
	}

	return connectionMap, nil
}

// promptInteractiveMove guides the player through an interactive move selection
func (t *Terminal) promptInteractiveMove() error {
	player, err := t.Controller.GetCurrentPlayer()
	if err != nil {
		return err
	}

	// Step 1: Show player's territories with units
	fmt.Println("\n=== Your Territories ===")
	playerTerritories := make([]*models.Territory, 0)
	for _, territory := range player.Territories {
		pieces := t.Controller.Game.GetPiecesInTerritory(territory.Name)
		if len(pieces) > 0 {
			playerTerritories = append(playerTerritories, territory)
		}
	}

	if len(playerTerritories) == 0 {
		fmt.Println("You have no units to move")
		return nil
	}

	// Sort territories by name
	sort.Slice(playerTerritories, func(i, j int) bool {
		return playerTerritories[i].Name < playerTerritories[j].Name
	})

	for i, territory := range playerTerritories {
		pieces := t.Controller.Game.GetPiecesInTerritory(territory.Name)
		fmt.Printf("  %d. %s (%d units)\n", i+1, territory.Name, len(pieces))
	}

	// Prompt for territory selection
	fmt.Print("\nFrom which territory? (number or 'cancel'): ")
	input, err := t.Reader.ReadString('\n')
	if err != nil {
		return err
	}
	input = strings.TrimSpace(input)

	if strings.ToLower(input) == "cancel" {
		fmt.Println("Move cancelled")
		return nil
	}

	territoryChoice, err := strconv.Atoi(input)
	if err != nil || territoryChoice < 1 || territoryChoice > len(playerTerritories) {
		return fmt.Errorf("invalid territory selection")
	}

	fromTerritory := playerTerritories[territoryChoice-1]

	// Step 2: Show units in selected territory
	DisplayTerritory(t.Controller.Game, fromTerritory.Name)

	fmt.Print("\nMove which unit? (enter piece ID or 'cancel'): ")
	input, err = t.Reader.ReadString('\n')
	if err != nil {
		return err
	}
	input = strings.TrimSpace(input)

	if strings.ToLower(input) == "cancel" {
		fmt.Println("Move cancelled")
		return nil
	}

	pieceID, err := strconv.Atoi(input)
	if err != nil {
		return fmt.Errorf("invalid piece ID")
	}

	// Verify piece exists and is in the territory
	piece, exists := t.Controller.Game.Pieces[pieceID]
	if !exists {
		return fmt.Errorf("piece %d not found", pieceID)
	}

	// Step 3: Show reachable destination options for this piece
	connectionMap, err := t.displayReachableTerritoriesForPiece(pieceID, fromTerritory.Name)
	if err != nil {
		return err
	}

	if len(connectionMap) == 0 {
		fmt.Printf("No reachable territories for %s (movement: %d)\n", piece.Name, piece.Movement)
		fmt.Println("Hint: Enemies may be blocking the path, or terrain may be incompatible")
		return nil
	}

	fmt.Print("\nTo which territory? (enter letter or 'cancel'): ")
	input, err = t.Reader.ReadString('\n')
	if err != nil {
		return err
	}
	input = strings.TrimSpace(strings.ToUpper(input))

	if strings.ToLower(input) == "CANCEL" {
		fmt.Println("Move cancelled")
		return nil
	}

	toTerritoryName, exists := connectionMap[input]
	if !exists {
		return fmt.Errorf("invalid territory letter")
	}

	// Execute the move planning
	err = t.Controller.PlanMove(pieceID, fromTerritory.Name, toTerritoryName)
	if err != nil {
		return err
	}

	fmt.Printf("Planned move: %s (ID: %d) from %s to %s\n",
		piece.Name, pieceID, fromTerritory.Name, toTerritoryName)

	return nil
}

// showPlannedAttack shows details about a planned attack
func (t *Terminal) showPlannedAttack(territoryName string) {
	territory, exists := t.Controller.Game.Board[territoryName]
	if !exists {
		fmt.Printf("Territory '%s' not found\n", territoryName)
		return
	}

	// Find attacking pieces
	moves := t.Controller.GetPlannedMoves()
	attackers := make([]*models.Piece, 0)
	for _, move := range moves {
		if move.To == territoryName && move.Type == game.CombatMove {
			piece := t.Controller.Game.Pieces[move.PieceID]
			attackers = append(attackers, piece)
		}
	}

	if len(attackers) == 0 {
		fmt.Printf("No attack planned on %s\n", territoryName)
		return
	}

	fmt.Printf("\n=== Planned Attack on %s ===\n", territoryName)
	fmt.Printf("Defender: %s\n", territory.Owner.Name)

	fmt.Println("\nAttacking forces:")
	displayUnitList(attackers)

	fmt.Println("\nDefending forces:")
	defenders := t.Controller.Game.GetPiecesInTerritory(territoryName)
	displayUnitList(defenders)

	// Calculate rough strength
	attackStrength := 0
	for _, unit := range attackers {
		attackStrength += int(unit.Attack)
	}
	defenseStrength := 0
	for _, unit := range defenders {
		defenseStrength += int(unit.Defend)
	}

	fmt.Printf("\nTotal attack strength: ~%d\n", attackStrength)
	fmt.Printf("Total defense strength: ~%d\n", defenseStrength)

	if attackStrength > defenseStrength*3/2 {
		fmt.Println("Assessment: Strong attack (likely to succeed)")
	} else if attackStrength > defenseStrength {
		fmt.Println("Assessment: Moderate attack (may succeed)")
	} else {
		fmt.Println("Assessment: Weak attack (may fail)")
	}
}

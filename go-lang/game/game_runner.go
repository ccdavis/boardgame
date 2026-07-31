package game

import (
	"boardgame/models"
	"fmt"
)

// GameRunner manages running a complete game to completion
type GameRunner struct {
	Controller *GameController
	Transcript *GameTranscript
	NPCPlayers map[string]*NPCAIPlayer
	MaxTurns   int // Safety limit to prevent infinite games
}

// NewGameRunner creates a new game runner
func NewGameRunner(game *models.Game, gameTitle string) *GameRunner {
	return &GameRunner{
		Controller: NewGameController(game),
		Transcript: NewGameTranscript(gameTitle),
		NPCPlayers: make(map[string]*NPCAIPlayer),
		MaxTurns:   100, // Default max turns
	}
}

// RegisterNPC registers an NPC player with unpredictable dice.
func (gr *GameRunner) RegisterNPC(playerName string, difficulty string) {
	gr.registerNPCPlayer(playerName, NewNPCAIPlayer(playerName, difficulty))
}

// RegisterSeededNPC registers an NPC whose play follows from a seed, so a whole
// game can be replayed. Each player is given a distinct seed derived from the
// one supplied, or they would all roll identically.
func (gr *GameRunner) RegisterSeededNPC(playerName string, difficulty string, seed int64) {
	gr.registerNPCPlayer(playerName, NewSeededNPCAIPlayer(playerName, difficulty, seed))
}

func (gr *GameRunner) registerNPCPlayer(playerName string, npc *NPCAIPlayer) {
	gr.NPCPlayers[playerName] = npc

	// Mark player as NPC in the game
	if player, exists := gr.Controller.Game.Players[playerName]; exists {
		player.NPC = true
	}
}

// RunToCompletion runs the game until victory conditions are met or max turns reached
func (gr *GameRunner) RunToCompletion() (string, error) {
	game := gr.Controller.Game

	// Start the game
	err := gr.Controller.StartGame()
	if err != nil {
		return "", fmt.Errorf("failed to start game: %v", err)
	}

	gr.Transcript.LogAction("System", fmt.Sprintf("Game started with %d players", len(game.Players)))
	for _, playerName := range game.PlayerOrder {
		player := game.Players[playerName]
		gr.Transcript.LogPlayerState(player)
	}

	// Main game loop
	for game.Turn <= gr.MaxTurns {
		// Log turn start
		gr.Transcript.LogTurnStart(game.Turn, game.CurrentPower)

		// Check if current player is NPC
		currentPlayer := game.Players[game.CurrentPower]
		if !currentPlayer.NPC {
			return "", fmt.Errorf("player %s is not an NPC - human players not supported in RunToCompletion", currentPlayer.Name)
		}

		// Get NPC AI for this player
		npc, exists := gr.NPCPlayers[game.CurrentPower]
		if !exists {
			return "", fmt.Errorf("no NPC AI registered for %s", game.CurrentPower)
		}

		// Execute NPC turn
		err := npc.TakeTurn(gr.Controller, gr.Transcript)
		if err != nil {
			return "", fmt.Errorf("turn %d, player %s failed: %v", game.Turn, game.CurrentPower, err)
		}

		// Log game state periodically
		if game.Turn%5 == 0 {
			gr.Transcript.LogGameState(game)
		}

		// Check victory conditions
		winner, hasWon, err := gr.Controller.CheckVictoryCondition()
		if err != nil {
			return "", fmt.Errorf("failed to check victory: %v", err)
		}

		if hasWon {
			axisVC, alliesVC := game.CountVictoryCities()
			reason := fmt.Sprintf("%d victory cities controlled (Axis: %d, Allies: %d)",
				axisVC+alliesVC, axisVC, alliesVC)
			gr.Transcript.LogVictory(winner, reason)
			gr.Transcript.LogGameState(game)
			return winner, nil
		}

		// Safety check for infinite loops
		if game.Turn >= gr.MaxTurns {
			gr.Transcript.LogAction("System", fmt.Sprintf("Game ended after reaching max turns (%d)", gr.MaxTurns))
			// Determine winner by victory city count
			axisVC, alliesVC := game.CountVictoryCities()
			if axisVC > alliesVC {
				gr.Transcript.LogVictory("Axis", fmt.Sprintf("Max turns reached, Axis leads %d-%d", axisVC, alliesVC))
				return "Axis", nil
			} else if alliesVC > axisVC {
				gr.Transcript.LogVictory("Allies", fmt.Sprintf("Max turns reached, Allies lead %d-%d", alliesVC, axisVC))
				return "Allies", nil
			} else {
				gr.Transcript.LogAction("System", "Game ended in a draw")
				return "Draw", nil
			}
		}
	}

	return "", fmt.Errorf("game loop exited unexpectedly")
}

// RunNTurns runs the game for a specific number of player turns (not game turns)
// Each iteration is one player taking their full turn
func (gr *GameRunner) RunNTurns(numPlayerTurns int) error {
	game := gr.Controller.Game

	// Start the game if not started
	if game.CurrentPower == "" {
		err := gr.Controller.StartGame()
		if err != nil {
			return fmt.Errorf("failed to start game: %v", err)
		}
	}

	for i := 0; i < numPlayerTurns; i++ {
		currentTurn := game.Turn
		currentPlayer := game.CurrentPower

		gr.Transcript.LogTurnStart(currentTurn, currentPlayer)

		// Get NPC for current player
		npc, exists := gr.NPCPlayers[currentPlayer]
		if !exists {
			return fmt.Errorf("no NPC AI registered for %s", currentPlayer)
		}

		// Execute turn
		err := npc.TakeTurn(gr.Controller, gr.Transcript)
		if err != nil {
			return fmt.Errorf("turn %d, player %s failed: %v", currentTurn, currentPlayer, err)
		}

		// Check victory
		winner, hasWon, err := gr.Controller.CheckVictoryCondition()
		if err != nil {
			return err
		}

		if hasWon {
			axisVC, alliesVC := game.CountVictoryCities()
			reason := fmt.Sprintf("%d victory cities controlled (Axis: %d, Allies: %d)",
				axisVC+alliesVC, axisVC, alliesVC)
			gr.Transcript.LogVictory(winner, reason)
			return nil
		}
	}

	gr.Transcript.LogGameState(game)
	gr.Transcript.LogAction("System", fmt.Sprintf("Completed %d player turns", numPlayerTurns))

	return nil
}

// GetTranscriptString returns the formatted transcript
func (gr *GameRunner) GetTranscriptString() string {
	return gr.Transcript.String()
}

// PrintTranscript prints the transcript to stdout
func (gr *GameRunner) PrintTranscript() {
	fmt.Println(gr.Transcript.String())
}

// GetGameState returns current game state summary
func (gr *GameRunner) GetGameState() string {
	game := gr.Controller.Game
	axisVC, alliesVC := game.CountVictoryCities()

	summary := fmt.Sprintf("\n=== Game State ===\n")
	summary += fmt.Sprintf("Turn: %d\n", game.Turn)
	summary += fmt.Sprintf("Current Player: %s (%s)\n", game.CurrentPower, game.CurrentPhase)
	summary += fmt.Sprintf("Victory Cities: Axis=%d, Allies=%d\n", axisVC, alliesVC)
	summary += fmt.Sprintf("\nPlayers:\n")

	for _, playerName := range game.PlayerOrder {
		player := game.Players[playerName]
		summary += fmt.Sprintf("  %s (%s): %d IPCs, %d territories\n",
			player.Name, player.Side, player.IPCs, len(player.Territories))
	}

	return summary
}

// SetMaxTurns sets the maximum number of turns before game ends
func (gr *GameRunner) SetMaxTurns(maxTurns int) {
	gr.MaxTurns = maxTurns
}

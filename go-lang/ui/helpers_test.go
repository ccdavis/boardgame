package ui

import "boardgame/game"

// newTestTerminal builds a Terminal in ExpertMode.
//
// TutorialMode prompts for a y/n confirmation before advancing a phase whenever
// there are warnings (see advancePhase). Tests drive the terminal with scripted
// input that has no answer for that prompt, so a tutorial-mode terminal fails
// with EOF and silently leaves the phase unchanged. Correctness of the phase
// machinery must not depend on which UI mode is active, so tests use the mode
// that has no interactive prompts.
func newTestTerminal(controller *game.GameController) *Terminal {
	terminal := NewTerminal(controller)
	terminal.Mode = ExpertMode
	return terminal
}

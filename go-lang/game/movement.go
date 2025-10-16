package game

import (
	"boardgame/models"
	"fmt"
)

// MoveType represents the type of move
type MoveType int

const (
	CombatMove MoveType = iota
	NoncombatMove
)

// Move represents a planned unit movement
type Move struct {
	PieceID     int
	From        string
	To          string
	Type        MoveType
	Path        []string // For multi-step moves
	DistanceCost int
}

// MovementTracker tracks all moves planned during a turn
type MovementTracker struct {
	Moves           []*Move
	PiecesMovedFrom map[int]string // PieceID -> original territory
}

// NewMovementTracker creates a new movement tracker
func NewMovementTracker() *MovementTracker {
	return &MovementTracker{
		Moves:           make([]*Move, 0),
		PiecesMovedFrom: make(map[int]string),
	}
}

// ValidateMovement checks if a move is legal
func ValidateMovement(game *models.Game, pieceID int, from, to string, moveType MoveType) error {
	// Get the piece
	piece, exists := game.Pieces[pieceID]
	if !exists {
		return fmt.Errorf("piece %d not found", pieceID)
	}

	// Get territories
	fromTerritory, exists := game.Board[from]
	if !exists {
		return fmt.Errorf("territory %s not found", from)
	}

	toTerritory, exists := game.Board[to]
	if !exists {
		return fmt.Errorf("territory %s not found", to)
	}

	// Check piece is in from territory
	pieceInTerritory := false
	for _, id := range fromTerritory.Pieces {
		if id == pieceID {
			pieceInTerritory = true
			break
		}
	}
	if !pieceInTerritory {
		return fmt.Errorf("piece %d not in territory %s", pieceID, from)
	}

	// Check terrain compatibility with destination
	if err := validateTerrain(piece, toTerritory); err != nil {
		return err
	}

	// Check movement range
	if piece.Movement < 1 {
		return fmt.Errorf("piece %s cannot move (movement=0)", piece.Name)
	}

	// Note: We don't check if territories are directly connected here
	// because multi-step movement is allowed. The caller should use
	// CanReachTerritory() to verify the piece can actually reach the destination.

	return nil
}

// validateTerrain checks if a piece can move to a terrain type
func validateTerrain(piece *models.Piece, territory *models.Territory) error {
	switch piece.Terrain {
	case models.Land:
		if territory.Terrain != models.Land && territory.Terrain != models.Both {
			return fmt.Errorf("land unit %s cannot move to %s terrain", piece.Name, territory.Terrain)
		}
	case models.Water:
		if territory.Terrain != models.Water && territory.Terrain != models.Both {
			return fmt.Errorf("sea unit %s cannot move to %s terrain", piece.Name, territory.Terrain)
		}
	case models.Air:
		// Air units can move anywhere
	case models.Both:
		// Amphibious units can move anywhere
	}
	return nil
}

// areConnected checks if two territories are directly connected
func areConnected(from, to *models.Territory) bool {
	for _, connected := range from.ConnectedTo {
		if connected == to {
			return true
		}
	}
	return false
}

// CalculateMovementDistance calculates the shortest path distance between territories
func CalculateMovementDistance(game *models.Game, from, to string) (int, []string, error) {
	fromTerritory, exists := game.Board[from]
	if !exists {
		return 0, nil, fmt.Errorf("territory %s not found", from)
	}

	toTerritory, exists := game.Board[to]
	if !exists {
		return 0, nil, fmt.Errorf("territory %s not found", to)
	}

	// BFS to find shortest path
	type node struct {
		territory *models.Territory
		distance  int
		path      []string
	}

	visited := make(map[string]bool)
	queue := []node{{fromTerritory, 0, []string{from}}}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.territory == toTerritory {
			return current.distance, current.path, nil
		}

		for _, neighbor := range current.territory.ConnectedTo {
			if !visited[neighbor.Name] {
				visited[neighbor.Name] = true
				newPath := make([]string, len(current.path))
				copy(newPath, current.path)
				newPath = append(newPath, neighbor.Name)
				queue = append(queue, node{neighbor, current.distance + 1, newPath})
			}
		}
	}

	return 0, nil, fmt.Errorf("no path from %s to %s", from, to)
}

// CalculateMovementDistanceForPiece calculates the shortest valid path for a specific piece,
// respecting terrain constraints (land pieces stay on land, sea pieces stay at sea, etc.)
// This version does NOT check for enemy units - use CalculateMovementPathForPiece for that.
func CalculateMovementDistanceForPiece(game *models.Game, piece *models.Piece, from, to string) (int, []string, error) {
	fromTerritory, exists := game.Board[from]
	if !exists {
		return 0, nil, fmt.Errorf("territory %s not found", from)
	}

	toTerritory, exists := game.Board[to]
	if !exists {
		return 0, nil, fmt.Errorf("territory %s not found", to)
	}

	// BFS to find shortest path that respects terrain constraints
	type node struct {
		territory *models.Territory
		distance  int
		path      []string
	}

	visited := make(map[string]bool)
	queue := []node{{fromTerritory, 0, []string{from}}}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.territory == toTerritory {
			return current.distance, current.path, nil
		}

		for _, neighbor := range current.territory.ConnectedTo {
			if !visited[neighbor.Name] {
				// Check if the piece can traverse this neighbor territory
				if validateTerrain(piece, neighbor) == nil {
					visited[neighbor.Name] = true
					newPath := make([]string, len(current.path))
					copy(newPath, current.path)
					newPath = append(newPath, neighbor.Name)
					queue = append(queue, node{neighbor, current.distance + 1, newPath})
				}
			}
		}
	}

	return 0, nil, fmt.Errorf("no valid path from %s to %s for %s (terrain: %s)", from, to, piece.Name, piece.Terrain)
}

// CalculateMovementPathForPiece calculates the shortest valid path for a piece, respecting:
// - Terrain constraints (land/sea/air)
// - Enemy-occupied territories (cannot path through them)
// - Move type (combat moves can target enemy territory, noncombat cannot)
func CalculateMovementPathForPiece(game *models.Game, piece *models.Piece, from, to string, currentPlayer *models.Player, moveType MoveType) (int, []string, error) {
	fromTerritory, exists := game.Board[from]
	if !exists {
		return 0, nil, fmt.Errorf("territory %s not found", from)
	}

	toTerritory, exists := game.Board[to]
	if !exists {
		return 0, nil, fmt.Errorf("territory %s not found", to)
	}

	// BFS to find shortest valid path
	type node struct {
		territory *models.Territory
		distance  int
		path      []string
	}

	visited := make(map[string]bool)
	queue := []node{{fromTerritory, 0, []string{from}}}
	visited[from] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.territory == toTerritory {
			return current.distance, current.path, nil
		}

		for _, neighbor := range current.territory.ConnectedTo {
			if !visited[neighbor.Name] {
				// Check terrain compatibility
				if validateTerrain(piece, neighbor) != nil {
					continue
				}

				// Check if we can traverse this territory based on ownership and units
				canTraverse := canTraverseTerritory(game, neighbor, toTerritory, currentPlayer, moveType)
				if !canTraverse {
					continue
				}

				visited[neighbor.Name] = true
				newPath := make([]string, len(current.path))
				copy(newPath, current.path)
				newPath = append(newPath, neighbor.Name)
				queue = append(queue, node{neighbor, current.distance + 1, newPath})
			}
		}
	}

	return 0, nil, fmt.Errorf("no valid path from %s to %s for %s (blocked by enemy units or terrain)", from, to, piece.Name)
}

// canTraverseTerritory checks if a unit can move through a territory
func canTraverseTerritory(game *models.Game, territory, destination *models.Territory, currentPlayer *models.Player, moveType MoveType) bool {
	// If this is the destination territory
	if territory == destination {
		// Combat moves can target enemy territories
		if moveType == CombatMove {
			// Can target enemy territories, but NOT allied territories
			// Rulebook page 14: "At no time can an Allied power attack another Allied power,
			// or an Axis power attack another Axis power"
			if territory.Owner != currentPlayer {
				// Check if target is an ally (same side)
				if areAllies(currentPlayer, territory.Owner) {
					return false // Cannot attack allies!
				}

				// Check neutral territory rules
				if !canAttackNeutral(territory, currentPlayer) {
					return false // Cannot attack this neutral
				}

				return true // Can attack enemies
			}
			return true // Can move to own territory
		}

		// Noncombat moves can target:
		// 1. Territories we own
		if territory.Owner == currentPlayer {
			return true
		}

		// 2. Allied territories (same side)
		if areAllies(currentPlayer, territory.Owner) {
			return true
		}

		// 3. Pro-Allied or pro-Axis neutrals we can activate
		if canActivateNeutral(territory, currentPlayer) {
			return true
		}

		// Cannot move into other territories during noncombat
		return false
	}

	// For waypoint territories (not the destination):
	// Can only traverse if it's friendly AND has no enemy units
	// OR if it's a neutral water territory (which can be freely traversed)
	if territory.Owner != currentPlayer {
		// Allow traversing neutral water territories
		if territory.Owner.Name == "Neutral" && territory.Terrain == models.Water {
			return true
		}
		return false
	}

	// Even if we own it, check if there are enemy units there
	// (in case of a battle we haven't resolved yet)
	pieces := game.GetPiecesInTerritory(territory.Name)
	for _, piece := range pieces {
		// Find the owner of this piece
		for _, player := range game.Players {
			if pieceOwner := findPieceOwner(game, piece, player); pieceOwner != nil {
				if pieceOwner != currentPlayer {
					return false // Enemy unit in our territory - can't traverse
				}
				break
			}
		}
	}

	return true
}

// canAttackNeutral checks if a player can attack a neutral territory
func canAttackNeutral(territory *models.Territory, attacker *models.Player) bool {
	// Not a neutral territory
	if territory.Owner.Name != "Neutral" {
		return true // Normal attack rules apply
	}

	// Strict neutrals cannot be attacked
	if territory.NeutralType == models.StrictNeutral {
		return false
	}

	// Pro-Allied neutrals can be attacked by Axis, but they will defend
	if territory.NeutralType == models.ProAlliedNeutral {
		return attacker.Side == "Axis" // Only Axis can attack pro-Allied neutrals
	}

	// Pro-Axis neutrals can be attacked by Allies
	if territory.NeutralType == models.ProAxisNeutral {
		return attacker.Side == "Allies" // Only Allies can attack pro-Axis neutrals
	}

	return false
}

// canActivateNeutral checks if a player can peacefully activate a neutral territory
// during noncombat move phase
func canActivateNeutral(territory *models.Territory, activator *models.Player) bool {
	// Not a neutral territory
	if territory.Owner.Name != "Neutral" {
		return false
	}

	// Cannot activate strict neutrals
	if territory.NeutralType == models.StrictNeutral {
		return false
	}

	// Pro-Allied neutrals can be activated by Allied powers
	if territory.NeutralType == models.ProAlliedNeutral {
		return activator.Side == "Allies"
	}

	// Pro-Axis neutrals can be activated by Axis powers
	if territory.NeutralType == models.ProAxisNeutral {
		return activator.Side == "Axis"
	}

	return false
}

// findPieceOwner finds which player owns a piece
func findPieceOwner(game *models.Game, piece *models.Piece, player *models.Player) *models.Player {
	// Check if the piece is in any of this player's territories
	for _, territory := range player.Territories {
		for _, pieceID := range territory.Pieces {
			if game.Pieces[pieceID] == piece {
				return player
			}
		}
	}
	return nil
}

// areAllies checks if two players are on the same side (Axis or Allies)
func areAllies(player1, player2 *models.Player) bool {
	// If either player doesn't have a side set, they're not allies
	if player1.Side == "" || player2.Side == "" {
		return false
	}
	return player1.Side == player2.Side
}

// CanReachTerritory checks if a piece can reach a territory with its movement
func CanReachTerritory(game *models.Game, pieceID int, from, to string) (bool, error) {
	piece, exists := game.Pieces[pieceID]
	if !exists {
		return false, fmt.Errorf("piece %d not found", pieceID)
	}

	distance, _, err := CalculateMovementDistanceForPiece(game, piece, from, to)
	if err != nil {
		return false, err
	}

	return distance <= int(piece.Movement), nil
}

// GetReachableTerritories returns all territories a piece can reach
func GetReachableTerritories(game *models.Game, pieceID int, fromTerritory string) ([]*models.Territory, error) {
	piece, exists := game.Pieces[pieceID]
	if !exists {
		return nil, fmt.Errorf("piece %d not found", pieceID)
	}

	from, exists := game.Board[fromTerritory]
	if !exists {
		return nil, fmt.Errorf("territory %s not found", fromTerritory)
	}

	reachable := make([]*models.Territory, 0)
	visited := make(map[string]bool)

	// BFS up to movement range
	type node struct {
		territory *models.Territory
		distance  int
	}

	queue := []node{{from, 0}}
	visited[fromTerritory] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.distance > 0 && current.distance <= int(piece.Movement) {
			reachable = append(reachable, current.territory)
		}

		if current.distance < int(piece.Movement) {
			for _, neighbor := range current.territory.ConnectedTo {
				if !visited[neighbor.Name] {
					// Only traverse if the piece can move through this terrain
					if validateTerrain(piece, neighbor) == nil {
						visited[neighbor.Name] = true
						queue = append(queue, node{neighbor, current.distance + 1})
					}
				}
			}
		}
	}

	return reachable, nil
}

// AddMove records a planned move
func (mt *MovementTracker) AddMove(pieceID int, from, to string, moveType MoveType) error {
	// Check if piece has already moved
	if originalFrom, hasMoved := mt.PiecesMovedFrom[pieceID]; hasMoved {
		return fmt.Errorf("piece %d has already moved from %s", pieceID, originalFrom)
	}

	move := &Move{
		PieceID: pieceID,
		From:    from,
		To:      to,
		Type:    moveType,
	}

	mt.Moves = append(mt.Moves, move)
	mt.PiecesMovedFrom[pieceID] = from

	return nil
}

// RemoveMove cancels a planned move
func (mt *MovementTracker) RemoveMove(pieceID int) error {
	// Find and remove the move
	found := false
	newMoves := make([]*Move, 0, len(mt.Moves))

	for _, move := range mt.Moves {
		if move.PieceID != pieceID {
			newMoves = append(newMoves, move)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("no move found for piece %d", pieceID)
	}

	mt.Moves = newMoves
	delete(mt.PiecesMovedFrom, pieceID)

	return nil
}

// GetMovesByType returns all moves of a specific type
func (mt *MovementTracker) GetMovesByType(moveType MoveType) []*Move {
	moves := make([]*Move, 0)
	for _, move := range mt.Moves {
		if move.Type == moveType {
			moves = append(moves, move)
		}
	}
	return moves
}

// Clear removes all tracked moves
func (mt *MovementTracker) Clear() {
	mt.Moves = make([]*Move, 0)
	mt.PiecesMovedFrom = make(map[int]string)
}

// ExecuteMoves applies all moves to the game state
func (mt *MovementTracker) ExecuteMoves(game *models.Game) error {
	for _, move := range mt.Moves {
		err := game.MovePiece(move.PieceID, move.From, move.To)
		if err != nil {
			return fmt.Errorf("failed to execute move from %s to %s: %v", move.From, move.To, err)
		}
	}
	return nil
}

// ValidateLoad checks if a piece can be loaded onto a transport
func ValidateLoad(game *models.Game, transportID, pieceID int, currentPlayerName string) error {
	// Get the pieces
	transport, exists := game.Pieces[transportID]
	if !exists {
		return fmt.Errorf("transport %d not found", transportID)
	}

	piece, exists := game.Pieces[pieceID]
	if !exists {
		return fmt.Errorf("piece %d not found", pieceID)
	}

	// Find territories where pieces are located
	var transportTerritory, pieceTerritory *models.Territory
	for _, territory := range game.Board {
		for _, id := range territory.Pieces {
			if id == transportID {
				transportTerritory = territory
			}
			if id == pieceID {
				pieceTerritory = territory
			}
		}
	}

	if transportTerritory == nil {
		return fmt.Errorf("transport %d not on board", transportID)
	}
	if pieceTerritory == nil {
		return fmt.Errorf("piece %d not on board", pieceID)
	}

	// Check if piece is already loaded
	if game.IsLoaded(pieceID) {
		return fmt.Errorf("piece %d is already loaded", pieceID)
	}

	// Check ownership - both must be owned by current player
	currentPlayer := game.Players[currentPlayerName]
	if transportTerritory.Owner != currentPlayer {
		return fmt.Errorf("transport is in enemy territory %s", transportTerritory.Name)
	}
	if pieceTerritory.Owner != currentPlayer {
		return fmt.Errorf("piece is in enemy territory %s", pieceTerritory.Name)
	}

	// Check if territories are the same or adjacent
	if transportTerritory != pieceTerritory {
		// Must be adjacent
		if !areConnected(pieceTerritory, transportTerritory) {
			return fmt.Errorf("piece in %s is not adjacent to transport in %s",
				pieceTerritory.Name, transportTerritory.Name)
		}
	}

	// The transport should be in water and piece should be land (for typical A&A rules)
	if transport.Terrain != models.Water {
		return fmt.Errorf("%s is not a sea transport", transport.Name)
	}
	if piece.Terrain != models.Land {
		return fmt.Errorf("only land units can be loaded onto transports")
	}

	return nil
}

// ValidateUnload checks if a piece can be unloaded from a transport
func ValidateUnload(game *models.Game, transportID, pieceID int, destinationName string) error {
	// Check if piece is loaded in this transport
	actualTransportID := game.GetTransportForPiece(pieceID)
	if actualTransportID == -1 {
		return fmt.Errorf("piece %d is not loaded in any transport", pieceID)
	}
	if actualTransportID != transportID {
		return fmt.Errorf("piece %d is not in transport %d", pieceID, transportID)
	}

	// Get destination territory
	destination, exists := game.Board[destinationName]
	if !exists {
		return fmt.Errorf("destination territory %s not found", destinationName)
	}

	// Find transport's current location
	var transportTerritory *models.Territory
	for _, territory := range game.Board {
		for _, id := range territory.Pieces {
			if id == transportID {
				transportTerritory = territory
				break
			}
		}
		if transportTerritory != nil {
			break
		}
	}

	if transportTerritory == nil {
		return fmt.Errorf("transport %d not found on board", transportID)
	}

	// Destination must be the same as transport location or adjacent
	if destination != transportTerritory && !areConnected(transportTerritory, destination) {
		return fmt.Errorf("cannot unload to %s - must be same or adjacent to transport location %s",
			destinationName, transportTerritory.Name)
	}

	// Get the piece to check terrain compatibility
	piece, exists := game.Pieces[pieceID]
	if !exists {
		return fmt.Errorf("piece %d not found", pieceID)
	}

	// Check if piece can move to destination terrain
	if err := validateTerrain(piece, destination); err != nil {
		return err
	}

	return nil
}

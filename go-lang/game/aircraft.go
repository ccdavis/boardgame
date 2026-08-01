package game

import (
	"sort"

	"boardgame/models"
)

// StrandedAircraft is an aircraft with nowhere legal to be when its owner's
// turn ends: parked on ground that is not friendly, or loose in a sea zone
// with no free seat on a friendly carrier.
type StrandedAircraft struct {
	Piece     *models.Piece
	Territory string
}

// StrandedAircraftFor lists the player's aircraft that will have nowhere to
// land when the turn ends. Pieces with a move already planned this phase are
// skipped -- PlanMove has validated their destination -- and planned carrier
// movements count: a carrier booked to arrive brings its free deck space with
// it, one booked to leave takes it away. After moves execute the tracker is
// empty, so the same function then judges the board exactly as it stands.
func (gc *GameController) StrandedAircraftFor(player *models.Player) []StrandedAircraft {
	planned := make(map[int]*Move)
	for _, move := range gc.MoveTracker.Moves {
		planned[move.PieceID] = move
	}

	stranded := make([]StrandedAircraft, 0)
	for _, name := range sortedTerritoryNames(gc.Game) {
		territory := gc.Game.Board[name]

		if territory.Terrain != models.Water {
			// Ground: friendly soil (own or allied) is an airfield, anything
			// else is a crash site.
			if territory.Owner == player || areAllies(territory.Owner, player) {
				continue
			}
			for _, id := range territory.Pieces {
				piece := gc.Game.Pieces[id]
				if piece == nil || piece.Owner != player || piece.Terrain != models.Air {
					continue
				}
				if planned[id] != nil {
					continue // flying out this phase; PlanMove approved it
				}
				stranded = append(stranded, StrandedAircraft{Piece: piece, Territory: name})
			}
			continue
		}

		// Sea: seats come from friendly carriers that will be here -- those
		// present and not booked to sail away, plus those booked to arrive.
		type deck struct {
			carries []string
			free    int
		}
		decks := make([]*deck, 0)
		addDeck := func(ship *models.Piece) {
			if ship == nil || ship.Owner == nil || ship.Capacity == 0 {
				return
			}
			if ship.Owner != player && !areAllies(ship.Owner, player) {
				return
			}
			if free := int(ship.Capacity) - len(ship.Holding); free > 0 {
				decks = append(decks, &deck{carries: ship.CanCarry, free: free})
			}
		}
		for _, id := range territory.Pieces {
			ship := gc.Game.Pieces[id]
			if ship == nil || ship.Terrain != models.Water {
				continue
			}
			if move := planned[id]; move != nil && move.To != name {
				continue // sailing away, deck goes with it
			}
			addDeck(ship)
		}
		for id, move := range planned {
			ship := gc.Game.Pieces[id]
			if ship != nil && ship.Terrain == models.Water && move.To == name && move.From != name {
				addDeck(ship)
			}
		}

		claim := func(unitName string) bool {
			for _, d := range decks {
				if d.free == 0 {
					continue
				}
				for _, kind := range d.carries {
					if kind == unitName {
						d.free--
						return true
					}
				}
			}
			return false
		}

		// Allied aircraft parked here hold their seats first: they were legal
		// when their owner's turn ended, and they are not ours to evict.
		var own []*models.Piece
		for _, id := range territory.Pieces {
			piece := gc.Game.Pieces[id]
			if piece == nil || piece.Terrain != models.Air || piece.Owner == nil {
				continue
			}
			switch {
			case piece.Owner == player:
				if planned[id] == nil {
					own = append(own, piece)
				}
			case areAllies(piece.Owner, player):
				claim(piece.Name)
			}
		}
		for _, piece := range own {
			if !claim(piece.Name) {
				stranded = append(stranded, StrandedAircraft{Piece: piece, Territory: name})
			}
		}
	}
	return stranded
}

// RecoverAircraft plans noncombat moves that bring stranded aircraft home:
// the nearest friendly ground (or carrier seat) their remaining movement can
// reach. Returns how many rescues were planned. Aircraft that cannot reach
// safety are left where they are; the end-of-turn sweep takes them, exactly
// as the rules demand.
func (npc *NPCAIPlayer) RecoverAircraft(controller *GameController, player *models.Player, transcript *GameTranscript) int {
	g := controller.Game
	moved := 0
	for _, s := range controller.StrandedAircraftFor(player) {
		piece := s.Piece
		remaining := controller.MoveTracker.Remaining(piece.ID, int(piece.Movement))
		if remaining <= 0 {
			continue // it spent everything getting here; it is lost
		}

		type option struct {
			name string
			dist int
		}
		options := make([]option, 0)
		for _, name := range sortedTerritoryNames(g) {
			dest := g.Board[name]
			if dest.Terrain == models.Water {
				// A carrier seat counts, but only where one will exist.
				if !controller.CarrierSlotFree(piece, dest, player) {
					continue
				}
			} else if dest.Owner != player && !areAllies(dest.Owner, player) {
				continue
			}
			dist, _, err := CalculateMovementPathForPiece(g, piece, s.Territory, name, player, NoncombatMove)
			if err != nil || dist == 0 || dist > remaining {
				continue
			}
			options = append(options, option{name, dist})
		}
		sort.Slice(options, func(i, j int) bool { return options[i].dist < options[j].dist })

		for _, opt := range options {
			if err := controller.PlanMove(piece.ID, s.Territory, opt.name); err == nil {
				transcript.LogMove(player.Name, piece.Name, s.Territory, opt.name, "noncombat")
				moved++
				break
			}
		}
	}
	return moved
}

// crashStrandedAircraft removes the outgoing player's aircraft that have
// nowhere to land. Called when the turn passes: by then every move has been
// executed, so what is stranded now stays stranded. Rulebook: aircraft that
// cannot reach a legal landing place at the end of the turn are lost.
func (gc *GameController) crashStrandedAircraft(player *models.Player) []StrandedAircraft {
	stranded := gc.StrandedAircraftFor(player)
	for _, s := range stranded {
		gc.removePieceFromBoard(s.Piece, s.Territory)
		// AdvanceTurn has no transcript, so losses are recorded on the
		// controller where a front end (or an observer) can read them.
		gc.CrashLog = append(gc.CrashLog,
			player.Name+" lost a "+s.Piece.Name+" with nowhere to land in "+s.Territory)
	}
	return stranded
}

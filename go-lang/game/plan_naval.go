package game

import (
	"fmt"

	"boardgame/models"
)

// What the fleet does once the troops are ashore.
//
// A landing leaves ships in a foreign sea with nothing to do. Left to the
// general logic they drift, because nothing in it has an opinion about idle
// warships -- so a surviving escort was simply lost to the war effort, and the
// aircraft aboard a carrier with it.
//
// Two answers, and the fleet is split between them rather than all doing one
// thing. Ships that are useful where they are stay and cover the beachhead:
// a carrier's aircraft can support the fighting ashore, and submarines screen
// the carrier. Ships that are not useful there go home, where they can pick up
// another landing force and meanwhile defend their own coast.

// NavalMission is what a surviving squadron has been told to do.
type NavalMission int

const (
	// NavalPatrol: stay off the beachhead and cover it.
	NavalPatrol NavalMission = iota
	// NavalReturn: sail for a friendly port.
	NavalReturn
)

func (m NavalMission) String() string {
	if m == NavalReturn {
		return "returning"
	}
	return "patrolling"
}

// NavalPlan is a squadron with somewhere to be.
type NavalPlan struct {
	ID      int
	Power   string
	Mission NavalMission

	// Codename is the operation's name, drawn from the side's vendored list.
	Codename string

	// Station is where the squadron is headed: the sea zone it covers, or the
	// home port it is making for.
	Station string

	// Supporting is the territory a patrol is covering, if any. When it stops
	// being contested the patrol has no reason to stay.
	Supporting string

	Ships []int

	Done         bool
	CreatedTurn  int
	LastProgress int
}

// Describe renders a naval plan for a transcript.
func (p *NavalPlan) Describe() string {
	name := p.Codename
	if name == "" {
		name = "UNNAMED"
	}
	text := fmt.Sprintf("Operation %s (squadron %d): %d ships %s at %s",
		name, p.ID, len(p.Ships), p.Mission, p.Station)
	if p.Supporting != "" {
		text += " covering " + p.Supporting
	}
	return text
}

// DisposeOfEscorts gives a finished operation's surviving warships something to
// do, splitting them between covering the beachhead and going home.
//
// Called when an amphibious plan completes. Aircraft carriers and their screen
// stay if the landing still needs support; everything else heads for a port,
// where it is available for the next operation and defends the coast meanwhile.
func (npc *NPCAIPlayer) DisposeOfEscorts(gc *GameController, player *models.Player, done *AmphibiousPlan, transcript *GameTranscript) {
	g := gc.Game

	survivors := make([]int, 0, len(done.Escorts)+len(done.Ships))
	for _, id := range append(append([]int{}, done.Escorts...), done.Ships...) {
		if piece, ok := g.Pieces[id]; ok && piece.Owner != nil && piece.Owner.Name == player.Name {
			survivors = append(survivors, id)
		}
	}
	if len(survivors) == 0 {
		return
	}

	// Is the beachhead still worth covering? If the landing failed or the place
	// is already secure, there is nothing to support.
	target := g.Board[done.Target]
	stillContested := target != nil && (target.Owner == nil || target.Owner.Name != player.Name)

	var patrol, home []int
	for _, id := range survivors {
		piece, ok := g.Pieces[id]
		if !ok {
			continue
		}
		if stillContested && usefulOnStation(g, piece) {
			patrol = append(patrol, id)
		} else {
			home = append(home, id)
		}
	}

	// A carrier on station wants a screen. Submarines and other warships are
	// worth more beside it than sailing home empty.
	if len(patrol) > 0 {
		for i := 0; i < len(home); {
			piece := g.Pieces[home[i]]
			if piece != nil && screensCarrier(g, piece) && len(patrol) < len(survivors)/2+1 {
				patrol = append(patrol, home[i])
				home = append(home[:i], home[i+1:]...)
				continue
			}
			i++
		}
	}

	if len(patrol) > 0 {
		plan := gc.Plans.AddNaval(&NavalPlan{
			Power:        player.Name,
			Mission:      NavalPatrol,
			Station:      done.DropZone,
			Supporting:   done.Target,
			Ships:        patrol,
			CreatedTurn:  g.Turn,
			LastProgress: g.Turn,
		})
		transcript.LogSecretAction(player.Name, "new "+plan.Describe())
	}
	if len(home) > 0 {
		port := nearestFriendlyPort(g, player, done.DropZone)
		if port == "" {
			port = done.Embark
		}
		plan := gc.Plans.AddNaval(&NavalPlan{
			Power:        player.Name,
			Mission:      NavalReturn,
			Station:      port,
			Ships:        home,
			CreatedTurn:  g.Turn,
			LastProgress: g.Turn,
		})
		transcript.LogSecretAction(player.Name, "new "+plan.Describe())
	}
}

// usefulOnStation reports whether a ship contributes to a landing it is sitting
// off: a carrier whose aircraft can join the fighting, or a warship that can
// bombard or fight off a counterattack.
func usefulOnStation(g *models.Game, piece *models.Piece) bool {
	caps := g.Units().For(piece)
	if len(piece.Holding) > 0 {
		return true // carrying aircraft that can support the landing
	}
	return caps.CanBombard
}

// screensCarrier reports whether a ship is worth keeping alongside a carrier.
// Submarines are the obvious screen, but anything that fights will do.
func screensCarrier(g *models.Game, piece *models.Piece) bool {
	caps := g.Units().For(piece)
	return caps.IsSubmarine || caps.NegatesSubmarines || piece.Attack > 0
}

// nearestFriendlyPort finds the closest sea zone next to territory this power
// holds, which is where a squadron with nothing to do should be.
func nearestFriendlyPort(g *models.Game, player *models.Player, from string) string {
	start, ok := g.Board[from]
	if !ok {
		return ""
	}

	seen := map[string]bool{from: true}
	queue := []*models.Territory{start}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// A sea zone beside our own coast: close enough to load again, and it
		// covers home waters meanwhile.
		for _, neighbour := range current.ConnectedTo {
			if neighbour.Terrain == models.Land &&
				neighbour.Owner == player && current.Terrain == models.Water {
				return current.Name
			}
		}
		for _, neighbour := range current.ConnectedTo {
			if neighbour.Terrain == models.Water && !seen[neighbour.Name] {
				seen[neighbour.Name] = true
				queue = append(queue, neighbour)
			}
		}
	}
	return ""
}

// ReviewNaval keeps squadrons current: it drops losses, retires plans that have
// arrived or lost their reason, and reports what is still under way.
func (npc *NPCAIPlayer) ReviewNaval(gc *GameController, player *models.Player, transcript *GameTranscript) {
	if gc.Plans == nil {
		return
	}
	g := gc.Game

	kept := make([]*NavalPlan, 0, len(gc.Plans.naval[player.Name]))
	for _, plan := range gc.Plans.naval[player.Name] {
		plan.Ships = survivors(g, player.Name, plan.Ships)
		if len(plan.Ships) == 0 {
			continue // squadron sunk; nothing left to command
		}

		switch plan.Mission {
		case NavalPatrol:
			// A patrol exists to cover a landing. Once the place is ours the
			// squadron is free, and goes home to be useful somewhere else.
			if target := g.Board[plan.Supporting]; target != nil &&
				target.Owner != nil && target.Owner.Name == player.Name {
				plan.Mission = NavalReturn
				plan.Station = nearestFriendlyPort(g, player, plan.Station)
				plan.LastProgress = g.Turn
				transcript.LogSecretAction(player.Name, plan.Describe())
			}
		case NavalReturn:
			// Arrived: the ships rejoin the general pool, where they can be
			// taken up by the next operation.
			if allAt(g, plan.Ships, plan.Station) {
				transcript.LogSecretAction(player.Name, fmt.Sprintf(
					"squadron %d reached %s and is available again", plan.ID, plan.Station))
				continue
			}
		}
		kept = append(kept, plan)
	}
	gc.Plans.naval[player.Name] = kept
}

func allAt(g *models.Game, ids []int, place string) bool {
	territory := g.Board[place]
	if territory == nil {
		return true // nowhere to be; treat as arrived rather than sailing forever
	}
	for _, id := range ids {
		if !contains(territory.Pieces, id) {
			return false
		}
	}
	return true
}

// SailNavalPlans moves squadrons toward their station during noncombat movement.
func (npc *NPCAIPlayer) SailNavalPlans(gc *GameController, player *models.Player, transcript *GameTranscript) int {
	if gc.Plans == nil {
		return 0
	}
	g := gc.Game
	moved := 0

	for _, plan := range gc.Plans.naval[player.Name] {
		for _, id := range plan.Ships {
			piece, ok := g.Pieces[id]
			if !ok {
				continue
			}
			from := territoryOf(g, id)
			if from == nil || from.Name == plan.Station {
				continue
			}
			step := nextStepTowards(g, piece, from.Name, plan.Station, player, models.Water)
			if step == "" {
				continue
			}
			if err := gc.PlanMove(id, from.Name, step); err == nil {
				transcript.LogMove(player.Name, piece.Name, from.Name, step, "noncombat")
				moved++
				plan.LastProgress = g.Turn
			}
		}
	}
	return moved
}

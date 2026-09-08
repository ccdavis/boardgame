package webserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"boardgame/models"
)

// simClient drives the web API exactly the way the browser does, so what it
// sees is what the player sees: the territory list that decides which
// territories glow, the detail view that fills the unit picker, and
// get-reachable that lights the destinations.
type simClient struct {
	t         *testing.T
	server    *Server
	sessionID string
}

func (c *simClient) do(method, path string, payload any) (int, map[string]any) {
	c.t.Helper()
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	} else {
		body = []byte("{}")
	}
	req := httptest.NewRequest(method, "/api/game/"+c.sessionID+path, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	c.server.handleGameRoutes(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (c *simClient) post(action string, payload any) (int, map[string]any) {
	return c.do(http.MethodPost, "/action/"+action, payload)
}

func (c *simClient) state() GameStateDTO {
	req := httptest.NewRequest(http.MethodGet, "/api/game/"+c.sessionID, nil)
	rec := httptest.NewRecorder()
	c.server.handleGameRoutes(rec, req)
	var s GameStateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		c.t.Fatalf("state: %v (%s)", err, rec.Body.String())
	}
	return s
}

func (c *simClient) territories() []TerritoryDTO {
	req := httptest.NewRequest(http.MethodGet, "/api/game/"+c.sessionID+"/territories", nil)
	rec := httptest.NewRecorder()
	c.server.handleGameRoutes(rec, req)
	var out struct {
		Territories []TerritoryDTO `json:"territories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		c.t.Fatalf("territories: %v", err)
	}
	return out.Territories
}

func (c *simClient) details(name string) TerritoryDetailDTO {
	req := httptest.NewRequest(http.MethodGet,
		"/api/game/"+c.sessionID+"/territory/"+url.PathEscape(name), nil)
	rec := httptest.NewRecorder()
	c.server.handleGameRoutes(rec, req)
	var d TerritoryDetailDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		c.t.Fatalf("details %s: %v (%s)", name, err, rec.Body.String())
	}
	return d
}

func (c *simClient) reachable(ids []int, from string) ([]ReachableTerritoryDTO, []string) {
	code, out := c.post("get-reachable", map[string]any{"pieceIds": ids, "fromTerritory": from})
	if code != http.StatusOK {
		c.t.Fatalf("get-reachable %v from %s: %d %v", ids, from, code, out)
	}
	raw, _ := json.Marshal(out["reachable"])
	var dests []ReachableTerritoryDTO
	_ = json.Unmarshal(raw, &dests)
	var notes []string
	rawNotes, _ := json.Marshal(out["boardNotes"])
	_ = json.Unmarshal(rawNotes, &notes)
	return dests, notes
}

func (c *simClient) advance() map[string]any {
	code, out := c.post("advance-phase", nil)
	if code != http.StatusOK && code != http.StatusConflict {
		c.t.Fatalf("advance-phase: %d %v", code, out)
	}
	return out
}

// simReport collects everything odd the simulated player ran into.
type simReport struct {
	bugs  []string
	warts []string
}

func (r *simReport) bug(format string, args ...any) {
	r.bugs = append(r.bugs, fmt.Sprintf(format, args...))
}
func (r *simReport) wart(format string, args ...any) {
	r.warts = append(r.warts, fmt.Sprintf(format, args...))
}

// checkMovementPhase audits what the browser would show at the start of a
// movement phase: every territory that glows must open a picker with something
// in it, and every unit the picker offers must have somewhere to go if the
// board plainly allows it.
func checkMovementPhase(c *simClient, session *GameSession, report *simReport, freshTurn bool, label string) {
	g := session.Controller.Game
	human := g.Players[session.HumanPlayer]
	for _, t := range c.territories() {
		// Audit every territory holding the player's pieces, glowing or not:
		// a territory that fails to glow is exactly the stranding complaint.
		holdsMine := false
		for _, id := range g.Board[t.Name].Pieces {
			if p := g.Pieces[id]; p != nil && p.Owner == human {
				holdsMine = true
			}
		}
		if !holdsMine {
			continue
		}
		d := c.details(t.Name)
		movable := 0
		for _, u := range d.Units {
			if u.Owner != session.HumanPlayer || u.Aboard != 0 {
				continue
			}
			if u.Movement == 0 {
				continue
			}
			if !u.CanMove {
				spent := session.Controller.MoveTracker.MovementSpent[u.ID]
				if freshTurn && u.WhyNot != "moves in noncombat only" {
					report.bug("%s: %s %d in %s cannot move at the start of the turn (spent=%d)",
						label, u.Name, u.ID, t.Name, spent)
				}
				continue
			}
			movable++
			if movable > 3 {
				continue // three units per territory keep the audit quick
			}

			// The browser's offer versus the engine's verdict: every
			// territory the engine would accept a move to must be lit, and
			// nothing lit may be refused. Trial moves are cancelled again.
			offered := map[string]ReachableTerritoryDTO{}
			dests, _ := c.reachable([]int{u.ID}, t.Name)
			for _, d := range dests {
				if !d.IsBoard && !d.IsUnload {
					offered[d.Name] = d
				}
			}
			piece := g.Pieces[u.ID]
			for _, name := range sortedNames(g) {
				if name == t.Name {
					continue
				}
				dest := g.Board[name]
				err := session.Controller.PlanMove(u.ID, t.Name, name)
				if err == nil {
					_ = session.Controller.CancelMove(u.ID)
				}
				_, lit := offered[name]
				switch {
				case err == nil && !lit:
					// The browser deliberately withholds open water from
					// aircraft in the combat phase unless there is a deck
					// or a fleet to fight -- a plane parked there is lost.
					if piece.Terrain == models.Air && dest.Terrain == models.Water {
						continue
					}
					report.bug("%s: %s %d in %s may move to %s (engine accepts) but the map does not offer it",
						label, u.Name, u.ID, t.Name, name)
				case err != nil && lit:
					report.bug("%s: %s %d in %s is offered %s but the engine refuses: %v",
						label, u.Name, u.ID, t.Name, name, err)
				}
			}
			if len(offered) == 0 {
				// Nowhere at all. Plausible only when every neighbour is
				// hostile ground it cannot take or water it cannot enter.
				terr := g.Board[t.Name]
				for _, n := range terr.ConnectedTo {
					friendly := n.Owner == human || (human.Side != "" && n.Owner.Side == human.Side)
					if piece.Terrain == models.Land && n.Terrain == models.Land && friendly {
						report.bug("%s: %s %d in %s has no destination though %s next door is friendly",
							label, u.Name, u.ID, t.Name, n.Name)
					}
					if piece.Terrain == models.Water && n.Terrain == models.Water && !hostileForces(g, n, human) {
						report.bug("%s: %s %d in %s has no destination though %s next door is open water",
							label, u.Name, u.ID, t.Name, n.Name)
					}
				}
			}
		}
		// The glow must tell the truth: lit when the picker would offer
		// something (a free unit, or cargo to put ashore), dark otherwise.
		offerable := movable
		for _, u := range d.Units {
			if u.Owner == session.HumanPlayer && u.Aboard != 0 && u.CanMove {
				offerable++
			}
		}
		names := []string{}
		for _, u := range d.Units {
			if u.Owner == session.HumanPlayer {
				names = append(names, fmt.Sprintf("%s(m%d,can=%v,aboard=%d)", u.Name, u.Movement, u.CanMove, u.Aboard))
			}
		}
		switch {
		case t.FriendlyUnits > 0 && offerable == 0:
			report.bug("%s: %s glows but the picker would be empty: %v", label, t.Name, names)
		case t.FriendlyUnits == 0 && offerable > 0:
			report.bug("%s: %s does not glow though units there can move: %v", label, t.Name, names)
		}
	}
}

func validateState(session *GameSession, report *simReport, label string) {
	for _, p := range session.Controller.Game.Validate() {
		report.bug("%s: invalid state: %s", label, p)
	}
}

// playHumanTurn plays one full turn as the browser would, with a seeded
// random policy that attacks, repositions, boards transports and unloads.
func playHumanTurn(c *simClient, session *GameSession, rng *rand.Rand, report *simReport, round int) {
	me := session.HumanPlayer
	label := func(phase string) string { return fmt.Sprintf("%s T%d %s", me, round, phase) }

	// Purchase: a handful of infantry, sometimes armor or a fighter.
	st := c.state()
	if st.CurrentPhase != "Purchase Units" {
		c.t.Fatalf("expected purchase phase, got %s", st.CurrentPhase)
	}
	for i := 0; i < 6; i++ {
		unit := "infantry"
		switch rng.Intn(6) {
		case 0:
			unit = "armor"
		case 1:
			unit = "fighter"
		case 2:
			unit = "transport"
		}
		c.post("purchase", map[string]any{"unitType": unit, "quantity": 1})
	}
	c.advance()
	validateState(session, report, label("after purchase"))

	// Combat move.
	st = c.state()
	if st.CurrentPhase != "Combat Move" {
		c.t.Fatalf("expected combat move, got %s", st.CurrentPhase)
	}
	checkMovementPhase(c, session, report, true, label("combat move start"))
	moveRandomly(c, session, rng, report, true, label("combat move"))
	out := c.advance()
	if out["blocked"] == true {
		c.t.Fatalf("combat move blocked: %v", out)
	}
	validateState(session, report, label("after combat move"))

	// Conduct combat.
	st = c.state()
	if st.CurrentPhase != "Conduct Combat" {
		c.t.Fatalf("expected conduct combat, got %s", st.CurrentPhase)
	}
	// Fight some battles round by round the way the browser would --
	// choosing losses, sometimes retreating or submerging -- and let the
	// engine settle the rest.
	fightSomeLive(c, session, rng, report, label("combat"))
	code, res := c.post("auto-resolve-battles", nil)
	if code != http.StatusOK {
		report.bug("%s: auto-resolve failed: %v", label("combat"), res)
	}
	out = c.advance()
	if out["blocked"] == true {
		report.bug("%s: still blocked after resolving all battles: %v", label("combat"), out)
		c.t.FailNow()
	}
	validateState(session, report, label("after combat"))

	// Noncombat move.
	st = c.state()
	if st.CurrentPhase != "Noncombat Move" {
		c.t.Fatalf("expected noncombat move, got %s", st.CurrentPhase)
	}
	checkMovementPhase(c, session, report, false, label("noncombat start"))
	moveRandomly(c, session, rng, report, false, label("noncombat"))
	c.advance()
	validateState(session, report, label("after noncombat"))

	// Mobilize: place everything at the first legal target.
	st = c.state()
	if st.CurrentPhase != "Mobilize New Units" {
		c.t.Fatalf("expected mobilize, got %s", st.CurrentPhase)
	}
	for tries := 0; tries < 20; tries++ {
		_, acts := c.do(http.MethodGet, "/available-actions", nil)
		raw, _ := json.Marshal(acts["actions"].(map[string]any)["purchasedUnits"])
		var groups []PurchasedUnitDTO
		_ = json.Unmarshal(raw, &groups)
		if len(groups) == 0 {
			break
		}
		rawCap, _ := json.Marshal(acts["actions"].(map[string]any)["factoryCapacity"])
		capacity := map[string]int{}
		_ = json.Unmarshal(rawCap, &capacity)
		rawYard, _ := json.Marshal(acts["actions"].(map[string]any)["yardFor"])
		yardFor := map[string]string{}
		_ = json.Unmarshal(rawYard, &yardFor)
		placed := false
		for _, gr := range groups {
			if len(gr.Targets) == 0 {
				continue
			}
			target := gr.Targets[rng.Intn(len(gr.Targets))]
			key := target
			if yard, ok := yardFor[target]; ok {
				key = yard
			}
			if left, known := capacity[key]; known && left <= 0 {
				continue // this factory is done for the turn; the offer list says so
			}
			capacity[key]--
			code, res := c.post("mobilize", map[string]any{"unitType": gr.Type, "territory": target, "quantity": 1})
			if code != http.StatusOK {
				report.bug("%s: mobilize %s at %s (offered as a target) refused: %v", label("mobilize"), gr.Type, target, res)
			} else {
				placed = true
			}
		}
		if !placed {
			// Units with nowhere to go (e.g. ships with no coastal factory)
			// block the phase for ever. That is a design gap, not this test's
			// subject; drop them so the game can go on.
			g := session.Controller.Game
			kept := g.PurchasedUnits[me][:0]
			for _, u := range g.PurchasedUnits[me] {
				if len(session.Controller.PlacementTargets(g.Players[me], u.Type)) > 0 {
					kept = append(kept, u)
				}
			}
			if len(kept) == len(g.PurchasedUnits[me]) {
				break
			}
			g.PurchasedUnits[me] = kept
		}
	}
	out = c.advance()
	if out["blocked"] == true {
		report.bug("%s: mobilize blocked: %v", label("mobilize"), out)
		session.Controller.Game.PurchasedUnits[me] = nil
		c.advance()
	}
	validateState(session, report, label("after mobilize"))

	// Collect income.
	c.advance()
	validateState(session, report, label("after income"))
}

// moveRandomly is the human's movement policy: visit every glowing
// territory, pick some units, and send them to a random legal destination --
// attacks in the combat phase, boarding and unloading whenever offered.
func moveRandomly(c *simClient, session *GameSession, rng *rand.Rand, report *simReport, combat bool, label string) {
	me := session.HumanPlayer
	terrs := c.territories()
	rng.Shuffle(len(terrs), func(i, j int) { terrs[i], terrs[j] = terrs[j], terrs[i] })

	for _, t := range terrs {
		if t.FriendlyUnits == 0 || rng.Intn(3) == 0 {
			continue
		}
		d := c.details(t.Name)

		// Cargo first: loaded ships offer "put ashore".
		cargoByShip := map[int][]int{}
		var free []UnitDTO
		for _, u := range d.Units {
			if u.Owner != me {
				continue
			}
			if u.Aboard != 0 {
				if u.CanMove {
					cargoByShip[u.Aboard] = append(cargoByShip[u.Aboard], u.ID)
				}
				continue
			}
			if u.CanMove && u.Movement > 0 {
				free = append(free, u)
			}
		}
		for _, cargo := range cargoByShip {
			if rng.Intn(2) == 0 {
				continue
			}
			dests, _ := c.reachable(cargo, t.Name)
			if len(dests) == 0 {
				continue
			}
			dest := dests[rng.Intn(len(dests))]
			code, res := c.post("unload-transport", map[string]any{"pieceIds": cargo, "territory": dest.Name})
			if code != http.StatusOK {
				report.bug("%s: unload %v onto %s (offered) refused: %v", label, cargo, dest.Name, res)
			}
		}

		if len(free) == 0 {
			continue
		}
		// Pick a random subset, grouped so slow units do not always drag.
		rng.Shuffle(len(free), func(i, j int) { free[i], free[j] = free[j], free[i] })
		n := 1 + rng.Intn(len(free))
		picked := free[:n]
		ids := make([]int, len(picked))
		for i, u := range picked {
			ids[i] = u.ID
		}
		dests, notes := c.reachable(ids, t.Name)
		if len(dests) == 0 {
			_ = notes
			continue
		}
		// Prefer attacks that look winnable; otherwise a random destination.
		var choice *ReachableTerritoryDTO
		if combat {
			for i := range dests {
				if dests[i].IsAttack && dests[i].UnitCount < len(ids) && rng.Intn(2) == 0 {
					choice = &dests[i]
					break
				}
			}
		}
		if choice == nil {
			choice = &dests[rng.Intn(len(dests))]
			if choice.IsAttack && choice.UnitCount > len(ids)*2 {
				continue // suicide is not the policy
			}
		}
		switch {
		case choice.IsBoard:
			code, res := c.post("load-transports", map[string]any{"pieceIds": ids, "seaZone": choice.Name})
			if code != http.StatusOK {
				report.bug("%s: boarding %v in %s (offered) refused: %v", label, ids, choice.Name, res)
			}
		case choice.IsUnload:
			code, res := c.post("unload-transport", map[string]any{"pieceIds": ids, "territory": choice.Name})
			if code != http.StatusOK {
				report.bug("%s: unload %v onto %s (offered) refused: %v", label, ids, choice.Name, res)
			}
		default:
			for _, id := range ids {
				code, res := c.post("plan-move", map[string]any{"pieceId": id, "from": t.Name, "to": choice.Name})
				if code != http.StatusOK {
					report.bug("%s: plan-move %s %d %s -> %s (offered by get-reachable) refused: %v",
						label, session.Controller.Game.Pieces[id].Name, id, t.Name, choice.Name, res["error"])
				}
			}
		}
	}
}

// TestSim_HumanPlaysEveryPowerWithoutStrandingUnits plays several rounds as
// each power through the web API and audits, at the start of every movement
// phase, that no unit the player owns is left unselectable or immovable for
// no reason the board can show.
func TestSim_HumanPlaysEveryPowerWithoutStrandingUnits(t *testing.T) {
	rounds := 5
	if v := os.Getenv("SIM_ROUNDS"); v != "" {
		rounds, _ = strconv.Atoi(v)
	}
	seed := int64(1)
	if v := os.Getenv("SIM_SEED"); v != "" {
		seed, _ = strconv.ParseInt(v, 10, 64)
	}
	powers := []string{"Germany", "USA", "USSR", "UK", "Japan", "Italy"}
	if v := os.Getenv("SIM_POWER"); v != "" {
		powers = []string{v}
	}

	for _, power := range powers {
		t.Run(power, func(t *testing.T) {
			server := NewServer(0)
			sessionID := newGameForTestAs(t, server, power)
			session, err := server.sessionManager.GetSession(sessionID)
			if err != nil {
				t.Fatal(err)
			}
			c := &simClient{t: t, server: server, sessionID: sessionID}
			rng := rand.New(rand.NewSource(seed))
			report := &simReport{}

			for round := 1; round <= rounds; round++ {
				// NPCs play until it is the human's turn.
				for i := 0; i < 12; i++ {
					st := c.state()
					if st.GameOver {
						break
					}
					if st.CurrentPower == power {
						break
					}
					code, res := c.post("execute-npc-turn", nil)
					if code != http.StatusOK {
						t.Fatalf("NPC turn for %s failed: %v", st.CurrentPower, res)
					}
					validateState(session, report, fmt.Sprintf("after %s's turn", st.CurrentPower))
				}
				st := c.state()
				if st.GameOver {
					t.Logf("game over in round %d: %s wins", round, st.Winner)
					break
				}
				if st.CurrentPower != power {
					t.Fatalf("play never reached %s (stuck at %s)", power, st.CurrentPower)
				}
				playHumanTurn(c, session, rng, report, round)
			}

			sort.Strings(report.warts)
			for _, w := range report.warts {
				t.Logf("wart: %s", w)
			}
			for _, b := range report.bugs {
				t.Errorf("BUG: %s", b)
			}
		})
	}
}

// fightSomeLive plays pending battles through the round-by-round API with a
// random policy, checking that every answer is consistent with the rules the
// screen shows: hits owed are exactly assignable, retreat and submerge are
// offered only when legal and work when taken, and a finished battle leaves
// nothing pending and a valid board.
func fightSomeLive(c *simClient, session *GameSession, rng *rand.Rand, report *simReport, label string) {
	for _, name := range session.Controller.BattleOrder() {
		if rng.Intn(3) == 0 {
			continue // leave some for auto-resolve
		}
		code, res := c.post("battle-begin", map[string]any{"territory": name})
		if code != http.StatusOK {
			// A landing whose covering sea battle is still pending is
			// refused by rule; anything else is a fault.
			if !strings.Contains(fmt.Sprint(res["error"]), "sea battle") {
				report.bug("%s: battle-begin %s: %v", label, name, res)
			}
			continue
		}
		for step := 0; step < 60; step++ {
			raw, _ := json.Marshal(res["battle"])
			var live LiveBattleDTO
			if err := json.Unmarshal(raw, &live); err != nil {
				report.bug("%s: bad live battle payload for %s: %v", label, name, err)
				break
			}
			if live.Done {
				if _, still := session.Controller.PendingBattles[name]; still {
					report.bug("%s: %s reported done but is still pending", label, name)
				}
				break
			}
			switch {
			case live.PendingHits > 0:
				// Spread the hits over random units, respecting capacity.
				owed := live.PendingHits
				picks := map[int]int{}
				units := append([]LiveUnitDTO{}, live.Units...)
				rng.Shuffle(len(units), func(i, j int) { units[i], units[j] = units[j], units[i] })
				for _, u := range units {
					if owed == 0 {
						break
					}
					room := u.MaxHits - u.Hits
					if room <= 0 {
						continue
					}
					take := 1 + rng.Intn(room)
					if take > owed {
						take = owed
					}
					picks[u.ID] = take
					owed -= take
				}
				if owed > 0 {
					report.bug("%s: %s owes %d hits but the units offered cannot absorb them", label, name, live.PendingHits)
					break
				}
				var casualties []map[string]int
				for id, n := range picks {
					casualties = append(casualties, map[string]int{"pieceId": id, "hits": n})
				}
				code, res = c.post("battle-casualties", map[string]any{"territory": name, "casualties": casualties})
			case live.CanRetreat && live.Round >= 1 && rng.Intn(4) == 0:
				code, res = c.post("battle-retreat", map[string]any{"territory": name})
			case live.CanSubmerge && rng.Intn(4) == 0:
				code, res = c.post("battle-submerge", map[string]any{"territory": name})
			case rng.Intn(6) == 0:
				code, res = c.post("resolve-battle", map[string]any{"territory": name})
				if code != http.StatusOK {
					report.bug("%s: resolve-battle mid-fight in %s: %v", label, name, res)
				}
				if _, still := session.Controller.PendingBattles[name]; still {
					report.bug("%s: resolve-battle left %s pending", label, name)
				}
				code = -1
			default:
				code, res = c.post("battle-round", map[string]any{"territory": name})
			}
			if code == -1 {
				break
			}
			if code != http.StatusOK {
				report.bug("%s: live battle step in %s failed: %v", label, name, res)
				break
			}
			if errText, has := res["error"]; has && errText != nil && errText != "" {
				report.bug("%s: live battle step in %s refused: %v", label, name, errText)
				break
			}
		}
		validateState(session, report, label+" after live battle in "+name)
	}
}

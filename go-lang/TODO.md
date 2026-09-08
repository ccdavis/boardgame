# TODO — unresolved issues

Known gaps and deliberate simplifications, in rough priority order; none
block play. Items fixed along the way are recorded at the bottom so the
history of what changed (and why) stays discoverable.

## Rules fidelity

- **Strategic bombing: the complex fires back on its own.** `RollICAADefense`
  gives every raided complex a one-in-six shot per bomber whether or not an
  AA gun stands in the territory. Classic requires the gun. The computer
  players buy guns for their factories anyway, so in practice the difference
  is a human who skipped one.

- **Casualty choice and retreat are the attacker's only.** The defender's
  losses are always the engine's (cheapest first). A human who is attacked
  during a computer turn never chooses; that would mean pausing the NPC's
  turn for a decision, which the web loop does not do.

- **A retreat withdraws everyone who can.** The rules let the attacker pull
  back some units and leave others; here the retreat is all-or-nothing
  among units with a line of retreat (amphibious troops stay regardless).

## Balance (measured by cmd/observe, seeds 2000–2019, 40-round cap)

- **Roughly even now, with a slight Axis edge: Axis 3, Allies 0, 17
  undecided** (was Axis 33–3 with 14 draws over 50 games). Undecided games
  sit between 9–5 and 6–8 on victory cities. The swing came from the air
  arm reaching beyond the next territory, bombing raids, joint operations,
  and liberation; the Axis edge came back once garrisons claimed their
  units before invasions did. The long tail of draws is the next thing to
  look at: both sides hold their cities and neither breaks through in forty
  rounds. Re-measure after any AI change with
  `go run ./cmd/observe -games 20 -seed 2000 -turns 40`.

- **Iran ping-pongs** (contested 72 times in 20 games) between Italy and the
  UK, as Byelorussia does between Germany and the USSR. Cosmetic; the front
  line IS there.

- **NPC purchases are capped at factory output but ignore the queue.** A
  unit left unplaced (a ship with no yard free) is bought again next turn;
  the backlog stays small (one or two units in a few games) but is not zero.

## Battle screen

- **Rounds are shown as hit counts, not dice.** The log says "2 hits (armor,
  infantry)" per side per round; the individual rolls are in the payload
  (`Hit.Roll`) and could be drawn as dice.

## Fog of war

- **Leaked garrison and squadron orders are read, not acted on.** Amphibious
  leaks feed the defender's garrison sizing (`RevealedThreatsAgainst`); a
  leaked defence or naval plan only becomes readable in the transcript. The
  attack picker could weigh a garrison it knows is being built up.

- **Only a human's BOOKED landings reach the shared plan book.** Transports
  being loaded and sailed toward a target are not a claim until the landing
  is booked, so an NPC ally may still plan against the same island a turn
  or two earlier.

## UX

- **Bombing raids are drawn like attacks.** A booked raid shows as a red
  combat arrow and a battle marker; only the review list and the battle
  screen say "bombing raid".

- **"Place N As Bought" places only earmarked groups.** Units bought with no
  factory open (none, in the browser; possible through the API) still need
  placing by hand.

## Fixed (kept for the record)

- ~~No per-round battle decisions~~ — battles can be fought a round at a
  time (`game/battle_live.go`): the attacker chooses casualties, may retreat
  (amphibious troops excepted), and may submerge submarines; "Fight it out"
  and "Resolve All" finish any battle from where it stands.

- ~~NPC air power short-ranged, no bombing~~ — aircraft join any battle they
  can reach and still land after (`airInRange`, `canLandAfter`); idle bombers
  raid enemy factories (`PlanBombingRaids`). Raids are an engine feature
  (`game/bombing.go`) the browser offers too. Powers repair bomb damage
  before buying.

- ~~No coordinated Allied operations~~ — a fortress beyond one power's lift
  is planned as a joint operation: the ally's planner joins it, each half is
  sized to its share, the halves wait for each other at their drop zones and
  land in the same round, and troops afloat beside a beach an ally took land
  as reinforcements.

- ~~Friendly unload allowed in the combat phase~~ — friendly shores unload in
  noncombat only; the combat phase says so when nothing else is lit.

- ~~AAA cannot move~~ — the board gives it a move of 1, noncombat only.

- ~~Factory output uncapped~~ — a complex builds at most its production value
  (less bomb damage) a turn, ships counting against the yard beside them; the
  placement dialog shows what is left, and NPC purchases respect the cap.

- ~~Purchase earmarks per unit type~~ — earmarks are per unit, server-side;
  buying at two factories places each stack where it was bought.

- ~~Industrial-complex repair not exposed~~ — the production menu offers to
  repair bomb damage at one IPC a point.

- ~~Units that fought are silently absent from the picker~~ — every unit is
  listed, greyed with its reason ("already fought this turn", "moves in
  noncombat only", "booked for a landing").

- ~~Empty destination sets unexplained~~ — the server explains an empty
  intersection by unit type, and a dark neighbour by its rule (strict
  neutral and its toll, enemy units, enemy ground).

- ~~Defence and naval plans never leak; the TUI printed nothing~~ — garrison
  and squadron orders run the same 5% leak; the terminal shows each NPC turn
  through the same redaction as the browser.

- ~~Human operations invisible to NPC allies~~ — a booked landing claims its
  target in the shared book until it executes.

- ~~Liberated territory stayed with the liberator~~ — `CaptureTerritory` had
  no notion of an original owner, so an NPC ally that retook a human player's
  province kept it and its income for the rest of the war. Territories now
  remember their `OriginalOwner` (the printed board, or a neutral's first
  conqueror); an ally retaking one hands it back if the owner's capital is
  free, keeps it in trust otherwise, and freeing the capital returns every
  province held that way. The liberator's own troops stay its own; only what
  the enemy left behind (AA guns, factories) changes hands.

- ~~Units beside allied ground had nowhere to go in the combat phase~~ — the
  pathfinder refused any allied-owned destination for a combat move, and since
  every sea zone carries a nominal owner that also barred fleets from attacking
  into zones "owned" by an ally (most of the ocean, for the UK and USA). An
  American infantry in Alaska, whose only road runs into Canada, had no
  destination at all; the territory glowed and the picker said "Nowhere to
  go". Allied ground is now as open as our own in both phases; a combat move
  ending there never stages a battle or changes the owner
  (`canTraverseTerritory`, `ExecuteCombatMoves`).

- ~~Territories glowed with nothing movable in them~~ — the map lit any
  territory holding the player's non-structure pieces, so an AA gun (0
  movement) or a stack that had spent its allowance attacking invited a click
  that opened an empty picker. `movableUnits` now counts only what the picker
  would offer, tracker-aware.

- ~~A slow poll could rewind the browser to the previous phase~~ — a state
  request that resolved after a later action-driven refresh applied its stale
  snapshot, and the apparent phase change cancelled whatever the player was in
  the middle of. Stale responses are dropped (`stateSeq`).

- ~~Sea battle in the drop zone does not gate the landing~~ — battles now
  resolve sea-first (`BattleOrder`), a landing may not be fought while its
  covering sea battle is pending, and troops whose drop zone stayed in enemy
  hands drown (`drownCutOffAttackers`). NPC amphibious attackers also lost
  their impossible teleport-retreat to the staging port.

- ~~Strict-neutral tolls gated per attack, not per treasury~~ — every
  violation booked in a phase must now be payable together
  (`checkNeutralTollFunds`), on both the overland and landing paths.

- ~~Walking into an EMPTY hostile territory stages a "battle"~~ — walk-ins
  capture at move execution (land units only; air and ships take nothing)
  and never enter the battle phase. Removed ~28 fake battles per game.

- ~~Victory thresholds unfair to the Allies on the 14-city board~~ — now
  symmetric at 9/9 (start is 7–7).

- ~~Axis re-plans hopeless US invasions forever~~ — plans watch the target
  garrison for two turns, size against its PROJECTED strength at landing
  time, and a target judged hopeless cools off for six rounds before it may
  be proposed again (never banned: the grand invasion stays on the table).

- ~~The Caucasus revolving door~~ (mitigated) — victory-city attacks now
  commit a real fraction of the stack instead of the cautious minimum;
  ownership changes fell from ~5.5 to ~2.4 per game.

- ~~The UK never plans landings in Europe~~ — `reachableOverland` walked
  through enemy land, so any continental target "had a land path" and was
  never an amphibious problem. It now marches only across friendly soil.
  This is what gave the Allies their first wins.

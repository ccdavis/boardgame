# TODO — unresolved issues

Known gaps and deliberate simplifications, in rough priority order; none
block play. Items fixed along the way are recorded at the bottom so the
history of what changed (and why) stays discoverable.

## Rules fidelity

- **Friendly unload is allowed during the combat-move phase.** The engine is
  deliberately lenient (`UnloadUnit` permits either movement phase); strictly,
  unloading onto friendly ground is noncombat movement only. Tightening this
  would also want UI copy explaining why the shore stopped glowing.

- **AAA cannot move at all** because `aaa.gdf` declares `0 movement`. Classic
  rules give AAA a move of 1 in noncombat. Board-data decision to revisit —
  the board is the authority, so fix it in the `.gdf` if it should change.

- **NPC air power is short-ranged.** Fighters and bombers only join attacks
  on territories ADJACENT to where they sit; a fighter never flies two zones
  to a battle it could reach, and the NPC never flies strategic bombing
  missions at all. Deep strikes need landing-spot planning (attack there,
  land here) that the attack picker does not do yet.

## Balance (measured by cmd/observe, seeds 2000–2049)

- **The Axis still wins most games: 33–3 with 14 draws** after the round of
  fixes below (was 43–0 with 7 draws). The Allies now take Pacific victory
  cities (the Philippines fell 23 times in 50 games) but have never cracked
  Fortress Europe — Western/Southern Europe garrisons exceed what any single
  Allied power's landing cap will lift. Closing the rest of the gap needs
  COORDINATED Allied operations: UK and USA pooling troops, transports and
  escorts on one target. The side-shared plan book is the natural place to
  hang a joint operation.

- **Byelorussia (non-VC) still ping-pongs** (~6 ownership changes a game) as
  the buffer between the German and Soviet lines. Cosmetically odd, probably
  harmless; the front line IS there.

## Battle screen

- **No per-round decisions.** Battles resolve start-to-finish with no retreat
  option and engine-chosen casualties (the original plan: "at some point we'd
  have all the options available to players during battles"). The engine
  already supports retreat deciders (`ResolveBattleWithRetreat`); the web
  layer always passes nil. Wants: round-by-round display, retreat button
  (amphibious attackers excluded — they have no retreat origin, by rule),
  casualty selection, submarine submerge.

## Fog of war (added with the feature)

- **Only amphibious operations leak.** The 5%-per-turn reveal roll covers
  amphibious plans; defence garrisons and naval squadrons are always secret
  and never leak. Extend if garrison intelligence should be obtainable.

- **The terminal UI shows unredacted transcripts.** Redaction happens in the
  web server's transcript rendering; the TUI still prints everything.

- **Human operations are invisible to NPC allies** (no way to register a
  human plan in the shared book). Accepted for now.

## UX

- **Empty or thinned destination sets are unexplained.** When a picked group's
  reachable intersection is empty (or a specific territory is missing — e.g. a
  strict neutral you cannot afford to violate), the player gets no reason.
  The server could return per-piece counts or blocked-reasons so the picker
  can say "your armor cannot reach there" or "violating Turkey needs 3 IPCs".

- **Purchase earmarks are per unit type, last factory wins.** Buying infantry
  at two different factories in one turn earmarks all of it to the factory
  used last; "Place All As Bought" then sends everything there. Correct
  placement is still enforced server-side; only the convenience is coarse.

- **Industrial-complex repair is not exposed in the web UI.**
  `RepairIndustrialComplex` exists in the engine (purchase phase) but no
  endpoint or dialog offers it.

## Fixed (kept for the record)

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

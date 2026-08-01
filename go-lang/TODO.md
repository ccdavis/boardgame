# TODO — unresolved issues

Known gaps and deliberate simplifications, in rough priority order. Each was
found (or introduced knowingly) while building the map-driven web UI and the
human amphibious-assault path; none block play.

## Rules fidelity

- **Sea battle in the drop zone does not gate the landing.** A booked landing
  goes ashore when combat moves execute, even if the drop zone's sea battle is
  still pending. Under the rules the sea fight resolves first, and losing it
  drowns the landing force. The bombardment forfeit for a contested drop zone
  IS implemented; the sequencing is not. This is shared with the NPC's landing
  path (`LandAssaultTroops`) and needs battle-resolution ordering — sea zones
  before the shores they cover — in `ResolveBattle`/auto-resolve, plus a rule
  for troops whose transports die before they land.

- **Strict-neutral tolls are gated per attack, not per treasury.** Booking
  assaults (or overland attacks) on two strict neutrals with only 3 IPCs
  passes each attack's individual `canAttackNeutral` check; execution floors
  the treasury at zero rather than overdraw, so the second violation is
  effectively discounted. Needs a cumulative check at planning time. Applies
  equally to the pre-existing overland path.

- **Friendly unload is allowed during the combat-move phase.** The engine is
  deliberately lenient (`UnloadUnit` permits either movement phase); strictly,
  unloading onto friendly ground is noncombat movement only. Tightening this
  would also want UI copy explaining why the shore stopped glowing.

- **AAA cannot move at all** because `aaa.gdf` declares `0 movement`. Classic
  rules give AAA a move of 1 in noncombat. Board-data decision to revisit —
  the board is the authority, so fix it in the `.gdf` if it should change.

## Battle screen

- **No per-round decisions.** Battles resolve start-to-finish with no retreat
  option and engine-chosen casualties (the original plan: "at some point we'd
  have all the options available to players during battles"). The engine
  already supports retreat deciders (`ResolveBattleWithRetreat`); the web
  layer always passes nil. Wants: round-by-round display, retreat button
  (amphibious attackers excluded — they have no retreat origin, by rule),
  casualty selection, submarine submerge.

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

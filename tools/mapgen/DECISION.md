# M1 decision gate: is the TripleA geometry usable?

**Yes.** The remaining work is bounded and localised. Recorded here because the
plan made this a go/no-go point.

## Method

Land territories map by name (see `namemap.py`). Sea zones cannot — the board
prints no names on them, so the `aaa.gdf` sea names are the author's invention.
They are matched **structurally** instead, by `match_seas.py`:

- a sea zone's signature is the set of land territories it borders;
- open-ocean zones border no land, so a second signal is used — which other sea
  zones they border — fed back iteratively as more of the map becomes known;
- each round runs a global Hungarian assignment over the combined score, so a
  confident match in one place corrects a doubtful one elsewhere.

Converges in 7 rounds. Iteration matters: the land-only pass mis-assigned
`Eastern USA Atlantic → West US Sea Zone` and `Western US Pacific → Gulf of
Mexico`; both are correct after the sea-neighbour pass.

## Result

| | count |
|---|---|
| sea zones matched strongly (≥0.75) | 45 |
| matched acceptably (0.4–0.75) | 6 |
| **not matched** | **7** |

51 of 58 sea zones resolve automatically.

## The 7 that don't, and why

All seven are in the Indian Ocean, where `aaa.gdf` subdivides more finely than
TripleA. `aaa.gdf` has 12 zones in that basin; TripleA has 8. Three of the seven
(`Central Indian Ocean`, `Bay of Bengal`, `Southern Indian Ocean`) do land on the
right TripleA zones — the `Compass` zones are TripleA's Indian Ocean — and merely
score low because neither side has a land signature to compare.

That leaves **4 genuine shortfalls**: `North Central Indian Ocean`,
`North East Indian Ocean`, `Eastern Indian Ocean`, and
`Antarctic Ocean southwest of Australia` have no TripleA counterpart and must be
cut out of the `Compass` / `Indian Ocean` zones.

Symmetrically, 4 TripleA zones are **surplus** — they exist on that board but not
on this one, and must be merged away:

| Surplus zone | Disposition |
|---|---|
| `Caspian Sea Zone` | `aaa.gdf` has no Caspian; Russia/Kazakstan/Caucases/Iran border each other directly. Drop. |
| `East Mediteranean Sea Zone` | TripleA has 3 Mediterranean zones, `aaa.gdf` has 2. Merge into `Eastern Mediteranian`. |
| `Gulf of Mexico Sea Zone` | Merge into `Eastern USA Atlantic`. |
| `West Panama Sea Zone` | Merge into `Mexican Pacific`. |

## Total manual geometry work

| Operation | Count |
|---|---|
| Land splits | 6 — Columbia→Colombia+Venezuela; Argentina-Chile→Argentina+Chile; Syria Jordan→Syria+Palestine+Iraq; and for the board extension: Finland Norway→Norway+Finland, East Europe→Eastern Europe+Byelorussia, South Europe→Southern Europe+Italy |
| Sea splits | 4 (Indian Ocean, above) |
| Merges | 4 (surplus zones, above) |

Ten cut lines and four merges. That is an afternoon of work against roughly a
week of computer vision, and the topology is already correct because it was
traced for an engine that plays on it.

## Caveat that survives

The two boards are *not* the same tessellation, only substantially similar.
`aaa.gdf` remains the authority: where the two disagree, the `.gdf` graph wins
and the geometry is cut or merged to match it. The layout validator
(`go-lang/layout`) is what proves that happened — it checks the generated
polygons against the `.gdf` adjacency graph rather than trusting this mapping.

# Vendored geometry: TripleA "World War II Classic"

These files are the territory geometry for the Axis & Allies Classic board
(Milton Bradley, 1984/86) — the same board `aaa.gdf` describes.

| | |
|---|---|
| Source | https://github.com/triplea-maps/world_war_ii_classic |
| Commit | `ea60168b53509e86b3d9a4cd45e91832a4e7c187` |
| Files | `polygons.txt`, `centers.txt`, `map.properties` |
| Canvas | 3500 x 2000 px (from `map.properties`) |
| Contents | 128 territories: 58 sea zones, 70 land |

Vendored rather than fetched at build time so the pipeline is reproducible and
offline, and so a change upstream cannot silently alter the generated map.

## Why this source

`aaa.gdf` and this map describe the same physical board. Measured correspondence:

| | TripleA | `aaa.gdf` |
|---|---|---|
| sea zones | 58 | 58 |
| land | 70 | 69 |
| adjacency edges | 313 (derived from polygons) | 293 |
| mean degree | 4.89 | 4.6 |
| degree-1 islands | 15 | 15 |

The island sets correspond almost one-to-one (Britain/United Kingdom,
West Indies/Cuba, Borneo/Borneo Celebes, Hawaii/Hawaiian Islands, ...), which is
strong evidence of the same tessellation. `aaa.gdf` subdivides the Indian Ocean
more finely, so some zones need splitting — see `../../namemap.py`.

## Licensing

**This repository has no LICENSE file**, and the content is derivative of
Hasbro / Milton Bradley intellectual property. That is fine for personal use
alongside a board you own. It is *not* cleared for redistribution or publication.
If this project is ever published, the geometry must be re-derived (see the
marker-controlled-watershed fallback described in the project plan) or replaced.

## Format

`polygons.txt`, one territory per line:

    Territory Name  < (x,y) (x,y) ... >  < (x,y) ... >

Each `< ... >` is one ring. The first ring is the outer boundary; further rings
are additional disjoint pieces or holes. Coordinates are integer pixels on the
3500x2000 canvas, y increasing downward.

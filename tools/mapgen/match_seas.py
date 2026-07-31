"""Match aaa.gdf sea zones to TripleA sea zones structurally.

The board prints no names on its sea zones, so the aaa.gdf names are invented
and cannot be matched by string similarity. What *is* shared between the two
descriptions is topology: a sea zone borders a particular set of land
territories, and the land names do map by name.

Coastal zones are pinned by their land signature. Open-ocean zones touch no land
at all, so they are resolved by a second signal -- which *other sea zones* they
border -- fed back iteratively as more of the map becomes known. Each round runs
a global assignment (Hungarian) over the combined score, so a confident match in
one place corrects a doubtful one elsewhere.

Run:  uv run --with shapely --with scipy --with numpy python tools/mapgen/match_seas.py
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import numpy as np
from scipy.optimize import linear_sum_assignment

import gdfgraph
import namemap
import triplea

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
GDF = os.path.join(ROOT, "aaa.gdf")
POLYS = os.path.join(HERE, "vendor", "triplea_classic", "polygons.txt")
OUT = os.path.join(HERE, "sea_match.json")

ROUNDS = 12
LAND_WEIGHT = 1.0
SEA_WEIGHT = 1.0


def jaccard(a, b):
    if not a or not b:
        return 0.0
    return len(a & b) / len(a | b)


def build():
    kinds, gdf_adj = gdfgraph.load(GDF)
    polys = triplea.load_polygons(POLYS)
    tri_adj = triplea.derive_adjacency(polys)

    gdf_seas = sorted(t for t, k in kinds.items() if k == "water")
    tri_seas = sorted(t for t in polys if triplea.is_sea(t))
    land_map = namemap.land_to_triplea()

    gdf_land_sig = {
        s: {land_map[n] for n in gdf_adj[s] if kinds.get(n) == "land" and n in land_map}
        for s in gdf_seas
    }
    tri_land_sig = {
        s: {n for n in tri_adj[s] if not triplea.is_sea(n)} for s in tri_seas
    }
    gdf_sea_nb = {
        s: {n for n in gdf_adj[s] if kinds.get(n) == "water"} for s in gdf_seas
    }
    tri_sea_nb = {s: {n for n in tri_adj[s] if triplea.is_sea(n)} for s in tri_seas}

    return (gdf_seas, tri_seas, gdf_land_sig, tri_land_sig,
            gdf_sea_nb, tri_sea_nb, kinds, gdf_adj, tri_adj)


def main():
    (gdf_seas, tri_seas, gdf_land_sig, tri_land_sig,
     gdf_sea_nb, tri_sea_nb, kinds, gdf_adj, tri_adj) = build()

    print(f"gdf sea zones: {len(gdf_seas)}   triplea sea zones: {len(tri_seas)}")

    land_score = np.zeros((len(gdf_seas), len(tri_seas)))
    for i, g in enumerate(gdf_seas):
        for j, t in enumerate(tri_seas):
            land_score[i, j] = jaccard(gdf_land_sig[g], tri_land_sig[t])

    mapping = {}
    for rnd in range(ROUNDS):
        sea_score = np.zeros_like(land_score)
        if mapping:
            for i, g in enumerate(gdf_seas):
                # Which TripleA zones do g's already-identified sea neighbours map to?
                projected = {mapping[n] for n in gdf_sea_nb[g] if n in mapping}
                if not projected:
                    continue
                for j, t in enumerate(tri_seas):
                    sea_score[i, j] = jaccard(projected, tri_sea_nb[t])

        total = LAND_WEIGHT * land_score + SEA_WEIGHT * sea_score
        rows, cols = linear_sum_assignment(-total)
        new_mapping = {gdf_seas[i]: tri_seas[j] for i, j in zip(rows, cols)}
        scores = {gdf_seas[i]: total[i, j] for i, j in zip(rows, cols)}

        if new_mapping == mapping:
            print(f"converged after {rnd} round(s)")
            break
        mapping = new_mapping
    else:
        print(f"stopped after {ROUNDS} rounds without full convergence")

    ranked = sorted(mapping, key=lambda g: -scores[g])
    strong = [g for g in ranked if scores[g] >= 0.75]
    ok = [g for g in ranked if 0.4 <= scores[g] < 0.75]
    weak = [g for g in ranked if scores[g] < 0.4]

    print(f"\ncombined score:  strong(>=0.75)={len(strong)}  "
          f"ok(0.4-0.75)={len(ok)}  weak(<0.4)={len(weak)}")

    for label, group in (("STRONG", strong), ("OK", ok), ("WEAK - REVIEW", weak)):
        print(f"\n--- {label} ---")
        for g in group:
            print(f"  {scores[g]:.2f}  {g:<38} -> {mapping[g]}")
            if label.startswith("WEAK"):
                print(f"        gdf land: {sorted(gdf_land_sig[g])}")
                print(f"        tri land: {sorted(tri_land_sig[mapping[g]])}")

    assert len(set(mapping.values())) == len(mapping), "assignment is not a bijection"

    with open(OUT, "w") as fh:
        json.dump({g: mapping[g] for g in sorted(mapping)}, fh, indent=2)
    print(f"\nwrote {OUT}")


if __name__ == "__main__":
    main()

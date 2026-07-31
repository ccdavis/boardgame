"""Generate aaa.layout.json from the vendored TripleA geometry.

    uv run --with shapely --with topojson --with numpy --with scipy \
        python tools/mapgen/build.py

Pipeline:  load -> rename -> merge -> split -> drop -> simplify -> normalise
           -> anchors -> emit

Simplification uses topojson so that a border shared by two territories is
simplified *once*, as a shared arc. Simplifying each polygon independently
would open gaps between neighbours and fail the layout validator's contact
check everywhere.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from shapely.geometry import MultiPolygon, Point, Polygon
from shapely.ops import unary_union

import gdfgraph
import namemap
import partition
import triplea

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
GDF = os.path.join(ROOT, "aaa.gdf")
POLYS = os.path.join(HERE, "vendor", "triplea_classic", "polygons.txt")
SPLITS_FILE = os.path.join(HERE, "splits.json")
SEA_MATCH = os.path.join(HERE, "sea_match.json")
OUT = os.path.join(ROOT, "aaa.layout.json")

VIEW_WIDTH = 1000.0
SIMPLIFY = float(os.environ.get("SIMPLIFY", 2.0))   # in normalised units
# Contact metric, kept deliberately consistent with go-lang/layout/validate.go:
# two borders count as shared when they run within CONTACT_EPS of each other for
# at least LMIN of length. Buffer-intersection area approximates that length as
# area / (2 * eps), which is close enough to keep the two implementations from
# disagreeing about which edges belong in `links`.
CONTACT_EPS = 0.75
LMIN = 4.0          # must match layout.Lmin in go-lang/layout/validate.go
MIN_POLY_AREA = 0.3     # drop slivers thrown off by merges and simplification
PRECISION = 1                                        # decimals emitted


def log(msg):
    print(msg, file=sys.stderr)


def resolve_geometries(polys, kinds):
    """Map every gdf territory name to a TripleA-space geometry."""
    split_spec = {k: v for k, v in json.load(open(SPLITS_FILE)).items()
                  if not k.startswith("_")}
    sea_match = json.load(open(SEA_MATCH))

    out, unresolved = {}, []
    claimed = set()  # TripleA regions consumed by a split, merge or drop

    # Splits first, so they take precedence over the plain name mapping.
    #
    # Anchors for territories that do not exist in this .gdf yet are filtered
    # out, and the partition runs over whatever remains. Because a Voronoi
    # partition always tiles the whole region, dropping anchors never leaves a
    # hole -- the survivors simply absorb the area. With nothing left, the
    # region passes through whole under its un-split name.
    for tri_name, anchors in split_spec.items():
        if tri_name not in polys:
            log(f"  split source {tri_name!r} not in polygons.txt, skipped")
            continue
        claimed.add(tri_name)
        present = {n: pts for n, pts in anchors.items() if n in kinds}

        if len(present) >= 2:
            for name, geom in partition.split_region(polys[tri_name], present).items():
                out[name] = geom
            produced = set(partition.split_region(polys[tri_name], present))
            if set(present) - produced:
                log(f"  split of {tri_name!r} produced nothing for "
                    f"{sorted(set(present) - produced)}")
        elif len(present) == 1:
            only = next(iter(present))
            out[only] = polys[tri_name]
            log(f"  {tri_name!r}: only {only!r} exists yet, kept whole")
        else:
            fallback = _reverse_name(tri_name, sea_match)
            if fallback and fallback in kinds:
                out[fallback] = polys[tri_name]
                log(f"  {tri_name!r}: no split targets yet, kept whole as {fallback!r}")
            else:
                log(f"  {tri_name!r}: no split targets and no fallback name")

    # Merges: several TripleA regions become one gdf territory.
    for gdf_name, tri_names in namemap.MERGES.items():
        parts = [polys[t] for t in tri_names if t in polys]
        claimed.update(tri_names)
        if parts:
            out[gdf_name] = unary_union(parts)

    # Land, via the name table.
    for gdf_name, tri_name in namemap.LAND.items():
        if gdf_name in out or tri_name in claimed:
            continue
        if tri_name in polys:
            out[gdf_name] = polys[tri_name]
        else:
            unresolved.append(gdf_name)

    # Sea: hand-pinned overrides, then the structural match.
    for gdf_name, tri_name in namemap.SEA_OVERRIDE.items():
        if gdf_name in kinds and tri_name in polys:
            out[gdf_name] = polys[tri_name]
            claimed.add(tri_name)
    for gdf_name, tri_name in sea_match.items():
        if gdf_name in out or tri_name in claimed:
            continue
        if tri_name in polys:
            out[gdf_name] = polys[tri_name]
        else:
            unresolved.append(gdf_name)

    return out, unresolved


def _reverse_name(tri_name, sea_match):
    """The gdf name that maps to this TripleA region when it is not split."""
    for gdf_name, mapped in namemap.LAND.items():
        if mapped == tri_name:
            return gdf_name
    for gdf_name, mapped in sea_match.items():
        if mapped == tri_name:
            return gdf_name
    return None


def normalise(geoms):
    """Scale TripleA canvas coordinates into a 0..VIEW_WIDTH viewBox."""
    xs0 = min(g.bounds[0] for g in geoms.values())
    ys0 = min(g.bounds[1] for g in geoms.values())
    xs1 = max(g.bounds[2] for g in geoms.values())
    ys1 = max(g.bounds[3] for g in geoms.values())

    scale = VIEW_WIDTH / (xs1 - xs0)
    height = round((ys1 - ys0) * scale, 1)

    def tx(geom):
        from shapely.affinity import affine_transform
        return affine_transform(geom, [scale, 0, 0, scale,
                                       -xs0 * scale, -ys0 * scale])

    return {n: tx(g) for n, g in geoms.items()}, height


def simplify_shared(geoms):
    """Simplify with topojson so shared borders stay shared."""
    try:
        import topojson as tp
    except ImportError:
        log("  topojson not available; falling back to per-polygon simplify "
            "(borders may separate)")
        return {n: g.simplify(SIMPLIFY, preserve_topology=True)
                for n, g in geoms.items()}

    names = list(geoms)
    topo = tp.Topology([geoms[n] for n in names], prequantize=False, shared_coords=True)
    simplified = topo.toposimplify(SIMPLIFY).to_gdf()
    return {names[i]: simplified.geometry.iloc[i] for i in range(len(names))}


def open_water(geom, land_geoms):
    """A sea zone minus the islands drawn on top of it.

    Sea polygons cover the islands sitting in them -- East Indies inside East
    Indies Ocean, Japan inside the Sea of Japan -- and the island is painted
    above the sea. Anchoring on the raw sea polygon can therefore put the label
    and the click target on the island, so clicking the sea selects the island
    instead. The rendered shape keeps the island area (the water still shows
    around it); only the anchor is computed on the water that is actually
    exposed.
    """
    exposed = geom
    for land in land_geoms:
        if not land.intersects(geom):
            continue
        try:
            trimmed = exposed.difference(land)
        except Exception:
            continue
        if not trimmed.is_empty:
            exposed = trimmed
    return exposed if not exposed.is_empty else geom


def label_anchor(geom):
    """Pole of inaccessibility: guaranteed inside, unlike the centroid."""
    target = geom
    if isinstance(geom, MultiPolygon):
        target = max(geom.geoms, key=lambda p: p.area)
    try:
        from shapely.algorithms.polylabel import polylabel
        pt = polylabel(target, tolerance=0.4)
    except Exception:
        pt = target.representative_point()
    radius = pt.distance(target.exterior) if isinstance(target, Polygon) else 0.0
    return pt, radius


def marker_anchor(geom, label_pt, radius):
    """Where the unit-count badge sits.

    Offset below the label so the two do not collide -- but only when the region
    is big enough to hold both. On the small island groups (Wake, Midway,
    Caroline, Solomon) any offset at all puts the badge in open water, so those
    keep it on the label. The validator enforces this: an anchor outside its
    region would silently break the click test that drives clicks at anchors.
    """
    offset = max(radius * 0.45, 4.0)
    candidate = Point(label_pt.x, label_pt.y + offset)
    if geom.covers(candidate):
        return candidate
    for factor in (0.6, 0.35, 0.15):
        candidate = Point(label_pt.x, label_pt.y + offset * factor)
        if geom.covers(candidate):
            return candidate
    return label_pt


def rings_of(geom):
    out = []
    parts = geom.geoms if isinstance(geom, MultiPolygon) else [geom]
    for p in parts:
        if p.geom_type != "Polygon" or p.is_empty:
            continue
        if p.area < MIN_POLY_AREA:
            continue
        rings = [list(p.exterior.coords)[:-1]]
        rings += [list(r.coords)[:-1] for r in p.interiors]
        out.append([[[round(x, PRECISION), round(y, PRECISION)] for x, y in r]
                    for r in rings])
    return out


def unrealised_edges(geoms, adjacency, view_width):
    """Declared adjacencies that the geometry does not express as a shared border.

    These are genuine disagreements between the board's drawn regions and the
    .gdf graph -- verified not to be an artefact of simplification, since the
    same set appears with simplification turned off entirely. Two causes:

      * the seam. The board splits North America across both edges, so e.g.
        Eastern US and Western US are adjacent in play but sit at opposite rims.
      * the .gdf abstracts away geography, e.g. it has Iraq bordering the
        Indian Ocean, standing in for the Persian Gulf.

    They are emitted as `links` so the front end can draw them as explicit
    connectors, and so the layout validator can tell "known and accepted" from
    "newly broken".
    """
    names = list(geoms)
    boundary = {n: geoms[n].boundary for n in names}
    near = {n: boundary[n].buffer(CONTACT_EPS) for n in names}
    touching = set()
    for i, a in enumerate(names):
        for b in names[i + 1:]:
            if not near[a].intersects(near[b]):
                continue
            # Length of a's boundary running within CONTACT_EPS of b's -- the
            # same quantity go-lang/layout.SharedBorder estimates by sampling.
            if boundary[a].intersection(near[b]).length >= LMIN:
                touching.add(frozenset((a, b)))

    declared = {frozenset((a, b)) for a, nbs in adjacency.items() for b in nbs
                if a in geoms and b in geoms}

    def on_rim(n):
        x0, _, x1, _ = geoms[n].bounds
        return x0 < 3 or x1 > view_width - 3

    links = []
    for edge in sorted(declared - touching, key=lambda e: sorted(e)):
        a, b = sorted(edge)
        reason = "seam" if on_rim(a) and on_rim(b) else "no shared border"
        links.append({"a": a, "b": b, "why": reason,
                      "gap": round(geoms[a].distance(geoms[b]), 1)})

    spurious = len(touching - declared)
    return links, len(declared), len(declared & touching), spurious


def main():
    kinds, adjacency = gdfgraph.load(GDF)
    polys = triplea.load_polygons(POLYS)
    for name in namemap.DROP:
        polys.pop(name, None)

    log(f"gdf territories: {len(kinds)}   triplea polygons: {len(polys)}")

    geoms, unresolved = resolve_geometries(polys, kinds)
    log(f"resolved geometry for {len(geoms)} of {len(kinds)} territories")

    missing = sorted(set(kinds) - set(geoms))
    extra = sorted(set(geoms) - set(kinds))
    if missing:
        log(f"  NO GEOMETRY ({len(missing)}): {missing}")
    if extra:
        log(f"  NOT IN GDF ({len(extra)}): {extra}")
    if unresolved:
        log(f"  UNRESOLVED NAMES: {unresolved}")

    geoms, height = normalise(geoms)
    geoms = simplify_shared(geoms)

    territories = {}
    land_geoms = [g for n, g in geoms.items() if kinds.get(n) != "water"]

    for name in sorted(geoms):
        geom = geoms[name]
        if geom.is_empty:
            log(f"  {name!r} simplified away to nothing")
            continue

        # Anchor sea zones in exposed water, never on an island they contain.
        anchor_on = geom
        if kinds.get(name) == "water":
            anchor_on = open_water(geom, land_geoms)

        pt, radius = label_anchor(anchor_on)
        rings = rings_of(geom)
        if not rings:
            log(f"  {name!r} has no polygon above the sliver threshold")
            continue
        mk = marker_anchor(anchor_on, pt, radius)
        x0, y0, x1, y1 = geom.bounds
        territories[name] = {
            "kind": "sea" if kinds.get(name) == "water" else "land",
            "polygons": rings,
            "label": {"x": round(pt.x, PRECISION), "y": round(pt.y, PRECISION),
                      "r": round(radius, PRECISION)},
            "marker": {"x": round(mk.x, PRECISION), "y": round(mk.y, PRECISION)},
            "bbox": [round(v, PRECISION) for v in (x0, y0, x1, y1)],
        }

    links, declared, realised, spurious = unrealised_edges(geoms, adjacency, VIEW_WIDTH)
    pct = 100.0 * realised / declared if declared else 0.0
    log(f"adjacency: {realised}/{declared} declared edges realised as shared "
        f"borders ({pct:.1f}%); {len(links)} recorded as links; "
        f"{spurious} contacts not declared in the .gdf")
    for link in links:
        log(f"    link  {link['a']} -- {link['b']}  ({link['why']}, gap {link['gap']})")

    doc = {
        "schemaVersion": 1,
        "sourceGdf": "aaa.gdf",
        "generatedBy": "tools/mapgen/build.py  src=triplea-maps/world_war_ii_classic"
                       "@ea60168b53509e86b3d9a4cd45e91832a4e7c187"
                       f"  simplify={SIMPLIFY}",
        "viewBox": [0, 0, VIEW_WIDTH, height],
        "views": {},
        "territories": territories,
        "links": links,
    }

    # Compact: coordinate arrays dominate the file, and pretty-printing them
    # costs several hundred KB for no readability worth having.
    with open(OUT, "w") as fh:
        json.dump(doc, fh, separators=(",", ":"))

    verts = sum(len(r) for t in territories.values()
                for poly in t["polygons"] for r in poly)
    log(f"wrote {OUT}: {len(territories)} territories, {verts} vertices, "
        f"{os.path.getsize(OUT) // 1024} KB")


if __name__ == "__main__":
    main()

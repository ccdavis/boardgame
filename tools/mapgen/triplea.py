"""Reader and adjacency derivation for TripleA polygons.txt."""

import re
from shapely.geometry import Polygon
from shapely.ops import unary_union

RING = re.compile(r"<\s*((?:\(\s*-?\d+\s*,\s*-?\d+\s*\)\s*)+)>")
POINT = re.compile(r"\(\s*(-?\d+)\s*,\s*(-?\d+)\s*\)")


def load_polygons(path):
    """Return {territory name: shapely geometry}.

    Rings after the first are treated as additional disjoint pieces (islands),
    which is how this map uses them -- Philippines has 8, East Indies 7.
    """
    out = {}
    for line in open(path):
        line = line.strip()
        if not line or "<" not in line:
            continue
        name = line.split("<", 1)[0].strip()
        parts = []
        for ring in RING.findall(line):
            pts = [(int(x), int(y)) for x, y in POINT.findall(ring)]
            if len(pts) < 3:
                continue
            poly = Polygon(pts)
            if not poly.is_valid:
                poly = poly.buffer(0)
            if not poly.is_empty:
                parts.append(poly)
        if parts:
            out[name] = unary_union(parts)
    return out


def is_sea(name):
    return "Sea Zone" in name


def derive_adjacency(polys, tolerance=1.5, min_contact_area=6.0):
    """Adjacency by polygon contact.

    The source coordinates are integer pixels, so touching borders are often a
    pixel or two apart. Buffering by `tolerance` and requiring a minimum overlap
    area filters out corner kisses without dropping real shared borders.
    """
    names = list(polys)
    buffered = {n: polys[n].buffer(tolerance) for n in names}
    boxes = {n: buffered[n].bounds for n in names}

    adjacency = {n: set() for n in names}
    for i, a in enumerate(names):
        ax0, ay0, ax1, ay1 = boxes[a]
        for b in names[i + 1:]:
            bx0, by0, bx1, by1 = boxes[b]
            if ax1 < bx0 or bx1 < ax0 or ay1 < by0 or by1 < ay0:
                continue
            if not buffered[a].intersects(buffered[b]):
                continue
            if buffered[a].intersection(buffered[b]).area <= min_contact_area:
                continue
            adjacency[a].add(b)
            adjacency[b].add(a)
    return adjacency

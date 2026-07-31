"""Split a polygon into named pieces by nearest-anchor partition."""

from shapely.geometry import MultiPoint, Point
from shapely.ops import unary_union, voronoi_diagram


def split_region(geom, anchors_by_name, buffer=0.0):
    """Partition `geom` among named anchor sets.

    anchors_by_name: {name: [(x, y), ...]}

    Every point of `geom` goes to the name owning the nearest anchor, so the
    pieces exactly tile the original with no gaps or overlaps -- which is what
    the layout validator's coverage and overlap checks require.

    Returns {name: geometry}. Names whose cells end up empty are omitted, which
    the caller should treat as a badly placed anchor rather than ignore.
    """
    flat, owner = [], []
    for name, points in anchors_by_name.items():
        for xy in points:
            flat.append(Point(xy))
            owner.append(name)

    if len(flat) < 2:
        raise ValueError("a split needs at least two anchors")

    outside = [(name, (pt.x, pt.y)) for pt, name in zip(flat, owner)
               if not geom.covers(pt)]
    if outside:
        listing = ", ".join(f"{n} at {xy}" for n, xy in outside)
        raise ValueError(f"{len(outside)} anchor(s) outside the region: {listing}")

    # extend_to keeps outer cells finite so they can be clipped to the region.
    envelope = geom.envelope.buffer(max(geom.bounds[2] - geom.bounds[0],
                                        geom.bounds[3] - geom.bounds[1]) + 10)
    cells = list(voronoi_diagram(MultiPoint(flat), envelope=envelope).geoms)

    # voronoi_diagram does not preserve input order, so match cells to anchors.
    pieces = {}
    for cell in cells:
        holder = None
        for pt, name in zip(flat, owner):
            if cell.covers(pt):
                holder = name
                break
        if holder is None:
            continue
        part = cell.intersection(geom)
        if part.is_empty:
            continue
        pieces.setdefault(holder, []).append(part)

    out = {}
    for name, parts in pieces.items():
        merged = unary_union(parts)
        if buffer:
            merged = merged.buffer(buffer).buffer(-buffer)
        if not merged.is_empty:
            out[name] = merged
    return out

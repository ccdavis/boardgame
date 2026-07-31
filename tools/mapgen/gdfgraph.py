"""Minimal reader for the parts of a .gdf the geometry pipeline needs.

Deliberately independent of the Go parser: this runs at build time and only
needs territory kinds and the adjacency graph. It reproduces one behaviour of
the Go scanner exactly -- names are token sequences, so internal whitespace
collapses and a name may wrap across source lines.
"""

import re


def _norm(name):
    """Collapse whitespace the way the Go scanner does when it joins tokens."""
    return " ".join(name.split())


def _section(text, name, following):
    body = text.split(name, 1)[1]
    for nxt in following:
        if nxt in body:
            body = body.split(nxt, 1)[0]
    return re.sub(r"#.*", "", body)


def load(path):
    """Return (kinds, adjacency).

    kinds:     {territory: "land" | "water"}
    adjacency: {territory: set(neighbours)}, symmetrised
    """
    text = open(path).read()

    kinds = {}
    for stmt in _section(text, "Territories", ["Map"]).split(";"):
        m = re.match(r"\s*(.+?)\s*:\s*(land|water|both)\b", stmt.strip())
        if m:
            kinds[_norm(m.group(1))] = m.group(2)

    adjacency = {t: set() for t in kinds}
    for stmt in _section(text, "Map", ["Units"]).split(";"):
        stmt = stmt.strip()
        if not stmt or ":" not in stmt:
            continue
        head, tail = stmt.split(":", 1)
        src = _norm(head)
        if src not in adjacency:
            continue
        for nb in (_norm(x) for x in tail.split(",") if x.strip()):
            if nb in adjacency:
                adjacency[src].add(nb)
                adjacency[nb].add(src)

    return kinds, adjacency

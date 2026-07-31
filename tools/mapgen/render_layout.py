"""Render aaa.layout.json to a PNG for visual review.

    uv run --with matplotlib python tools/mapgen/render_layout.py [out.png]

This is the build-time preview. The real check is the Playwright suite against
the running app; this exists so the geometry can be judged before any of the
front end is wired up.
"""

import json
import os
import sys

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import Polygon as MplPolygon

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
LAYOUT = os.path.join(ROOT, "aaa.layout.json")

LAND = "#D8C9A3"
LAND_EDGE = "#4A4034"
SEA = "#A8CBE0"
SEA_EDGE = "#FFFFFF"


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(HERE, "layout_preview.png")
    show_labels = os.environ.get("LABELS", "1") == "1"

    doc = json.load(open(LAYOUT))
    vx, vy, vw, vh = doc["viewBox"]

    fig, ax = plt.subplots(figsize=(22, 22 * vh / vw))
    ax.add_patch(MplPolygon([(vx, vy), (vx + vw, vy), (vx + vw, vy + vh), (vx, vy + vh)],
                            closed=True, fc=SEA, ec="none", zorder=0))

    counts = {"land": 0, "sea": 0}
    for name, t in doc["territories"].items():
        is_sea = t["kind"] == "sea"
        counts[t["kind"]] += 1
        for rings in t["polygons"]:
            ax.add_patch(MplPolygon(
                rings[0], closed=True,
                fc=SEA if is_sea else LAND,
                ec=SEA_EDGE if is_sea else LAND_EDGE,
                lw=0.5 if is_sea else 0.8,
                zorder=1 if is_sea else 2))
        if show_labels:
            lab = t["label"]
            ax.text(lab["x"], lab["y"], name,
                    fontsize=3.6 if is_sea else 4.6,
                    ha="center", va="center", zorder=3,
                    color="#2A4A5A" if is_sea else "#1A1410",
                    style="italic" if is_sea else "normal")

    ax.set_xlim(vx, vx + vw)
    ax.set_ylim(vy + vh, vy)   # SVG convention: y increases downward
    ax.set_aspect("equal")
    ax.axis("off")
    plt.tight_layout(pad=0.2)
    plt.savefig(out, dpi=135, facecolor="#EAF2F7")
    print(f"wrote {out}  ({counts['land']} land, {counts['sea']} sea)")


if __name__ == "__main__":
    main()

"""Render each split so the anchors can be judged visually.

Run: uv run --with shapely --with numpy --with matplotlib python tools/mapgen/render_splits.py
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from shapely.geometry import MultiPolygon

import partition
import triplea

HERE = os.path.dirname(os.path.abspath(__file__))
POLYS = os.path.join(HERE, "vendor", "triplea_classic", "polygons.txt")
SPLITS = os.path.join(HERE, "splits.json")
OUT = os.environ.get("SPLIT_RENDER", os.path.join(HERE, "splits_preview.png"))

COLORS = ["tab:orange", "tab:green", "tab:purple", "tab:red", "tab:brown"]


def draw(ax, geom, **kw):
    geoms = geom.geoms if isinstance(geom, MultiPolygon) else [geom]
    for p in geoms:
        if p.geom_type != "Polygon":
            continue
        ax.fill(*p.exterior.xy, **kw)


def main():
    polys = triplea.load_polygons(POLYS)
    spec = {k: v for k, v in json.load(open(SPLITS)).items() if not k.startswith("_")}

    cols = 3
    rows = (len(spec) + cols - 1) // cols
    fig, axes = plt.subplots(rows, cols, figsize=(6 * cols, 5 * rows))
    axes = axes.ravel() if hasattr(axes, "ravel") else [axes]

    for ax, (region, anchors) in zip(axes, spec.items()):
        geom = polys[region]
        pieces = partition.split_region(geom, anchors)

        for other, og in polys.items():
            if other != region and og.buffer(2).intersects(geom.buffer(2)):
                draw(ax, og, alpha=0.12, fc="tab:blue", ec="gray", lw=0.4)
                ax.text(og.centroid.x, og.centroid.y, other, fontsize=5.5,
                        ha="center", color="navy")

        for i, (name, piece) in enumerate(sorted(pieces.items())):
            draw(ax, piece, alpha=0.65, fc=COLORS[i % len(COLORS)], ec="k", lw=1.2)
            ax.text(piece.centroid.x, piece.centroid.y, name, fontsize=9,
                    ha="center", weight="bold")
        for name, pts in anchors.items():
            for x, y in pts:
                ax.plot(x, y, "kx", ms=7, mew=2)

        missing = set(anchors) - set(pieces)
        title = region + (f"   MISSING: {sorted(missing)}" if missing else "")
        ax.set_title(title, fontsize=11, weight="bold")
        x0, y0, x1, y1 = geom.bounds
        m = 50
        ax.set_xlim(x0 - m, x1 + m)
        ax.set_ylim(y1 + m, y0 - m)
        ax.grid(alpha=0.35, lw=0.5)
        ax.tick_params(labelsize=7)

    for ax in axes[len(spec):]:
        ax.axis("off")

    plt.tight_layout()
    plt.savefig(OUT, dpi=95)
    print("wrote", OUT)


if __name__ == "__main__":
    main()

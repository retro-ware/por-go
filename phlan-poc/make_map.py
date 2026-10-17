#!/usr/bin/env python3
"""Remaster pipeline: blobber/maps/AREA.map from a Pool of Radiance GEO block
  python3 make_map.py [AREA]      (areas.py, default phlan)
(geo<disk>_<block>.bin, the 0x400 bytes after the block's 2-byte header,
written by `go run ./cmd/geodump DISK BLOCK OUT` from the repo root).

PoR walls are thin edges between 16x16 squares, two-sided (each square keeps
its own value per side). The map is 16x16 open floor, one cell per square,
and every walled side becomes, in the engine copy, a thin wall of its own:
"face X Y SIDE CHAR" draws it (only from inside that square, so a one-sided
wall is not seen from its empty side, as in PoR) and "edge X Y SIDE" blocks
stepping through it unless it is a door (PoR also checks only your own side).

GEO planes (engine/geo.go WallAt/DoorAt/MapCell): 0x000 north<<4|east,
0x100 south<<4|west, 0x200 the cell byte, 0x300 door bits (2 per side,
N E S W from bit 0)."""
import sys
from pathlib import Path
from areas import AREAS, origin

HERE = Path(__file__).resolve().parent
NAME = sys.argv[1] if sys.argv[1:] else "phlan"
A = AREAS[NAME]
g = (HERE / "geo{}_{}.bin".format(*A["geo"])).read_bytes()
POND = {v for v in A["walls"] if (o := origin(NAME, v))[1] in AREAS[o[0]].get("pond", ())}  # drawn as water floor
TEX = {v: "wall_{}_v{:02}".format(*origin(NAME, v)) for v in A["walls"]}  # reused walls: the first area's texture
START = A["start"]


def side(x, y, d):
    """(wall value, door) on side d (0 N, 1 E, 2 S, 3 W) of square (x, y)."""
    i = y * 16 + x
    v = [g[i] >> 4, g[i] & 15, g[0x100 + i] >> 4, g[0x100 + i] & 15][d]
    return v, v != 0 and ((g[0x300 + i] >> (2 * d) & 3) != 0 or (x, y, d) in A.get("doors", ()))


STEP = ((0, -1), (1, 0), (0, 1), (-1, 0))


def reach(start):
    """The squares a party can walk to from start."""
    land, todo = set(), [start]
    while todo:
        x, y = todo.pop()
        if (x, y) in land or not (0 <= x < 16 and 0 <= y < 16):
            continue
        land.add((x, y))
        for d, (dx, dy) in enumerate(STEP):
            v, door = side(x, y, d)
            if not v or door:
                todo.append((x + dx, y + dy))
    return land


if START is None:  # no entry square pinned: a square of the largest walkable region
    START = max(((x, y) for y in range(16) for x in range(16)), key=lambda c: len(reach(c))) + ("n",)


def trunks(v):
    """Where the trees of wall v stand along its face (0 left .. 1 right, as seen from inside) and their height in
    squares: from the original's art, the trunk columns in its bottom rows; a tree cut by an edge stands on it."""
    import numpy as np
    from PIL import Image
    from areas import src, DUMP
    a = np.asarray(Image.open(DUMP / f"{src(NAME, v)}.png").convert("RGBA"))[..., 3] > 0
    h, w = a.shape
    cols = a[int(h * 0.9):].any(axis=0)
    out, x = [], 0
    while x < w:
        if cols[x]:
            x0 = x
            while x < w and cols[x]:
                x += 1
            out.append(0.0 if x0 == 0 else 1.0 if x == w else (x0 + x) / 2 / w)
        x += 1
    rows = np.nonzero(a.any(axis=1))[0]
    return out, (rows.max() - rows.min() + 1) / h * 64 / 56


def main():
    extra = []
    props = {}  # (x, y) -> line: props on shared corners only once
    ponds = set()  # the square beyond a pond wall is water floor; the wall itself is not drawn
    for y in range(16):
        for x in range(16):
            for d in range(4):
                v, door = side(x, y, d)
                if v in A.get("props", {}):  # billboards where the trunks stand; still blocks like the wall
                    ends = [((x, y), (x + 1, y)), ((x + 1, y), (x + 1, y + 1)), ((x + 1, y + 1), (x, y + 1)),
                            ((x, y + 1), (x, y))][d]
                    ts, size = trunks(v)
                    for t in ts:
                        px, py = (round(ends[0][i] + (ends[1][i] - ends[0][i]) * t, 3) for i in (0, 1))
                        props[px, py] = f"prop {A['props'][v]} {px} {py} {size:.3f}"
                elif v in POND:
                    ponds.add((x + STEP[d][0], y + STEP[d][1]))
                elif v:
                    extra.append(f"face {x} {y} {d} {TEX.get(v, "?")}")
                if v:
                    if not door:
                        extra.append(f"edge {x} {y} {d}")
                    if v in A.get("nomap", ()):
                        extra.append(f"nomap {x} {y} {d}")
                    # stakes stand free: also seen from the far side (PoR stores them one-sided)
                    nx, ny = x + (0, 1, 0, -1)[d], y + (-1, 0, 1, 0)[d]
                    if v in A["free_walls"] and 0 <= nx < 16 and 0 <= ny < 16 and not side(nx, ny, d ^ 2)[0]:
                        extra.append(f"face {nx} {ny} {d ^ 2} {TEX.get(v, "?")}")
    # land: the squares a party can walk to from the start (doors pass, a one-sided wall only blocks
    # from its own side). Water: the squares beyond a harbour wall (water_walls) the party can't reach,
    # and what joins them; off the map the engine draws water anyway.
    land = reach(START[:2])
    water = set()
    todo = [(x + STEP[d][0], y + STEP[d][1]) for y in range(16) for x in range(16) for d in range(4)
            if side(x, y, d)[0] in A["water_walls"]]
    while todo:
        x, y = todo.pop()
        if (x, y) in water or (x, y) in land or not (0 <= x < 16 and 0 <= y < 16):
            continue
        water.add((x, y))
        todo += [(x + dx, y + dy) for d, (dx, dy) in enumerate(STEP) if not side(x, y, d)[0]]
    extra += [f"water {x} {y}" for x, y in sorted(water)]
    extra += [f"pond {x} {y}" for x, y in sorted(ponds)]
    for (x, y), things in A.get("cellprops", {}).items():
        for i, (kind, tex, *nums) in enumerate(things):
            props[x + 0.5, y + 0.5, i] = " ".join(map(str, [kind, tex, x + 0.5, y + 0.5, *nums]))
    extra += [props[k] for k in sorted(props)]
    lines = ["." * 16] * 16 + [f"floor {A['floor']}", f"ceiling {A['ceiling']}"] + extra
    (HERE / f"blobber/maps/{NAME}.map").write_text("\n".join(lines) + "\n")

    if NAME != "phlan":  # the campaign starts in Phlan; other areas are reached with BLOBBER_OPEN
        return
    world = HERE / "blobber/maps/world.txt"
    x, y, f = START
    world.write_text("".join(f"start phlan {x} {y} {f}\n" if l.startswith("start ") else l + "\n"
                             for l in world.read_text().splitlines()))


if __name__ == "__main__":
    main()

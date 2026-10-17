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
from areas import AREAS

HERE = Path(__file__).resolve().parent
NAME = sys.argv[1] if sys.argv[1:] else "phlan"
A = AREAS[NAME]
g = (HERE / "geo{}_{}.bin".format(*A["geo"])).read_bytes()
CHARS = A["chars"]  # wall value -> map character (walls.txt rows written below)
START = A["start"]


def side(x, y, d):
    """(wall value, door) on side d (0 N, 1 E, 2 S, 3 W) of square (x, y)."""
    i = y * 16 + x
    v = [g[i] >> 4, g[i] & 15, g[0x100 + i] >> 4, g[0x100 + i] & 15][d]
    return v, v != 0 and ((g[0x300 + i] >> (2 * d) & 3) != 0 or (x, y, d) in A.get("doors", ()))


def main():
    extra = []
    for y in range(16):
        for x in range(16):
            for d in range(4):
                v, door = side(x, y, d)
                if v:
                    extra.append(f"face {x} {y} {d} {CHARS[v]}")
                    if not door:
                        extra.append(f"edge {x} {y} {d}")
                    if v in A.get("nomap", ()):
                        extra.append(f"nomap {x} {y} {d}")
                    # stakes stand free: also seen from the far side (PoR stores them one-sided)
                    nx, ny = x + (0, 1, 0, -1)[d], y + (-1, 0, 1, 0)[d]
                    if v in A["free_walls"] and 0 <= nx < 16 and 0 <= ny < 16 and not side(nx, ny, d ^ 2)[0]:
                        extra.append(f"face {nx} {ny} {d ^ 2} {CHARS[v]}")
    # land: the squares a party can walk to from the start (doors pass, a one-sided wall only blocks
    # from its own side). Water: the squares beyond a harbour wall (water_walls) the party can't reach,
    # and what joins them; off the map the engine draws water anyway.
    STEP = ((0, -1), (1, 0), (0, 1), (-1, 0))
    land, todo = set(), [START[:2]]
    while todo:
        x, y = todo.pop()
        if (x, y) in land or not (0 <= x < 16 and 0 <= y < 16):
            continue
        land.add((x, y))
        for d, (dx, dy) in enumerate(STEP):
            v, door = side(x, y, d)
            if not v or door:
                todo.append((x + dx, y + dy))
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
    lines = ["." * 16] * 16 + [f"floor {A['floor']}", f"ceiling {A['ceiling']}"] + extra
    (HERE / f"blobber/maps/{NAME}.map").write_text("\n".join(lines) + "\n")

    walls = HERE / "blobber/assets/walls/walls.txt"
    rows = [l for l in walls.read_text().splitlines() if f"wall_{NAME}_" not in l]
    rows += [f"{c} wall_{NAME}_v{v:02}" for v, c in CHARS.items() if v in A["walls"]]
    walls.write_text("\n".join(rows) + "\n")

    if NAME != "phlan":  # the campaign starts in Phlan; other areas are reached with BLOBBER_OPEN
        return
    world = HERE / "blobber/maps/world.txt"
    x, y, f = START
    world.write_text("".join(f"start phlan {x} {y} {f}\n" if l.startswith("start ") else l + "\n"
                             for l in world.read_text().splitlines()))


if __name__ == "__main__":
    main()

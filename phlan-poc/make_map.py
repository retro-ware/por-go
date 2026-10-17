#!/usr/bin/env python3
"""New Phlan PoC: blobber/maps/phlan.map from Pool of Radiance GEO3 block 0
(geo3_0.bin, the 0x400 bytes after the block's 2-byte header).

PoR walls are thin edges between 16x16 squares, two-sided (each square keeps
its own value per side). The map is 16x16 open floor, one cell per square,
and every walled side becomes, in the engine copy, a thin wall of its own:
"face X Y SIDE CHAR" draws it (only from inside that square, so a one-sided
wall is not seen from its empty side, as in PoR) and "edge X Y SIDE" blocks
stepping through it unless it is a door (PoR also checks only your own side).

GEO planes (engine/geo.go WallAt/DoorAt/MapCell): 0x000 north<<4|east,
0x100 south<<4|west, 0x200 the cell byte, 0x300 door bits (2 per side,
N E S W from bit 0)."""
from pathlib import Path

HERE = Path(__file__).resolve().parent
g = (HERE / "geo3_0.bin").read_bytes()

# wall value -> map character (walls.txt rows written below)
CHARS = {1: "T", 3: "U", 5: "V", 7: "X", 9: "Y", 10: "Z", 2: "H", 4: "J", 6: "K", 8: "N", 11: "O", 12: "P",
         13: "Q", 14: "R", 15: "S"}
START = (7, 7, "n")  # the main north-south street


def side(x, y, d):
    """(wall value, door) on side d (0 N, 1 E, 2 S, 3 W) of square (x, y)."""
    i = y * 16 + x
    v = [g[i] >> 4, g[i] & 15, g[0x100 + i] >> 4, g[0x100 + i] & 15][d]
    return v, v != 0 and (g[0x300 + i] >> (2 * d) & 3) != 0


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
                    # stakes stand free: also seen from the far side (PoR stores them one-sided)
                    nx, ny = x + (0, 1, 0, -1)[d], y + (-1, 0, 1, 0)[d]
                    if v == 14 and 0 <= nx < 16 and 0 <= ny < 16 and not side(nx, ny, d ^ 2)[0]:
                        extra.append(f"face {nx} {ny} {d ^ 2} {CHARS[v]}")
    # land: the squares a party can walk to from the start (doors pass, a one-sided wall only blocks
    # from its own side). Water: the squares beyond a harbour wall (14, 15) the party can't reach,
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
            if side(x, y, d)[0] in (14, 15)]
    while todo:
        x, y = todo.pop()
        if (x, y) in water or (x, y) in land or not (0 <= x < 16 and 0 <= y < 16):
            continue
        water.add((x, y))
        todo += [(x + dx, y + dy) for d, (dx, dy) in enumerate(STEP) if not side(x, y, d)[0]]
    extra += [f"water {x} {y}" for x, y in sorted(water)]
    lines = ["." * 16] * 16 + ["floor phlan_dirt", "ceiling sky_phlan"] + extra
    (HERE / "blobber/maps/phlan.map").write_text("\n".join(lines) + "\n")

    walls = HERE / "blobber/assets/walls/walls.txt"
    rows = [l for l in walls.read_text().splitlines() if "wall_phlan_" not in l]
    rows += [f"{c} wall_phlan_v{v:02}" for v, c in CHARS.items()]
    walls.write_text("\n".join(rows) + "\n")

    world = HERE / "blobber/maps/world.txt"
    x, y, f = START
    world.write_text("".join(f"start phlan {x} {y} {f}\n" if l.startswith("start ") else l + "\n"
                             for l in world.read_text().splitlines()))


if __name__ == "__main__":
    main()

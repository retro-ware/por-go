"""Per-area settings for the remaster pipeline (make_map.py, make_textures.py,
compare/compare.py, see PROCESS.md). Each script takes the area name as its
first argument (default phlan).

Keys:
  geo         (disk, block) of GEO<disk>.DAX; the map is geo<disk>_<block>.bin
  walldefs    (block, slot) loads in the order the area's ECL does (opLoadFiles 37)
  area_kind   ECL block number, DS:84DC: 0 and 10 leave off-map squares empty, others wrap
  chunks      viewdump name of wall values 1-5, 6-10, 11-15 (WALLDEF<disk>-<block>-<chunk>)
  start       (x, y, facing) where the party enters
  water_walls wall values with the sea behind them (water cells are flooded from there)
  free_walls  free-standing walls stored one-sided in the GEO: also drawn from the far side
  floor, ceiling  painted flats
  walls       wall value -> description for the image model (what it IS, not its pixels)
  notrim      walls whose teal end trim is removed from the reference
  water       walls whose painted sea is removed from the reference
  tile        walls crossfaded at the edges instead of seamed
  noseam      walls a seam would cut through (objects, posts, gaps)
  arch        see-through openings: cut to the original's shape, a dark rim around the cutout
  clear       wall value -> boxes (x0, y0, x1, y1 in dump pixels) painted over with the wall colour in the
              reference: things the PoC draws elsewhere (Kuto's well hole and ladder: a prop, see cellprops)
  beds        wall value -> plant textures: a garden bed's wall, not drawn; plants stand over the bed's squares
  cut         wall value -> boxes (dump pixels) made transparent in the reference: water painted on the wall that
              the engine draws behind it instead (Stojanow's river railing: bank and river cut, railing kept)
  cellprops   (x, y) -> [(kind, texture, numbers...)]: map lines placed in the middle of that square:
              prop (size, front, sink) and well (half width, half depth, rim width, front), see host.go;
              front: that far ahead of the centre while the party stands on it (make_props.py)
  props       wall value -> billboard texture: trees drawn where their trunks stand, not as faces (make_props.py)
  doors       (x, y, side) fixes to the original's data: sides made passable (one-way doors)
  nomap       wall values left off the minimap (objects the party walks through, like graves)
  views       comparison shots (x, y, facing), reachable squares only"""

import hashlib
from pathlib import Path

AREAS = {}
DUMP = Path(__file__).resolve().parent / "viewdump"


def src(name, v):
    """viewdump name of wall value v of an area."""
    return f"{AREAS[name]['chunks'][(v - 1) // 5]}-wall{(v - 1) % 5 + 1}"


def _key(s):
    from PIL import Image
    return hashlib.sha1(Image.open(DUMP / f"{s}.png").convert("RGBA").tobytes()).digest()


def origin(name, v):
    """(area, value) of the first area here with the same original art as wall v of area name: its texture is
    reused, so a wall the game repeats in several areas is generated once and looks the same everywhere."""
    k = _key(src(name, v))
    for n, a in AREAS.items():
        for w in a["walls"]:
            if _key(src(n, w)) == k:
                return n, w
    return name, v

AREAS["phlan"] = dict(
    geo=(3, 0), walldefs=[(0, 1)], area_kind=11,
    chunks=["WALLDEF3-000-1", "WALLDEF3-000-2", "WALLDEF3-000-3"],
    start=(7, 7, "n"), water_walls={14, 15}, free_walls={14},
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim={1, 2, 3, 6, 7, 8, 9, 10}, water={14, 15}, tile={15}, noseam={14, 15},
    views=[(12, 13, "E"), (14, 3, "S")],
    walls={k: v[1] for k, v in {
    1: ("WALLDEF3-000-1-wall1", "a grey stone wall with a wide dark square tunnel opening in the middle "
        "leading into darkness, a small lit blue doorway far away at the end of the tunnel; the tunnel "
        "floor is empty."),
    3: ("WALLDEF3-000-1-wall3", "a plain smooth grey plastered stone town wall with a sturdy arched "
        "wooden door of vertical brown planks and a metal handle at the bottom centre."),
    5: ("WALLDEF3-000-1-wall5", 'a grey stone wall overgrown with climbing ivy and green vines that grow thickest towards the bottom and thin out towards the top, with a sturdy arched wooden door of vertical brown planks with a metal handle at the bottom centre.'),
    7: ("WALLDEF3-000-2-wall2", "a town house wall of white irregular fieldstones set in grey mortar "
        "with a sturdy arched wooden door of vertical brown planks and a metal handle at the bottom "
        "centre."),
    9: ("WALLDEF3-000-2-wall4", "a shop front in a town house wall of white irregular fieldstones: a "
        "red and white striped cloth awning across the whole width just above the middle, below it a "
        "wooden plank door on the left and a large shop window of blue glass panes (two rows of four) to "
        "its right."),
    10: ("WALLDEF3-000-2-wall5", "a town house wall of white irregular fieldstones with a large window "
         "of blue glass panes (two rows of four) on the left and an arched wooden plank door on the "
         "right, both in the lower half."),
    2: ("WALLDEF3-000-1-wall2", "a plain smooth grey plastered stone town wall, even and unadorned, the top row empty."),
    4: ("WALLDEF3-000-1-wall4", "a grey stone town wall overgrown with climbing ivy and green vines, "
        "dense near the bottom, thinner tendrils reaching up towards the top."),
    6: ("WALLDEF3-000-2-wall1", "a town house wall of white irregular fieldstones set in grey mortar, "
        "rounded flat stones in a dense even pattern over the whole face."),
    8: ("WALLDEF3-000-2-wall3", "a town house wall of white irregular fieldstones set in grey mortar, "
        "rounded flat stones in a dense even pattern over the whole face."),
    11: ("WALLDEF3-000-3-wall1", "the front of a grand civic hall: red brick wall with four tall "
         "pale cyan marble pilasters with capitals at the top, tall dark blue glass windows between "
         "them in the lower half, and broad grey stone steps along the bottom."),
    12: ("WALLDEF3-000-3-wall2", 'the front of a red brick town building in Phlan: an even wall of red bricks across the whole width, thin dark red vertical edges at the far left and right, in the lower middle a wide window of blue glass panes in a stone frame (two rows of three panes), and below it broad grey stone steps leading up to the building across the bottom.'),
    13: ("WALLDEF3-000-3-wall3", 'the facade of a temple: a tall pointed gothic gable of dark brown and maroon checkered brickwork with a round window near its peak, flanked left and right by slender yellow-green stone towers topped with small dark spires; at the bottom centre a pointed-arch doorway with a purple stone frame and a dark wooden door of vertical planks.'),
    14: ("WALLDEF3-000-3-wall4", "the edge of a harbour pier: a row of four tall, thick round wooden "
         "mooring stakes (weathered dark brown and reddish timber, dark shadow on their right side) "
         "standing in a row, a short stake stump between each pair at the bottom, and white hemp ropes "
         "hanging in shallow sagging curves from stake to stake at mid height. The water behind the "
         "stakes is not drawn: everything around and between the stakes and ropes is empty."),
    15: ("WALLDEF3-000-3-wall5", "a low harbour quay wall: white irregular fieldstones set in grey "
         "mortar across the bottom quarter only, its top a straight edge; above it nothing is drawn, the "
         "water behind it is not part of the wall."),
    }.items()},
)

AREAS["graveyard"] = dict(  # Valhingen Graveyard: ECL4 block 10
    geo=(4, 10), walldefs=[(21, 1), (20, 3)], area_kind=10,  # the ECL also asks for 22, which is missing (21 fills slots 1-2)
    chunks=["WALLDEF4-021-1", "WALLDEF4-021-2", "WALLDEF4-020-1"],
    doors={(5, 3, 3)},  # the cross between 5,3 and 4,3 is a door only from 4,3 in the original
    nomap={2, 3},  # crosses and headstones
    start=(0, 4, "e"), water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim={1, 4, 5, 6, 7, 9, 10, 14, 15}, water=set(), tile=set(), noseam={2, 3, 8, 11},
    views=[(1, 0, "S"), (13, 11, "W"), (15, 7, "W"), (0, 7, "S"), (6, 9, "S"), (14, 12, "W"), (15, 11, "E"),
           (8, 14, "S"), (5, 2, "S"), (1, 1, "E"), (15, 10, "W"), (10, 7, "E"), (2, 6, "W"), (7, 15, "W"),
           (11, 8, "N"), (1, 6, "N"), (13, 7, "E"), (4, 7, "E")],  # random.Random(10), reachable squares
    walls={
        1: "the graveyard's outer wall of grey stone with a wide dark round-arched gateway in the middle, a "
           "path leading through it into darkness towards a small lit blue doorway far away.",
        2: "a single weathered grey stone grave cross standing upright in tufts of green grass and weeds "
           "along the bottom; nothing else, everything around the cross and the grass is empty.",
        3: "a single weathered grey stone headstone with a rounded top standing in tufts of green grass and "
           "weeds along the bottom; nothing else, everything around the headstone and the grass is empty.",
        4: "a cemetery wall of rough mottled grey granite, dark and light speckled stone over the whole face.",
        5: "a wall of pale grey dressed stone blocks in regular courses with dark mortar joints.",
        6: "the wall of a mausoleum: smooth pale grey stone with four sealed burial niches (two rows of two), "
           "each a dark-framed stone plaque with a short purple inscription line.",
        7: "the wall of a stone crypt: pale grey stone with a crenellated parapet along the top and a large "
           "recessed plain stone panel below it.",
        8: "a tall wrought-iron cemetery fence: thin dark iron bars with spear-point finials joined by two "
           "horizontal rails; everything between and above the bars is empty.",
        9: "a wall of rough mottled grey granite with a dark round-arched opening in the middle, grey stone "
           "steps leading down into it towards a small glowing blue doorway at the bottom.",
        10: "a wall of rough mottled grey granite with a sturdy arched wooden door of vertical brown planks "
            "and a metal handle in the lower middle.",
        11: "a low heap of broken grey stone rubble and fallen blocks along the bottom only; above it nothing "
            "is drawn.",
        14: "a ruined grey stone wall, its top broken off in a jagged uneven line, a few loose blocks showing "
            "in the plaster; above the broken top nothing is drawn.",
        15: "a ruined grey stone wall, its top broken off in a jagged uneven line, with a sturdy arched wooden "
            "door of vertical brown planks in the lower middle; above the broken top nothing is drawn.",
    },
)

AREAS["slums"] = dict(  # the Slums of Phlan: ECL2 block 20
    geo=(2, 20), walldefs=[(2, 1), (4, 2), (1, 3)], area_kind=20,
    chunks=["WALLDEF2-002-1", "WALLDEF2-004-1", "WALLDEF2-001-1"],
    start=(14, 10, "n"), water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim={2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 13, 14, 15}, water=set(), tile=set(), noseam=set(), arch={13},
    views=[(4, 13, "S"), (8, 5, "W"), (3, 3, "N"), (10, 7, "W"), (5, 6, "W"), (0, 13, "N"), (13, 2, "E"),
           (13, 0, "E"), (2, 6, "N"), (3, 4, "E"), (4, 0, "N"), (10, 3, "N"), (15, 2, "E"), (14, 6, "S"),
           (6, 10, "S"), (6, 6, "S"), (10, 2, "S"), (10, 11, "E")],  # random.Random(20), reachable squares
    walls={
        2: "a shabby slum house wall of weathered vertical brown wooden planks with an arched wooden plank "
           "door with a metal handle on the left and a window of blue glass panes (two rows of four) on the "
           "right.",
        3: "a shabby slum house wall of weathered vertical brown wooden planks with a horizontal wooden beam "
           "across the upper part, a window of blue glass panes (two rows of four) on the left and an arched "
           "wooden plank door with a metal handle on the right.",
        4: "a plain shabby slum house wall of weathered vertical brown wooden planks, dark gaps between the "
           "boards.",
        5: "a shabby slum house wall of weathered vertical brown wooden planks with an arched wooden plank "
           "door with a metal handle in the lower middle.",
        6: "a small shop front in a wall of white irregular fieldstones: a red and white striped cloth awning "
           "across the whole width above the middle, below it a wooden plank door on the left and a window "
           "of blue glass panes (two rows of four) on the right.",
        7: "a town house wall of white irregular fieldstones set in grey mortar, rounded flat stones in a "
           "dense even pattern over the whole face.",
        8: "a town house wall of white irregular fieldstones with a window of blue glass panes (two rows of "
           "four) on the left and an arched wooden plank door with a metal handle on the right.",
        9: "a town house wall of white irregular fieldstones set in grey mortar, rounded flat stones in a "
           "dense even pattern over the whole face.",
        10: "a town house wall of white irregular fieldstones with an arched wooden plank door with a metal "
            "handle in the lower middle.",
        11: "a plain grey plastered stone wall with a wide dark round-arched tunnel opening in the middle, a "
            "small wooden door far away at the end of the tunnel; the tunnel floor is empty.",
        13: "a plain grey plastered stone wall with a tall open round-topped archway in the middle; the "
            "archway is an empty opening, nothing is drawn inside it.",
        14: "a plain smooth grey plastered stone town wall, even and unadorned.",
        15: "a plain smooth grey plastered stone town wall with an arched wooden plank door with a metal "
            "handle in the lower right.",
    },
)

AREAS["podol"] = dict(  # Podol Plaza: ECL1 block 18
    geo=(1, 18), walldefs=[(1, 1), (3, 2), (5, 3)], area_kind=18,
    chunks=["WALLDEF1-001-1", "WALLDEF1-003-1", "WALLDEF1-005-1"],
    start=(2, 10, "n"), water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(range(1, 16)), water=set(), tile=set(), noseam=set(), arch={3, 8},
    views=[(2, 14, "N"), (1, 15, "S"), (10, 9, "E"), (7, 2, "E"), (5, 5, "E"), (3, 13, "E"), (3, 2, "E"),
           (15, 4, "S"), (7, 13, "E"), (10, 0, "W"), (7, 14, "E"), (7, 10, "S"), (4, 11, "S"), (7, 5, "N"),
           (14, 4, "S"), (4, 3, "W"), (4, 0, "E"), (14, 7, "W")],  # random.Random(18), reachable squares
    walls={
        1: "a plain grey plastered stone wall with a wide dark round-arched tunnel opening in the middle, a "
           "small wooden door far away at the end of the tunnel; the tunnel floor is empty.",
        3: "a plain grey plastered stone wall with a tall open round-topped archway in the middle; the "
           "archway is an empty opening, nothing is drawn inside it.",
        4: "a plain smooth grey plastered stone town wall, even and unadorned.",
        5: "a plain smooth grey plastered stone town wall with an arched wooden plank door with a metal "
           "handle in the lower middle.",
        6: "a wall of rough grey speckled granite blocks, fine dark and light grain over the whole face.",
        7: "a wall of rough grey speckled granite with an arched wooden plank door with a metal handle in "
           "the lower middle.",
        8: "a wall of rough grey speckled granite with a tall open round-topped archway in the middle; the "
           "archway is an empty opening, nothing is drawn inside it.",
        9: "a grey plastered stone town wall overgrown with climbing ivy and green vines over most of its "
           "face, thicker towards the bottom.",
        10: "a grey plastered stone town wall overgrown with climbing ivy and green vines, with an arched "
            "wooden plank door with a metal handle in the lower right.",
        11: "the facade of a dark temple: a tall pointed gable of dark brown and maroon checkered brickwork "
            "with a round window near its peak, flanked left and right by slender yellow-green stone towers "
            "topped with small dark spires; at the bottom centre a pointed-arch doorway with a purple stone "
            "frame and a dark wooden door of vertical planks.",
        12: "the side wall of a dark temple: dark brown and maroon checkered brickwork between two slender "
            "yellow-green stone towers topped with small dark spires.",
        13: "the upper front of a dark temple: dark brown and maroon checkered brickwork with a pointed gothic "
            "window holding a small purple sigil, a stone balcony with a purple ornamental railing across "
            "the middle, slender yellow-green pilasters at both sides, a plain grey stone base at the bottom.",
        14: "a low fence of vertical brown wooden planks across the lower half only, a dark gap along the "
            "ground under it; above the fence nothing is drawn.",
        15: "a wall of dark mottled stone, deep blue, dark red and black speckles over the whole face.",
    },
)

AREAS["temple"] = dict(  # Temple of Bane, level 1: ECL1 block 24
    geo=(1, 24), walldefs=[(1, 1), (3, 2), (5, 3)], area_kind=24,
    chunks=['WALLDEF1-001-1', 'WALLDEF1-003-1', 'WALLDEF1-005-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(1, 0, 'W'), (1, 4, 'W'), (8, 2, 'N'), (0, 4, 'E'), (0, 11, 'E'), (4, 4, 'N'), (13, 12, 'E'), (12, 10, 'S'), (8, 14, 'E'), (2, 5, 'N'), (12, 5, 'W'), (11, 11, 'E'), (11, 14, 'N'), (15, 1, 'E'), (1, 12, 'E'), (10, 6, 'E')],  # random, reachable squares,
    walls={},
)

AREAS["temple2"] = dict(  # Temple of Bane, level 2: ECL1 block 24
    geo=(1, 31), walldefs=[(1, 1), (3, 2), (5, 3)], area_kind=24,
    chunks=['WALLDEF1-001-1', 'WALLDEF1-003-1', 'WALLDEF1-005-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(0, 12, 'S'), (15, 13, 'S'), (14, 2, 'E'), (12, 1, 'W'), (14, 6, 'N'), (3, 4, 'E'), (3, 7, 'E'), (12, 8, 'S'), (6, 10, 'S'), (11, 10, 'W'), (11, 9, 'W'), (13, 9, 'W'), (15, 6, 'W'), (6, 12, 'N'), (7, 13, 'N'), (13, 8, 'S')],  # random, reachable squares,
    walls={},
)

AREAS["stojanow"] = dict(  # Stojanow Gate: ECL2 block 9
    geo=(2, 9), walldefs=[(1, 1), (9, 2), (23, 3)], area_kind=9,
    chunks=['WALLDEF2-001-1', 'WALLDEF2-009-1', 'WALLDEF2-023-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(5, 2, 'S'), (12, 5, 'S'), (12, 2, 'W'), (7, 11, 'S'), (9, 0, 'E'), (1, 2, 'E'), (11, 3, 'S'), (2, 10, 'W'), (13, 2, 'S'), (12, 4, 'S'), (10, 1, 'E'), (10, 14, 'W'), (3, 12, 'S'), (9, 14, 'E'), (2, 13, 'N'), (9, 12, 'N')],  # random, reachable squares,
    walls={},
)

AREAS["library"] = dict(  # Mendor's Library: ECL2 block 15
    geo=(2, 15), walldefs=[(1, 1), (19, 2), (23, 3)], area_kind=15,
    chunks=['WALLDEF2-001-1', 'WALLDEF2-019-1', 'WALLDEF2-023-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(7, 8, 'N'), (15, 2, 'E'), (2, 1, 'W'), (14, 14, 'E'), (2, 11, 'N'), (9, 9, 'S'), (1, 12, 'S'), (7, 13, 'S'), (11, 13, 'S'), (5, 12, 'E'), (9, 6, 'N'), (11, 7, 'S'), (11, 4, 'N'), (9, 5, 'N'), (3, 5, 'E'), (15, 15, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["textile"] = dict(  # Cadorna Textile House: ECL3 block 14
    geo=(3, 14), walldefs=[(1, 1), (3, 2), (5, 3)], area_kind=14,
    chunks=['WALLDEF3-001-1', 'WALLDEF3-003-1', 'WALLDEF3-005-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(3, 14, 'W'), (4, 15, 'N'), (3, 0, 'W'), (8, 9, 'E'), (14, 15, 'S'), (5, 9, 'S'), (4, 9, 'N'), (7, 7, 'E'), (5, 13, 'W'), (13, 8, 'N'), (1, 14, 'S'), (15, 7, 'E'), (3, 4, 'E'), (9, 7, 'E'), (9, 0, 'N'), (13, 13, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["hobgoblin"] = dict(  # Hobgoblin courtyard and barracks: ECL4 block 2
    geo=(4, 2), walldefs=[(1, 1), (3, 2), (20, 3)], area_kind=2,
    chunks=['WALLDEF4-001-1', 'WALLDEF4-003-1', 'WALLDEF4-020-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(7, 2, 'N'), (7, 14, 'E'), (11, 13, 'E'), (1, 10, 'E'), (8, 12, 'E'), (13, 12, 'S'), (8, 2, 'N'), (7, 10, 'S'), (13, 8, 'S'), (11, 8, 'S'), (3, 10, 'S'), (15, 12, 'N'), (15, 13, 'N'), (12, 5, 'N'), (8, 7, 'S'), (2, 15, 'E')],  # random, reachable squares,
    walls={},
)

AREAS["sokal"] = dict(  # Sokal Keep: ECL4 block 21
    geo=(4, 21), walldefs=[(1, 1), (5, 2), (9, 3)], area_kind=21,
    chunks=['WALLDEF4-001-1', 'WALLDEF4-005-1', 'WALLDEF4-009-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(7, 4, 'N'), (13, 0, 'W'), (11, 3, 'E'), (10, 8, 'W'), (15, 10, 'W'), (3, 11, 'N'), (6, 6, 'S'), (0, 7, 'N'), (9, 11, 'W'), (3, 12, 'W'), (2, 3, 'E'), (4, 14, 'N'), (13, 5, 'S'), (2, 10, 'E'), (1, 6, 'E'), (12, 0, 'S')],  # random, reachable squares,
    walls={},
)

AREAS["valjevo3"] = dict(  # Valjevo Castle (ECL5 3): ECL5 block 3
    geo=(5, 3), walldefs=[(24, 1), (1, 3)], area_kind=3,
    chunks=['WALLDEF5-024-1', 'WALLDEF5-024-2', 'WALLDEF5-001-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(10, 4, 'S'), (8, 3, 'N'), (4, 13, 'E'), (13, 1, 'W'), (11, 2, 'S'), (4, 12, 'W'), (8, 6, 'N'), (7, 12, 'E'), (6, 14, 'S'), (4, 7, 'E'), (0, 5, 'E'), (10, 15, 'E'), (6, 4, 'W'), (10, 1, 'E'), (3, 5, 'N'), (9, 6, 'E')],  # random, reachable squares,
    walls={},
)

AREAS["valjevo4"] = dict(  # Valjevo Castle (ECL5 4): ECL5 block 4
    geo=(5, 4), walldefs=[(24, 1), (1, 3)], area_kind=4,
    chunks=['WALLDEF5-024-1', 'WALLDEF5-024-2', 'WALLDEF5-001-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(15, 6, 'N'), (14, 11, 'S'), (12, 8, 'S'), (8, 1, 'E'), (13, 7, 'E'), (7, 2, 'N'), (13, 10, 'E'), (13, 9, 'E'), (3, 0, 'N'), (12, 7, 'N'), (13, 0, 'W'), (8, 0, 'N'), (2, 1, 'E'), (13, 4, 'S'), (15, 1, 'W'), (12, 10, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["valjevo6"] = dict(  # Valjevo Castle (ECL5 6): ECL5 block 6
    geo=(5, 6), walldefs=[(24, 1), (1, 3)], area_kind=6,
    chunks=['WALLDEF5-024-1', 'WALLDEF5-024-2', 'WALLDEF5-001-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(8, 12, 'W'), (6, 12, 'S'), (2, 7, 'W'), (0, 5, 'W'), (1, 6, 'E'), (8, 13, 'E'), (8, 14, 'N'), (1, 11, 'W'), (1, 13, 'W'), (1, 5, 'N'), (11, 15, 'S'), (0, 1, 'S'), (10, 15, 'S'), (0, 6, 'E'), (5, 15, 'S'), (2, 12, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["valjevo7"] = dict(  # Valjevo Castle (ECL5 7): ECL5 block 7
    geo=(5, 7), walldefs=[(24, 1), (1, 3)], area_kind=7,
    chunks=['WALLDEF5-024-1', 'WALLDEF5-024-2', 'WALLDEF5-001-1'],
    start=(5, 7, "n"), water_walls=set(), free_walls=set(),  # entry: ECL5 5 opNewECL 7 at 5,7
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(12, 2, 'N'), (14, 5, 'W'), (6, 14, 'S'), (14, 1, 'S'), (9, 7, 'N'), (1, 3, 'S'), (10, 8, 'N'), (9, 9, 'S'), (13, 9, 'E'), (13, 11, 'E'), (0, 6, 'N'), (15, 6, 'W'), (4, 8, 'S'), (12, 0, 'N'), (1, 12, 'W'), (3, 10, 'S')],  # random, reachable squares,
    walls={},
)

AREAS["buccaneer"] = dict(  # Buccaneer Base: ECL6 block 1
    geo=(6, 1), walldefs=[(2, 1), (6, 2), (6, 3)], area_kind=1,
    chunks=['WALLDEF6-002-1', 'WALLDEF6-006-1', 'WALLDEF6-006-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(5, 4, 'W'), (6, 14, 'W'), (9, 12, 'S'), (4, 14, 'W'), (8, 3, 'E'), (8, 8, 'S'), (11, 12, 'W'), (4, 8, 'N'), (12, 3, 'S'), (13, 9, 'E'), (8, 9, 'W'), (10, 13, 'W'), (5, 8, 'N'), (4, 11, 'W'), (12, 13, 'N'), (9, 4, 'S')],  # random, reachable squares,
    walls={},
)

AREAS["zhentil"] = dict(  # Zhentil Outpost: ECL6 block 28
    geo=(6, 28), walldefs=[(1, 1), (2, 2), (9, 3)], area_kind=28,
    chunks=['WALLDEF6-001-1', 'WALLDEF6-002-1', 'WALLDEF6-009-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(12, 14, 'N'), (1, 13, 'E'), (13, 14, 'S'), (1, 11, 'S'), (8, 3, 'E'), (9, 3, 'S'), (7, 3, 'N'), (0, 11, 'W'), (13, 3, 'E'), (9, 1, 'E'), (7, 2, 'E'), (1, 1, 'S'), (5, 13, 'E'), (2, 7, 'N'), (12, 0, 'E'), (14, 7, 'S')],  # random, reachable squares,
    walls={},
)

AREAS["nomad"] = dict(  # Nomad Camp: ECL7 block 17
    geo=(7, 17), walldefs=[(7, 1), (8, 2), (23, 3)], area_kind=17,
    chunks=['WALLDEF7-007-1', 'WALLDEF7-008-1', 'WALLDEF7-023-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(11, 5, 'W'), (14, 12, 'E'), (5, 12, 'E'), (15, 14, 'S'), (3, 5, 'N'), (3, 7, 'E'), (0, 13, 'E'), (6, 12, 'W'), (14, 14, 'W'), (13, 15, 'N'), (4, 0, 'E'), (2, 4, 'N'), (10, 15, 'S'), (5, 3, 'S'), (4, 8, 'W'), (9, 12, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["swamp"] = dict(  # Lizard Man swamp: ECL7 block 22
    geo=(7, 22), walldefs=[(1, 1), (3, 2)], area_kind=22,
    chunks=['WALLDEF7-001-1', 'WALLDEF7-003-1', None],
    start=(15, 7, "e"), water_walls=set(), free_walls=set(),  # entry: ECL7 23 opNewECL 22 at 15,7 E
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(14, 12, 'S'), (4, 11, 'E'), (11, 9, 'N'), (7, 11, 'N'), (13, 8, 'N'), (1, 12, 'S'), (7, 8, 'E'), (2, 8, 'S'), (2, 11, 'S'), (12, 14, 'W'), (3, 15, 'N'), (6, 14, 'E'), (11, 8, 'E'), (1, 14, 'E'), (11, 12, 'W'), (5, 15, 'S')],  # random, reachable squares,
    walls={},
)

AREAS["lizvillage"] = dict(  # Lizard Man village: ECL7 block 23
    geo=(7, 23), walldefs=[(1, 1), (3, 2)], area_kind=23,
    chunks=['WALLDEF7-001-1', 'WALLDEF7-003-1', None],
    start=(4, 15, "n"), water_walls=set(), free_walls=set(),  # entry: ECL7 22 opNewECL 23 at 4,15 N
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(5, 7, 'N'), (4, 7, 'W'), (1, 13, 'N'), (13, 11, 'W'), (10, 12, 'S'), (1, 11, 'N'), (15, 4, 'E'), (11, 2, 'S'), (3, 1, 'E'), (4, 2, 'N'), (0, 12, 'S'), (3, 3, 'N'), (3, 11, 'W'), (4, 8, 'N'), (12, 1, 'W'), (3, 14, 'E')],  # random, reachable squares,
    walls={},
)

AREAS["kobold"] = dict(  # Kobold Caves: ECL8 block 13
    geo=(8, 13), walldefs=[(17, 1), (18, 2)], area_kind=13,
    chunks=['WALLDEF8-017-1', 'WALLDEF8-018-1', None],
    start=(6, 15, "n"), water_walls=set(), free_walls=set(),  # entry: ECL8 27 opNewECL 13 at 6,15 N
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(4, 7, 'E'), (5, 2, 'N'), (15, 3, 'S'), (2, 9, 'E'), (1, 8, 'N'), (0, 10, 'E'), (9, 0, 'S'), (0, 5, 'W'), (7, 3, 'W'), (5, 4, 'S'), (15, 10, 'W'), (11, 3, 'S'), (3, 8, 'E'), (15, 12, 'W'), (11, 8, 'S'), (1, 12, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["pyramid"] = dict(  # Yarash's Pyramid, level 1: ECL8 block 16
    geo=(8, 16), walldefs=[(9, 1), (20, 2), (23, 3)], area_kind=16,
    chunks=['WALLDEF8-009-1', 'WALLDEF8-020-1', 'WALLDEF8-023-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(15, 8, 'S'), (9, 1, 'W'), (12, 6, 'E'), (12, 1, 'S'), (4, 13, 'W'), (14, 7, 'S'), (5, 5, 'E'), (15, 14, 'W'), (4, 2, 'W'), (6, 7, 'S'), (13, 3, 'W'), (12, 3, 'E'), (3, 6, 'S'), (3, 13, 'E'), (15, 15, 'N'), (9, 14, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["pyramid2"] = dict(  # Yarash's Pyramid, level 2: ECL8 block 16
    geo=(8, 30), walldefs=[(9, 1), (17, 2), (23, 3)], area_kind=16,
    chunks=['WALLDEF8-009-1', 'WALLDEF8-017-1', 'WALLDEF8-023-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(14, 14, 'W'), (1, 4, 'W'), (5, 1, 'S'), (10, 6, 'N'), (7, 0, 'W'), (1, 11, 'S'), (6, 6, 'N'), (1, 2, 'E'), (6, 2, 'W'), (8, 2, 'S'), (2, 12, 'W'), (10, 15, 'W'), (10, 11, 'N'), (6, 0, 'W'), (10, 8, 'E'), (3, 10, 'E')],  # random, reachable squares,
    walls={},
)

AREAS["ruined"] = dict(  # Ruined castle in the quagmire: ECL8 block 27
    geo=(8, 27), walldefs=[(17, 1)], area_kind=27,
    chunks=['WALLDEF8-017-1', None, None],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(6, 12, 'E'), (5, 7, 'W'), (4, 10, 'W'), (1, 12, 'W'), (0, 13, 'N'), (2, 13, 'W'), (3, 11, 'S'), (0, 15, 'W'), (2, 10, 'S'), (5, 12, 'N'), (3, 13, 'W'), (0, 9, 'W'), (7, 14, 'S'), (3, 7, 'N'), (1, 9, 'N'), (2, 8, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["kuto"] = dict(  # Kuto's Well, surface: ECL8 block 29
    geo=(8, 29), walldefs=[(3, 1), (20, 2), (1, 3)], area_kind=29,
    chunks=['WALLDEF8-003-1', 'WALLDEF8-020-1', 'WALLDEF8-001-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(14, 5, 'W'), (0, 8, 'E'), (10, 7, 'W'), (6, 13, 'W'), (13, 8, 'N'), (10, 12, 'E'), (12, 14, 'N'), (13, 1, 'N'), (7, 11, 'W'), (0, 9, 'W'), (3, 9, 'S'), (2, 10, 'S'), (11, 5, 'N'), (11, 7, 'E'), (7, 7, 'N'), (4, 10, 'W')],  # random, reachable squares,
    walls={},
)

AREAS["kuto2"] = dict(  # Kuto's Well, below: ECL8 block 29
    geo=(8, 32), walldefs=[(18, 1), (17, 2), (1, 3)], area_kind=29,
    chunks=['WALLDEF8-018-1', 'WALLDEF8-017-1', 'WALLDEF8-001-1'],
    start=None, water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim=set(), water=set(), tile=set(), noseam=set(),
    views=[(1, 1, 'S'), (15, 6, 'E'), (8, 13, 'E'), (14, 15, 'E'), (0, 12, 'S'), (15, 10, 'S'), (14, 6, 'W'), (0, 5, 'E'), (4, 2, 'S'), (15, 14, 'S'), (9, 15, 'S'), (0, 8, 'W'), (12, 6, 'W'), (14, 1, 'W'), (5, 3, 'E'), (12, 0, 'W')],  # random, reachable squares,
    walls={},
)

# The walls first drawn in the areas above (every other value repeats an earlier area's art, areas.origin).
GATE = ("a crenellated grey stone town wall: a dark blue and black checkered parapet band along the top edge, "
        "below it ")
NEW = {
    "stojanow": {
        6: GATE + "a big double gate of vertical brown wooden planks filling the lower part, iron hinges and "
           "two small iron ring handles where the two wings meet in the middle.",
        7: GATE + "smooth grey plastered stone split by a second checkered dark blue and black stone band "
           "running straight across the whole width at one third of the height, plain grey plaster above and "
           "below it.",
        8: GATE + "a tall dark opening in the middle with grey stone steps rising inside it towards a small "
           "double wooden door far away at the top.",
        9: GATE + "plain smooth grey plastered stone, even and unadorned.",
        12: "a low wooden bridge railing across the bottom quarter only: a brown top beam and a row of short "
            "square brown wooden posts with open gaps between them; the gaps are empty, and above the railing "
            "nothing is drawn (the river behind is drawn by the game).",
    },
    "library": {
        6: "the front of a stately red brick library: a stepped brick gable at the top with a small grey vent, "
           "two small square windows of blue glass in grey stone frames, slender pale cyan marble columns at "
           "the left and right edges, a double door of dark vertical wooden planks under a grey stone lintel "
           "in the lower middle, and grey stone steps across the bottom.",
        7: "a plain wall of red bricks in even courses with fine light mortar, the whole face.",
        8: "a tall wooden library bookcase filling the whole face: a brown wooden cornice at the top, three "
           "shelves full of old leather-bound books in red, green, blue and white with gilded spines.",
        9: "a tall wooden library bookcase filling the whole face: a brown wooden cornice at the top, three "
           "shelves full of old leather-bound books in red, green, blue and white with gilded spines.",
        10: "a plain wall of red bricks in even courses with an arched wooden plank door with a metal "
            "handle in the lower right.",
        11: "a small round garden pond of clear blue water lying on brown earth, seen low along the bottom "
            "quarter only, a few grass blades along its rim; above it nothing is drawn.",
        13: "a few overgrown mounds of brown earth with green grass and weeds, irregular blocky heaps of "
            "different heights with gaps between them; everything around the mounds is empty.",
    },
    "hobgoblin": {
        12: "a rough grey stone barracks wall with a dark blue and black checkered parapet along the top, a "
            "wide open rectangular window opening in the upper half with a small dented metal well bucket of "
            "riveted iron, rust-red inside, hanging by its curved handle from a short rope in the middle, and a few loose pale grey bricks showing in the plaster below; the window "
            "opening is empty, nothing is drawn inside it.",
    },
    "sokal": {
        2: "a stone fountain against a plain grey plastered wall: a slender white marble fountain column in "
           "the middle spouting pale water into a wide oval basin of deep blue water at the bottom.",
        15: GATE + "smooth grey plastered stone with an arched wooden plank door with a metal handle in "
            "the lower middle.",
    },
    "valjevo3": {
        1: "a castle portcullis: a white fieldstone lintel across the top, below it a heavy grid of dark iron "
           "bars and vertical brown wooden beams filling the whole face.",
        2: "a castle wall of white irregular fieldstones in grey mortar with a dark square opening in the "
           "middle, grey stone steps leading down inside it towards a small glowing blue arched doorway.",
        3: "a castle wall of white irregular fieldstones set in grey mortar, rounded flat stones in a dense "
           "even pattern over the whole face.",
        4: "a castle wall of white irregular fieldstones with an arched wooden plank door with a metal "
           "handle in the lower middle.",
        7: "a tall dense castle garden hedge of dark green leaves with small yellow roses and thin brown "
           "twigs, its top ragged and uneven; above the hedge nothing is drawn.",
        8: "the cut end of a tall dense garden hedge with small yellow roses: a narrow strip of hedge at the "
           "far left edge only; everything else is empty.",
        9: "a tall dense castle garden hedge of dark green leaves with small yellow roses, with a narrow gap "
           "near the left: a thin strip of hedge at the far left, an empty opening, then hedge over the "
           "rest; the gap is empty, nothing is drawn in it.",
    },
    "valjevo4": {
        5: "a ruined castle wall of white irregular fieldstones, its top broken off in a jagged uneven line "
           "with a few brown loose stones; above the broken top nothing is drawn.",
    },
    "valjevo7": {
        6: "a castle study wall of white fieldstones above a tall dark bookcase: a wooden shelf with a stack "
           "of books lying flat, tall books standing in red, white, blue and green, and a brown wooden "
           "cabinet base of vertical boards across the bottom.",
        10: "a small oval pond of clear blue water lying on brown earth, seen low along the bottom quarter "
            "only; above it nothing is drawn.",
    },
    "buccaneer": {
        6: "a pirate stockade: a row of tall red-brown tree-trunk logs with sharpened pointed tops standing "
           "side by side, lashed together with dark ropes at three heights.",
        7: "a pirate stockade of tall red-brown sharpened logs with a narrow log gate in the middle, two "
           "pale iron rings where the gate wings meet.",
        8: "a tall dark wooden fence: vertical brown planks with small pointed tops, held by grey weathered "
           "horizontal rails at regular heights.",
        9: "a tall dark wooden fence of vertical brown planks and grey horizontal rails with a narrow gate "
           "in the middle marked by two thin iron hinges.",
        10: "a picket fence of thin vertical brown wooden stakes with pointed tops and two thin dark rails, "
            "three quarters as tall as a wall; above the fence and between the stakes nothing is drawn.",
    },
    "nomad": {
        1: "the side of a large nomad hut: a thick thatched roof of dry yellow straw and reeds across the "
           "upper part, below it a wall of vertical yellow bamboo poles bound with thin red cords.",
        2: "the front of a large nomad hut: a thick thatched roof of dry yellow straw across the upper part, "
           "below it a wall of vertical yellow bamboo poles with a wide doorway in the middle: a pitch black "
           "opening with a ragged straw edge at its top, no door, no curtain, only darkness inside.",
        6: "a single tall leafy green tree at the far left: a full rounded crown of dense green leaves with "
           "a few red berries, a gnarled dark brown trunk at the bottom; everything right of the tree is "
           "empty.",
        7: "the left half of a tall leafy green tree at the far right edge: dense green leaves and a brown "
           "trunk, cut off by the right edge; everything left of it is empty.",
        8: "two tall leafy green trees, one at each edge: the right half of a tree at the far left edge and "
           "the left half of a tree at the far right edge, dense green crowns and brown trunks; the "
           "middle between them is empty.",
    },
    "kobold": {
        1: "a natural cave wall of dark red rock with long hanging stalactites and dripstone ribs, deep black "
           "shadows between them, over the whole face.",
        2: "a natural cave wall of dark red rock with long hanging stalactites and dripstone ribs, deep black "
           "shadows between them, over the whole face.",
        3: "a natural cave wall of dark red rock with a tall wide opening in the middle, ragged stalactites "
           "hanging into its top; the opening is empty, nothing is drawn inside it.",
        4: "a natural cave wall of dark red rock with a tunnel in the middle: a brown dirt path running away "
           "into the darkness towards a small dark arched doorway far away.",
        6: "a natural cave wall of dark red and black rock with dripstone ribs, darker than the rest.",
        8: "a rough wall of irregular grey and brown rubble stones packed tightly, over the whole face.",
        9: "a natural cave wall of dark red rock with an arched wooden plank door with a metal handle in "
           "the lower middle.",
        10: "a rough wall of irregular grey and brown rubble stones with an arched wooden plank door with a "
            "metal handle in the lower middle.",
    },
    "pyramid": {
        14: "a low patch of green grass and weeds on brown earth along the bottom half only; above it "
            "nothing is drawn.",
    },
    "kuto": {
        8: "a rough grey stone wall with an open rectangular window opening in the upper half with a small "
           "dented metal well bucket of riveted iron, partly filled with water and rust-red inside, hanging "
           "by its curved handle from a short rope in the middle of the opening, and loose pale grey bricks "
           "showing in the plaster below; the window opening is empty, nothing is drawn inside it besides "
           "the bucket: no bottle neck above it, no shelf, plank or sill under it.",
    },
    "kuto2": {
        2: "a natural cave wall of dark red rock with a long iron ladder climbing straight up the middle "
           "towards the light.",
        10: "a natural cave wall of dark red rock with a heavy iron-banded door of dark red vertical planks "
            "in the lower middle.",
    },
}
FLAGS = dict(  # texture flags of the new walls (by area)
    stojanow=dict(noseam={12}, water_walls={12}, cut={12: [(0, 0, 224, 192)]}), library=dict(noseam={8, 9, 11, 13}, pond={11}, water_walls={12},
                                                          beds={13: ["prop_grass1", "prop_grass2"]}),
    hobgoblin=dict(arch={12}), sokal=dict(noseam={2}), valjevo3=dict(noseam={1, 7, 8, 9}),
    valjevo4=dict(), valjevo7=dict(noseam={6, 10}, pond={10}), buccaneer=dict(noseam={6, 7, 8, 9, 10}),
    nomad=dict(props={6: "prop_nomad_tree", 7: "prop_nomad_tree", 8: "prop_nomad_tree"}, water_walls={12}), kobold=dict(tile={1, 2, 6}, noseam={3, 4, 9}),
    pyramid=dict(noseam={14}), kuto=dict(arch={8}, clear={8: [(72, 128, 150, 256), (30, 232, 194, 256)]},
                     cellprops={(7, 7): [("well", "floor_kuto_rim", 0.3, 0.12, 0.07, 0.37),
                                         ("prop", "prop_kuto_ladder", 0.65, 0.42, 0.08)]}), kuto2=dict(noseam={2, 10}),
)


def used(name):
    """Wall values the area's GEO block draws."""
    g = (DUMP.parent / "geo{}_{}.bin".format(*AREAS[name]["geo"])).read_bytes()[:0x200]
    return sorted(({b >> 4 for b in g} | {b & 15 for b in g}) - {0})


for _n, _a in AREAS.items():
    if _a["walls"]:
        continue  # described by hand above
    _a["walls"] = {v: NEW.get(_n, {}).get(v, "") for v in used(_n)}  # "": repeats an earlier area's wall
    _a["notrim"] = set(_a["walls"])
    for _k, _s in FLAGS.get(_n, {}).items():
        _a[_k] = _s

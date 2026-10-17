"""Per-area settings for the remaster pipeline (make_map.py, make_textures.py,
compare/compare.py, see PROCESS.md). Each script takes the area name as its
first argument (default phlan).

Keys:
  geo         (disk, block) of GEO<disk>.DAX; the map is geo<disk>_<block>.bin
  walldefs    (block, slot) loads in the order the area's ECL does (opLoadFiles 37)
  area_kind   ECL block number, DS:84DC: 0 and 10 leave off-map squares empty, others wrap
  chunks      viewdump name of wall values 1-5, 6-10, 11-15 (WALLDEF<disk>-<block>-<chunk>)
  chars       wall value -> map character (unique across all areas: walls.txt is shared)
  start       (x, y, facing) where the party enters
  water_walls wall values with the sea behind them (water cells are flooded from there)
  free_walls  free-standing walls stored one-sided in the GEO: also drawn from the far side
  floor, ceiling  painted flats
  walls       wall value -> description for the image model (what it IS, not its pixels)
  notrim      walls whose teal end trim is removed from the reference
  water       walls whose painted sea is removed from the reference
  tile        walls crossfaded at the edges instead of seamed
  noseam      walls a seam would cut through (objects, posts, gaps)
  views       comparison shots (x, y, facing), reachable squares only"""

AREAS = {}

AREAS["phlan"] = dict(
    geo=(3, 0), walldefs=[(0, 1)], area_kind=11,
    chunks=["WALLDEF3-000-1", "WALLDEF3-000-2", "WALLDEF3-000-3"],
    chars={1: "T", 3: "U", 5: "V", 7: "X", 9: "Y", 10: "Z", 2: "H", 4: "J", 6: "K", 8: "N", 11: "O", 12: "P",
           13: "Q", 14: "R", 15: "S"},
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
    chars={v: "abcdefghijklmno"[v - 1] for v in range(1, 16)},
    start=(0, 4, "e"), water_walls=set(), free_walls=set(),
    floor="phlan_dirt", ceiling="sky_phlan",
    notrim={1, 4, 5, 6, 7, 9, 10, 14, 15}, water=set(), tile=set(), noseam={2, 3, 8, 11, 14},
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

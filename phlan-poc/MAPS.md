# 3D maps of Pool of Radiance

Every walkable 3D map in the game: the area scripts (ECL) that load a GEO
block with `21 opLoadFiles GEO 2 255`, with the wall sets they load
(`37 opLoadFiles A B C`, WALLDEF<disk> blocks for slots 1-3). Found by
scanning every ECL block (`go run ./cmd/ecl DISK BLOCK`); names come from
the scripts' own text. A tick means the map is in `areas.py`, remastered
and compared (PROCESS.md).

| done | area | ECL | GEO block(s) | wall sets |
|:---:|------|-----|-------------|-----------|
| [x] | New Phlan (civilized district, incl. City Hall) | 3/0, 3/8, 3/11 | 3/0 | 3: 0 |
| [x] | Valhingen Graveyard | 4/10 | 4/10 | 4: 21, (22 missing), 20 |
| [x] | Slums | 2/20 | 2/20 | 2: 2, 4, 1 |
| [x] | Podol Plaza | 1/18 | 1/18 | 1: 1, 3, 5 |
| [ ] | Temple of Bane (orc hall, black hands) | 1/24 | 1/24, 1/31 | 1: 1, 3, 5 |
| [ ] | Stojanow Gate (gate and towers) | 2/9 | 2/9 | 2: 1, 9, 23 |
| [ ] | Mendor's Library | 2/15 | 2/15 | 2: 1, 19, 23 |
| [ ] | Cadorna Textile House (thieves, traps) | 3/14 | 3/14 | 3: 1, 3, 5 |
| [ ] | Hobgoblin courtyard and barracks | 4/2 | 4/2 | 4: 1, 3, 20 |
| [ ] | Sokal Keep | 4/21 | 4/21 (+ 4/16, 4/2 variants) | 4: 1, 5, 9 |
| [ ] | Valjevo Castle (Tyranthraxus) | 5/3, 5/4, 5/5, 5/6, 5/7 | 5/3, 5/4, 5/6, 5/7 | 5: 24, 25, 1 |
| [ ] | Buccaneer Base | 6/1 | 6/1 | 6: 2, 6, 6 |
| [ ] | Zhentil Outpost | 6/28 | 6/28 | 6: 1, 2, 9 |
| [ ] | Nomad Camp | 7/17 | 7/17 | 7: 7, 8, 23 |
| [ ] | Lizard Man swamp | 7/22 | 7/22 | 7: 1, 3 |
| [ ] | Lizard Man village (Yarash's enemies) | 7/23 | 7/23 | 7: 1, 3 |
| [ ] | Kobold Caves | 8/13 | 8/13 | 8: 17, 18 |
| [ ] | Yarash's Pyramid (two levels) | 8/16 | 8/16, 8/30 | 8: 9, 20, 23 / 9, 17, 23 |
| [ ] | Ruined castle in the quagmire (kobolds) | 8/27 | 8/27 | 8: 17 |
| [ ] | Kuto's Well (surface and below) | 8/29 | 8/29, 8/32 | 8: 3, 20, 1 / 18, 17, 1 |

Not counted: the wilderness scripts (6/25, 7/26, 8/27's outer part) load
GEO 4/5/6 with `4 4 0`, the overland travel map, and 6/19 (a cave) is text
only. Names marked by a description instead of a proper name could not be
pinned from the script text alone.

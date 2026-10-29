#!/usr/bin/env python3
"""Convert The Dark Queen of Krynn's GAME.FON (96 glyphs of 8x12 for
characters 0x20..0x7F, letters on rows 0-9) to a raw .f8 file (256 glyphs
x 8 rows, bit 7 = left pixel) for the font cycle (engine/fonts.go loads
data/assets/fonts/*.f8). Rows 2, 5, 7 and 11 are dropped: the
letters keep 7 rows and a blank one, descenders land on row 7.

    dqfon2f8.py GAME.FON OUT.f8
"""
import sys

ROWS = [0, 1, 3, 4, 6, 8, 9, 10]

src = open(sys.argv[1], 'rb').read()
if len(src) != 96 * 12:
    sys.exit('not a Dark Queen GAME.FON (96 glyphs of 12 rows)')
out = bytearray(256 * 8)
for i in range(96):
    for r, s in enumerate(ROWS):
        out[(0x20 + i) * 8 + r] = src[i * 12 + s]
open(sys.argv[2], 'wb').write(out)

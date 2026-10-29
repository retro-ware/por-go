package engine

// pageSet mirrors a write to the original's drawing page DS:4A20 (shown at
// screen pixel 8,8; wider in combat), so sprites can restore what they hid.
// It keeps the whole pixel: Amiga pixels (pal.AmigaBase up) lost their
// palette slot to & 15, and the restored floor came back EGA light red.
func (e *Engine) pageSet(x, y int, c uint8) {
	if x >= 0 && y >= 0 && x < pageStride && y < 168 {
		e.page[y*pageStride+x] = c
	}
}

// SetPaletteColor = 0464:2A0D (colour, index): EGA palette register.
func (e *Engine) SetPaletteColor(colour, index int) {
	e.pal[index&15] = uint8(colour & 15)
	e.S.SetPalette(index, colour) // ponytail: mirrored for the classic frame until it goes
}

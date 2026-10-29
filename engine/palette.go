package engine

// SetPaletteColor = 0464:2A0D (colour, index): EGA palette register.
func (e *Engine) SetPaletteColor(colour, index int) {
	e.pal[index&15] = uint8(colour & 15)
}

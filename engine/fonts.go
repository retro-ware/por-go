package engine

import (
	"embed"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Text fonts (extra): Ctrl-F cycles from the game's
// own font through the 8x8 fonts of The Ultimate Oldschool PC Font Pack
// (fonts/, CC BY-SA 4.0, see fonts/LICENSE.txt), Ctrl-Shift-L goes back.
// A PC font draws text in its real case (the game's font has capitals
// only); the game's symbols (borders, cursors) keep its own glyphs. The
// choice is kept in data/save/FONT.CFG.

//go:embed fonts/*.f8
var fontFS embed.FS

// pcFont is a code page 437 font: 256 glyphs of 8 rows, bit 7 leftmost.
type pcFont struct {
	name string
	bits []byte
}

var pcFonts = func() []pcFont {
	ents, _ := fontFS.ReadDir("fonts")
	var fs []pcFont
	for _, e := range ents {
		b, _ := fontFS.ReadFile("fonts/" + e.Name())
		fs = append(fs, pcFont{strings.TrimSuffix(e.Name(), ".f8"), b})
	}
	sort.Slice(fs, func(i, j int) bool { return strings.ToLower(fs[i].name) < strings.ToLower(fs[j].name) })
	return fs
}()

// textCell is what drawChar last put into a cell, so a font change can
// redraw the text still showing there.
type textCell struct {
	set, upcase   bool
	c, colour, bg byte
	glyph         [8]byte
}

const fontFile = "FONT.CFG"

// loadFont reads the fonts in data/assets/fonts and picks the one named in
// FONT.CFG.
func (e *Engine) loadFont() {
	e.ext.fonts = nil
	ents, _ := os.ReadDir(filepath.Join(e.data, AssetsDir, "fonts"))
	for _, d := range ents {
		if !strings.EqualFold(filepath.Ext(d.Name()), ".f8") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(e.data, AssetsDir, "fonts", d.Name()))
		if err == nil && len(b) == 256*8 {
			e.ext.fonts = append(e.ext.fonts, pcFont{strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())), b})
		}
	}
	e.ext.fonts = append(e.ext.fonts, pcFonts...)
	b, err := os.ReadFile(filepath.Join(e.Cfg.SaveDir, fontFile))
	if err != nil {
		return
	}
	name := strings.TrimSpace(string(b))
	for i, f := range e.ext.fonts {
		if f.name == name {
			e.ext.font = i + 1
		}
	}
}

// cycleFont moves to the next font (step 1) or the one before (-1), redraws
// the text on screen and shows the font's name for a moment.
func (e *Engine) cycleFont(step int) {
	n := len(e.ext.fonts) + 1
	e.ext.font = (e.ext.font + step + n) % n
	e.ext.note, e.ext.noteAt = e.FontName(), time.Now()
	for y := range e.ext.cells {
		for x := range e.ext.cells[y] {
			c := &e.ext.cells[y][x]
			if !c.set {
				continue
			}
			e.paintChar(x, y, int(c.colour), int(c.bg), c.c, c.upcase)
		}
	}
	name := ""
	if e.ext.font > 0 {
		name = e.ext.fonts[e.ext.font-1].name
	}
	os.WriteFile(filepath.Join(e.Cfg.SaveDir, fontFile), []byte(name+"\n"), 0o644)
}

// FontName is the current font's name ("Pool of Radiance" for the game's).
func (e *Engine) FontName() string {
	if e.ext.font == 0 {
		return "Pool of Radiance"
	}
	return strings.ReplaceAll(e.ext.fonts[e.ext.font-1].name, "_", " ")
}

// Note is a short message for the window to show over the picture (the
// font name after Ctrl-F) and when it was set.
func (e *Engine) Note() (string, time.Time) { return e.ext.note, e.ext.noteAt }

// glyph is the 8x8 pattern drawChar draws for ch, or false when there is
// none.
func (e *Engine) glyph(ch byte, upcase bool) ([8]byte, bool) {
	var g [8]byte
	if e.ext.font > 0 && upcase && ch >= 0x20 && ch < 0x7F {
		copy(g[:], e.ext.fonts[e.ext.font-1].bits[int(ch)*8:])
		return g, true
	}
	i := int(ch)
	if upcase {
		switch ch { // the game's font has no square brackets (those slots hold other symbols): its angle ones stand in
		case '[':
			ch = '<'
		case ']':
			ch = '>'
		}
		i = int(upper(ch)) % 0x40
	}
	if e.font == nil || i*8+8 > len(e.font) {
		return g, false
	}
	copy(g[:], e.font[i*8:])
	return g, true
}

// Glyph is the pattern of ch as text in the current font (the window's
// note uses it).
func (e *Engine) Glyph(ch byte) [8]byte {
	g, _ := e.glyph(ch, true)
	return g
}

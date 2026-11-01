package engine

import (
	"image"
	"image/color"
	"image/draw"
	"path/filepath"

	"por/dax"
	"por/pal"
)

// fullPicture draws a whole-screen picture (320x200) in the middle of the
// page: on a page of another shape (a full screen) it gets black bars,
// never a crop or a stretch.
func (e *Engine) fullPicture(pic *dax.Image) {
	e.ClearScreen()
	e.DrawPicture((textCols-minCols)/2, (textRows-minRows)/2, pic, 0)
}

// Title = ovr02:0626 title_screen: a timed screen (modeTitle) through the
// title pictures, the lightning and the credits; a key cuts each short.
func (e *Engine) Title() {
	e.Music(1) // the Amiga intro, 0x2B580
	e.fullPicture(e.LoadPic("TITLE", 1))
	const (
		first   = iota // TITLE 1, five seconds (WaitSeconds)
		second         // TITLE 2 with the lightning, five seconds
		credits        // ten seconds (WaitSeconds)
	)
	phase, end := first, e.Ticks()+500
	var acc int
	var flash []struct{ c, ms int } // the lightning's steps left (Delay each)
	var flashEnd int64
	off := func() {}
	next := func() bool { // the phase after this one; true when the title is done
		switch phase {
		case first:
			if !e.Cfg.FullIntro {
				return true
			}
			phase, end = second, e.Ticks()+500
			e.fullPicture(e.LoadPic("TITLE", 2))
		case second:
			e.ClearKeyboard()
			phase, end = credits, e.Ticks()+1000 // Credits = ovr02:0206
			off = e.redrawWith(e.drawCredits)
			e.drawCredits()
		default:
			return true
		}
		return false
	}
	e.pushTimed(modeTitle, func(a Action) bool {
		if len(flash) > 0 { // Delay: the step goes on until its time is up or a key
			if a.Kind == ActTick && e.Ticks() < flashEnd {
				return false
			}
			if a.Kind != ActTick && !e.pressed(a) {
				return false
			}
			flash = flash[1:]
			if len(flash) == 0 || len(flash)%5 == 0 {
				e.SetPaletteColor(1, 1)
			}
			if len(flash) > 0 {
				e.SetPaletteColor(flash[0].c, 1)
				flashEnd = e.Ticks() + int64(flash[0].ms/10)
			}
			return false
		}
		key := a.Kind != ActTick && e.pressed(a)
		if phase != second {
			if key || a.Kind == ActTick && e.Ticks() >= end {
				return next()
			}
			return false
		}
		if a.Kind != ActTick && !key {
			return false
		}
		if a.Kind == ActTick && e.Cfg.Graphics == 'E' { // [4A1E] == 1: lightning on palette entry 1
			acc += e.Random(4)
			if acc > 50000 {
				acc = 0
				n := e.Random(3) + 1
				for i := 0; i < n; i++ {
					flash = append(flash, []struct{ c, ms int }{{9, 150}, {11, 100}, {15, 50}, {11, 100}, {9, 150}}...)
				}
				e.SetPaletteColor(flash[0].c, 1)
				flashEnd = e.Ticks() + int64(flash[0].ms/10)
				return false
			}
		}
		if key || e.Ticks() >= end {
			return next()
		}
		return false
	}, func() {
		off()
		e.Music(0)
	})
}

// drawCredits = ovr02:0206's page.
func (e *Engine) drawCredits() {
	e.frame(1, 1, winRight, 3, colBlack, "")
	e.frame(1, 5, winRight, promptRow-1, colBlack, "")
	e.snap.credits = true // the lines are the presenter's (View.Credits)
}

// RemasterCredit is the remaster's own lines at the bottom of the credits
// (extra: centred, yellow, from three rows over the prompt row, on a page
// tall enough for them: two rows under the last of Credits).
var RemasterCredit = [2]string{"Opinionated Remaster", "directed by Felix Ruzzoli using Claude"}

// Credit is a line of the credits page: Text at cell X, Y in EGA colour
// Colour (DisplayString's way, upcased).
type Credit struct {
	X, Y, Colour int
	Text         string
}

// Credits is ovr02:0206's page.
var Credits = []Credit{
	{5, 1, 0xa, "scenario created by:"},
	{0x1a, 1, 0xe, "tsr, inc."},
	{0xf, 2, 0xa, "jim ward"},
	{2, 3, 0xa, "david cook,steve winter,mike breault"},
	{1, 5, 0xa, "game created by:"},
	{0x12, 5, 0xe, "ssi special projects"},
	{0xe, 6, 0xb, "version 1.3"},
	{2, 7, 0xe, "programming:"},
	{0x14, 7, 0xe, "original"},
	{2, 8, 0xb, "scot bayless"},
	{0x14, 8, 0xe, "programming:"},
	{2, 9, 0xb, "brad myers"},
	{0x14, 9, 0xb, "keith brors"},
	{2, 0xa, 0xb, "russ brown"},
	{0x14, 0xa, 0xb, "brad myers"},
	{2, 0xb, 0xb, "ted greer"},
	{2, 0xd, 0xe, "graphic arts:"},
	{0x14, 0xd, 0xe, "encounter coding:"},
	{2, 0xe, 0xb, "tom wahl"},
	{0x14, 0xe, 0xb, "paul murray"},
	{2, 0xf, 0xb, "fred butts"},
	{0x14, 0xf, 0xb, "russ brown"},
	{2, 0x10, 0xb, "darla marasco"},
	{0x14, 0x10, 0xb, "victor penman"},
	{2, 0x11, 0xb, "susan halbleib"},
	{0x14, 0x11, 0xb, "dave shelley"},
	{2, 0x13, 0xe, "project manager"},
	{0x14, 0x13, 0xe, "developer:"},
	{2, 0x14, 0xb, "victor penman"},
	{0x14, 0x14, 0xb, "george mac donald"},
	{2, 0x16, 0xe, "testing:"},
	{0xb, 0x16, 0xb, "joel billings,steve salyer"},
	{2, 0x17, 0xb, "james kucera,robert daly,rick white"},
}

// Title logo (extra): the remastered main menu shows the title picture's
// lettering on black. The second TITLE picture paints "Pool of Radiance"
// in yellow with a light blue shadow over a stone wall, and the subtitle
// under it in light magenta; only those colours are kept.
const (
	logoSub    = 108 // the subtitle's first row
	logoBottom = 160 // under the subtitle
)

// The subtitle in the Amiga intro's colours: gold (0xF90) with a dark red
// (0x600) outline, the intro's credits palette entries 2 and 1.
var subGold, subRed = color.RGBA{0xff, 0x99, 0, 0xff}, color.RGBA{0x66, 0, 0, 0xff}

// titleBackdrop is the Amiga painted title (data/amiga/TITLE.PNG, written by
// -import-disk2), nil without it.
func (e *Engine) titleBackdrop() image.Image {
	if e.snap.backdropRead {
		return e.snap.backdrop
	}
	e.snap.backdropRead = true
	e.snap.backdrop = readPNG(filepath.Join(e.data, AmigaDir, "TITLE.PNG"))
	return e.snap.backdrop
}

// titleLogo cuts the lettering out of TITLE picture 2 (nil if missing).
func (e *Engine) titleLogo() image.Image {
	if e.snap.logo != nil {
		return e.snap.logo
	}
	im := e.LoadPic("TITLE", 2)
	if im == nil {
		return nil
	}
	w := im.Width * 8
	out := image.NewRGBA(image.Rect(0, 0, w, min(im.Height, logoBottom)))
	var r image.Rectangle // the lettering's extent
	for i, c := range im.Frame(0) {
		x, y := i%w, i/w
		c &= 15
		if y < logoSub && (c == colYellow || c == colLightBlue) {
			out.Set(x, y, pal.EGA(int(c)))
			r = r.Union(image.Rect(x, y, x+1, y+1))
		} else if y >= logoSub && y < logoBottom && c == colLightMagenta {
			out.Set(x, y, subGold)
			r = r.Union(image.Rect(x-1, y-1, x+2, y+2))
		}
	}
	for y := logoSub - 1; y <= logoBottom; y++ { // the outline round the gold
		for x := 0; x < w; x++ {
			if out.RGBAAt(x, y).A != 0 {
				continue
			}
			for _, d := range [...]image.Point{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}} {
				if out.RGBAAt(x+d.X, y+d.Y) == subGold {
					out.Set(x, y, subRed)
					break
				}
			}
		}
	}
	cut := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(cut, cut.Bounds(), out, r.Min, draw.Src)
	e.snap.logo = cut
	return cut
}

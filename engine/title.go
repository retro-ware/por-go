package engine

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
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

// Title = ovr02:0626 title_screen.
func (e *Engine) Title() {
	e.Music(1) // the Amiga intro, 0x2B580
	defer e.Music(0)
	e.fullPicture(e.LoadPic("TITLE", 1))
	e.WaitSeconds(5)
	if !e.Cfg.FullIntro {
		return
	}
	e.fullPicture(e.LoadPic("TITLE", 2))
	end := e.Ticks() + 500
	var acc int
	for {
		if e.Cfg.Graphics == 'E' { // [4A1E] == 1: lightning on palette entry 1
			acc += e.Random(4)
			if acc > 50000 {
				acc = 0
				n := e.Random(3) + 1
				for i := 0; i < n; i++ {
					for _, f := range [...]struct{ c, ms int }{{9, 150}, {11, 100}, {15, 50}, {11, 100}, {9, 150}} {
						e.SetPaletteColor(f.c, 1)
						e.Delay(f.ms)
					}
					e.SetPaletteColor(1, 1)
				}
			}
		}
		if e.keyHit() || e.Ticks() >= end {
			break
		}
		e.idle()
	}
	e.ClearKeyboard()
	e.Credits()
}

// Credits = ovr02:0206.
func (e *Engine) Credits() {
	draw := func() {
		e.DrawBoxTitled(1, 1, winRight, 3, 0, colBlack, "")
		e.DrawBoxTitled(1, 5, winRight, promptRow-1, 0, colBlack, "")
		for _, t := range creditLines {
			e.DisplayString(t.x, t.y, t.c, t.s)
		}
		if promptRow-2 >= creditLines[len(creditLines)-1].y+2 { // extra: the remaster's own lines, on a page tall enough for them
			for i, s := range RemasterCredit {
				e.DisplayString((textCols-len(s))/2, promptRow-3+i, colYellow, s)
			}
		}
	}
	defer e.redrawWith(draw)()
	draw()
	e.WaitSeconds(10)
}

// RemasterCredit is the remaster's own lines at the bottom of the credits.
var RemasterCredit = [2]string{"Opinionated Remaster", "directed by Felix Ruzzoli using Claude"}

var creditLines = []struct {
	x, y, c int
	s       string
}{
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
	if f, err := os.Open(filepath.Join(e.data, AmigaDir, "TITLE.PNG")); err == nil {
		e.snap.backdrop, _ = png.Decode(f)
		f.Close()
	}
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

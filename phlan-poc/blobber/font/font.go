// Package font draws text in "ThickOne" by artiil, a 16x16 bitmap font made with BitFontMaker2:
// https://www.pentacom.jp/pentacom/bitfontmaker2/gallery/?id=2854 (Creative Commons Attribution).
//
// thickone.json maps a rune to 16 rows; bit x of a row is pixel x. Glyphs start at column 2.
package font

import (
	_ "embed"
	"encoding/json"
	"image"
	"image/color"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed thickone.json
var data []byte

const (
	cell   = 16
	left   = 2 // first column of every glyph
	space  = 4 // advance of a space
	track  = 1 // gap between glyphs
	Height = 16
)

type glyph struct {
	rows    [cell]uint16
	advance int
	img     *ebiten.Image // built on first use
}

var glyphs = func() map[rune]*glyph {
	var raw map[string][cell]uint16
	if err := json.Unmarshal(data, &raw); err != nil {
		panic(err)
	}
	m := map[rune]*glyph{}
	for k, rows := range raw {
		n, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		right := left
		for _, r := range rows {
			for x := cell - 1; x >= 0; x-- {
				if r>>x&1 != 0 {
					right = max(right, x+1)
					break
				}
			}
		}
		m[rune(n)] = &glyph{rows: rows, advance: right - left + track}
	}
	return m
}()

func get(r rune) *glyph {
	if g := glyphs[r]; g != nil {
		return g
	}
	return glyphs['?']
}

// Advance is how far r moves the pen.
func Advance(r rune) int {
	if r == ' ' {
		return space
	}
	return get(r).advance
}

// Width is the pixel width of s.
func Width(s string) int {
	w := 0
	for _, r := range s {
		w += Advance(r)
	}
	return max(0, w-track)
}

func (g *glyph) image() *ebiten.Image {
	if g.img == nil {
		img := image.NewAlpha(image.Rect(0, 0, cell, cell))
		for y, r := range g.rows {
			for x := range cell {
				if r>>x&1 != 0 {
					img.SetAlpha(x, y, color.Alpha{255})
				}
			}
		}
		g.img = ebiten.NewImageFromImage(img)
	}
	return g.img
}

// Draw writes s with its cell top-left at x,y in colour col. Ink sits roughly from y+3 to y+12,
// descenders to y+14, like ebiten's debug font.
func Draw(dst *ebiten.Image, s string, x, y int, col color.Color) {
	for _, r := range s {
		if r != ' ' {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x-left), float64(y))
			op.ColorScale.ScaleWithColor(col)
			dst.DrawImage(get(r).image(), op)
		}
		x += Advance(r)
	}
}

// Print draws white text with a 1 px dark shadow, for text over the 3D view.
func Print(dst *ebiten.Image, s string, x, y int) {
	Draw(dst, s, x+1, y+1, color.RGBA{0, 0, 0, 200})
	Draw(dst, s, x, y, color.White)
}

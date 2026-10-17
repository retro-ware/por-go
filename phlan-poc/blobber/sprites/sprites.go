// Package sprites holds debug pixel art drawn as ASCII, turned into images on first use.
// One character per pixel; see palette. Rows may differ in length; short rows are padded transparent.
package sprites

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

var palette = map[byte]color.RGBA{
	'k': {20, 20, 24, 255},    // outline
	'w': {240, 240, 235, 255}, // white
	'g': {150, 150, 155, 255}, // grey
	'G': {90, 90, 98, 255},    // dark grey
	's': {200, 205, 215, 255}, // steel
	'b': {130, 85, 45, 255},   // brown
	'B': {85, 55, 30, 255},    // dark brown
	'y': {235, 195, 60, 255},  // gold
	'Y': {170, 130, 30, 255},  // dark gold
	'r': {205, 45, 45, 255},   // red
	'u': {60, 100, 220, 255},  // blue
	'p': {150, 70, 180, 255},  // purple
	'c': {110, 220, 235, 255}, // cyan
	'e': {70, 170, 80, 255},   // green
	'f': {225, 175, 135, 255}, // skin
}

var art = map[string]string{
	"sword": `
..............ks
.............ksk
............ksk
...........ksk
..........ksk
.........ksk
........ksk
.......ksk
..k...ksk
..kk.ksk
...kyyk
....yYk
...kbYyk
..kbk.kyk
.kbk....k
.kk`,
	"dagger": `



...........ks
..........ksk
.........ksk
........ksk
...k...ksk
...kk.ksk
....kyyk
.....yYk
....kbYyk
...kbk..k
...kk`,
	"bow": `
.....kkk
....kbbk
...kbk.kw
...kbk..w
..kbk...w
..kbk....w
..kbk....w
..kbk....w
..kbk....w
..kbk....w
..kbk...w
...kbk..w
...kbk.kw
....kbbk
.....kkk`,
	"shield": `

..kkkkkkkkkkkk
..kuuuuyyuuuuk
..kuuuuyyuuuuk
..kuuuuyyuuuuk
..kyyyyyyyyyyk
..kyyyyyyyyyyk
..kuuuuyyuuuuk
...kuuuyyuuuk
...kuuuyyuuuk
....kuuyyuuk
.....kuyyuk
......kyyk
.......kk`,
	"helmet": `


.....kkkkkk
....kssssssk
...kssggggssk
..kssgssssssk
..ksgssssssgk
..kssssssssssk
..ksssskkkkssk
..kssk....kssk
..kssk....kssk
..kssk....kssk
..kkkk....kkkk`,
	"armor": `

...kkk....kkk
..kgsgk..kgsgk
..kgssskksssgk
..kgssssssssgk
...kgssssssgk
...kgsGssGsgk
...kgssssssgk
...kgsGssGsgk
...kgssssssgk
...kgssssssgk
...kGgggggggk
....kkkkkkkk`,
	"boots": `


....kkkk
....kbbk
....kbbk
....kbbk...kkkk
....kbbk...kbbk
....kbbk...kbbk
....kbbbk..kbbk
...kbbbbbk.kbbk
...kBBBBBk.kbbbk
...kkkkkkk.kbbbbk
...........kBBBBk
...........kkkkkk`,
	"robe": `

.....kk..kk
....kppkkppk
...kpppyypppk
..kppppyyppppk
..kpkppyypppkpk
..kk.kpyypk.kk
.....kpyypk
.....kpyypk
....kppyyppk
....kppyyppk
...kpppyypppk
...kppppppppk
...kkkkkkkkkk`,
	"ring": `



.......kk
......kcck
.....kkcckk
....kyykkyyk
...kyk....kyk
...ky......yk
...ky......yk
...kyk....kyk
....kyyyyyyk
.....kkkkkk`,
	"amulet": `
....k......k
.....k....k
......k..k
.......kk
......kyyk
.....kyrryk
....kyrrrryk
....kyrrrryk
.....kyrryk
......kyyk
.......kk`,
	"potion": `

......kkkk
......kbbk
.......kk
......kwwk
......kwwk
.....kwwwwk
....kwrrrrwk
...kwrrrrrrwk
...krrrrrrrrk
...krrrrrwrrk
...krrrrrwrrk
....krrrrrrk
.....kkkkkk`,
	"scroll": `

..kkkkkkkkkkk
.kwwwwwwwwwwwk
.kkkkkkkkkkkwk
...kwwwwwwwwk
...kwGGGGGwwk
...kwwwwwwwwk
...kwGGGGwwwk
...kwwwwwwwwk
...kwGGGGGGwk
...kwwwwwwwwk
..kkkkkkkkkkwk
.kwwwwwwwwwwwk
..kkkkkkkkkkk`,
	"key": `



....kkkk
...kyyyyk
..kyykkyyk
..kyk..kyk
..kyykkyyk
...kyyyyyk
....kkkyykkkkkk
.......kyyyyyyyk
.......kkkkykykk
..........kk.k`,
	"bag": `

......kkkk
.......kk
......kbbk
.....kbbbbk
....kbbbbbbk
...kbbbbbbbbk
...kbbbyybbbk
...kbbbyybbbk
...kbbbbbbbbk
....kbbbbbbk
.....kkkkkk`,
	"coin": `
....kkkkkk
...kyyyyyyk
..kyyYYYYyyk
..kyYyyyyYyk
..kyYyyyyYyk
..kyyYYYYyyk
...kyyyyyyk
....kkkkkk`,
	"gem": `

.....kkkkkk
....kcwcccck
...kcwcccccck
..kkkkkkkkkkkk
...kcccccccck
....kccccccck
.....kccccck
......kccck
.......kck
........k`,
	"guardian": `
.....kkkkkk
....kggggggk
....kgrggrgk
....kggggggk
.....kgGGgk
..kkkkggggkkkk
.kgggkggggkgggk
.kgggkGggGkgggk
.kggk.kggk.kggk
.kGGk.kggk.kGGk
..kk.kggggk.kk
.....kgkkgk
....kggk.kggk
....kggk.kggk
...kGGGk.kGGGk
...kkkkk.kkkkk`,
	"statue": `
......kkkk
.....kwwwwk
.....kwkwkk
.....kwwwwk
......kwwk
....kkwwwwkk
...kwwwwwwwwk
...kwkwwwwkwk
...kwkwwwwkwk
....k.kwwk.k
......kwwk
.....kwkkwk
.....kwk.kwk
....kkkkkkkkk
....kGGGGGGGk
....kkkkkkkkk`,
	"lever_up": `

.....kk
....krrk
....krrk
.....kk
......kgk
.......kgk
........kgk
.........kgk
..........kgk
......kkkkkkkkk
......kGGGGGGGk
......kGgggggGk
......kGGGGGGGk
......kkkkkkkkk`,
	"lever_down": `





......kkkkkkkkk
......kGGGGGGGk
......kGgggggGk
......kGGGGGGGk
......kkkkkkkkk
..........kgk
.........kgk
........kgk
.......kgk
.....kkkk
....krrk
....krrk
.....kk`,
}

var cache = map[string]*ebiten.Image{}

// Get returns the named sprite, or nil if there is none.
func Get(name string) *ebiten.Image {
	if img, ok := cache[name]; ok {
		return img
	}
	src, ok := art[name]
	if !ok {
		return nil
	}
	img := ebiten.NewImageFromImage(Decode(src))
	cache[name] = img
	return img
}

// Decode turns ASCII art into an RGBA image; the first line (after the opening backtick) is skipped if empty.
func Decode(src string) *image.RGBA {
	rows := strings.Split(strings.TrimPrefix(src, "\n"), "\n")
	w := 1
	for _, r := range rows {
		w = max(w, len(r))
	}
	img := image.NewRGBA(image.Rect(0, 0, w, len(rows)))
	for y, r := range rows {
		for x := 0; x < len(r); x++ {
			if c, ok := palette[r[x]]; ok {
				img.SetRGBA(x, y, c)
			}
		}
	}
	return img
}

// Names lists all sprites, for tests and tools.
func Names() []string {
	var out []string
	for k := range art {
		out = append(out, k)
	}
	return out
}

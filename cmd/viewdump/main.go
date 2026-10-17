// viewdump [-scale N] [DIR [OUT]]: writes the walls of the 3D view as PNGs
// into OUT (default DIR/viewdump). Each WALLDEF<n>.DAX block holds chunks of
// five walls; a wall is the 8x8 symbol ids of the ten view parts
// (engine/geo.go partBase..partH, far front to near sides). One PNG per
// wall shows its front face as seen from right in front of it (part 6),
// assembled from the symbol sets the engine uses: ids up to 0x2D from 8X8D1 block 203, the rest from the
// wall's own blocks (LoadWalldef). Colour 13 shows the default ground
// behind the wall (the sky stays transparent), or all of it stays
// transparent with -transparent.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"por/dax"
	"por/ega"
)

// The engine's tables (engine/geo.go, engine/pic.go), unexported there.
const (
	walldefSize = 0x9C
	walldefSet  = 5 * walldefSize
	nearFront   = 6 // the part drawn for a wall right in front of the party
)

// symFix: first symbol id of each set (engine/pic.go); ids up to 0x11E.
var symFix = [5]int{0x001, 0x02E, 0x074, 0x0BA, 0x100}

var (
	partBase = [10]int{0, 2, 6, 10, 22, 38, 54, 110, 132, 154}
	partW    = [10]int{1, 1, 1, 3, 2, 2, 7, 2, 2, 1}
	partH    = [10]int{2, 4, 4, 4, 8, 8, 8, 11, 11, 2}
)

func main() {
	scale := flag.Int("scale", 4, "pixel scale")
	transparent := flag.Bool("transparent", false, "leave the ground behind the wall transparent too")
	flag.Parse()
	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}
	out := filepath.Join(dir, "viewdump")
	if flag.NArg() > 1 {
		out = flag.Arg(1)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		log.Fatal(err)
	}
	l, err := dax.NewLoader(dir)
	if err != nil {
		log.Fatal(err)
	}
	n := 0
	for set := 1; set <= 8; set++ {
		wname, sname := fmt.Sprintf("WALLDEF%d", set), fmt.Sprintf("8X8D%d", set)
		f := l.File(wname)
		if f == nil {
			continue
		}
		for _, en := range f.Entries {
			b := f.Block(en.ID)
			chunks := len(b) / walldefSet
			// The block loaded into wall slot 1, as LoadWalldef does: set 0
			// is 8X8D1 block 203 (loaded at start, init.go), chunk k's
			// symbols are set k.
			var sets [4]*dax.Image
			sets[0] = pic(l, "8X8D1", 203)
			for k := 1; k <= chunks && k <= 3; k++ {
				sb := en.ID
				if chunks > 1 {
					sb = en.ID * 10
					if en.ID == 0 {
						sb = 0xA * 10
					}
					sb += k
				}
				sets[k] = pic(l, sname, sb)
			}
			for k := 1; k <= chunks; k++ {
				for w := range 5 {
					def := b[(k-1)*walldefSet+w*walldefSize:][:walldefSize]
					p := filepath.Join(out, fmt.Sprintf("%s-%03d-%d-wall%d.png", wname, en.ID, k, w+1))
					if err := writeWall(p, def, sets, *scale, !*transparent); err != nil {
						log.Fatal(err)
					}
					n++
				}
			}
		}
	}
	fmt.Printf("%d walls in %s\n", n, out)
}

// pic loads a symbol set as the engine does (masked on colour 13).
func pic(l *dax.Loader, name string, id int) *dax.Image {
	b := l.Block(name, id)
	if b == nil {
		return nil
	}
	return dax.ParseImage(b, true, 0xD)
}

// groundRows: the default colours below the sky (SetSkyColours(0xB, 0, 7,
// 6): horizon, band, ground) by the first wall row each covers; drawSky's
// page rows less the 24 rows above the near front (view row 16, part row 1).
var groundRows = [3]struct {
	from   int
	colour byte
}{{0x3B - 24, 0}, {0x3C - 24, 7}, {0x3E - 24, 6}}

func writeWall(path string, def []byte, sets [4]*dax.Image, scale int, ground bool) error {
	img := image.NewNRGBA(image.Rect(0, 0, partW[nearFront]*8*scale, partH[nearFront]*8*scale))
	if ground {
		for _, band := range groundRows {
			p := ega.Palette[band.colour]
			r := image.Rect(0, band.from*scale, img.Rect.Dx(), img.Rect.Dy())
			draw.Draw(img, r, image.NewUniform(color.NRGBA{p[0], p[1], p[2], 255}), image.Point{}, draw.Src)
		}
	}
	part, ox := nearFront, 0
	s := partBase[part]
	for cy := range partH[part] {
		for cx := range partW[part] {
			id := int(def[s])
			s++
			set := 0
			for set < 3 && id >= symFix[set+1] {
				set++
			}
			im, frame := sets[set], id-symFix[set]
			if id == 0 || im == nil || frame >= im.Count {
				continue
			}
			for j, c := range im.Frame(frame) {
				if c >= dax.Clear {
					continue
				}
				p := ega.Palette[c&15]
				col := color.NRGBA{p[0], p[1], p[2], 255}
				x, y := ox+cx*8+j%8, cy*8+j/8
				for sy := range scale {
					for sx := range scale {
						img.SetNRGBA(x*scale+sx, y*scale+sy, col)
					}
				}
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

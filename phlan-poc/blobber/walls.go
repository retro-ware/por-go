package blobber

import (
	"bufio"
	"bytes"
	"embed"
	"image"
	"image/png"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed assets/walls/*.png assets/walls/walls.txt
var wallFS embed.FS

var wallCache = map[string]*ebiten.Image{}

func decodeWall(name string) image.Image {
	b, err := wallFS.ReadFile("assets/walls/" + name + ".png")
	if err != nil {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	return img
}

// wallTexture loads assets/walls/<name>.png (once), or nil when there is none; a painted texture
// replaces the generated one for its material character. Its relief comes from NAME_normal.png
// when there is one (OpenGL convention, green up), else from its brightness.
func wallTexture(name string) *ebiten.Image {
	if t, ok := wallCache[name]; ok {
		return t
	}
	img := decodeWall(name)
	if img == nil {
		wallCache[name] = nil
		return nil
	}
	t := ebiten.NewImageFromImage(img)
	wallCache[name] = t
	if nm := decodeWall(name + "_normal"); nm != nil && nm.Bounds().Size() == img.Bounds().Size() {
		normalSlopes(t, nm)
	} else if !strings.HasSuffix(name, "_light") { // painted: the brightness stands in for the height
		b := img.Bounds()
		addSlopes(t, b.Dx(), b.Dy(), func(x, y int) float64 {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			return (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 65535
		})
	}
	return t
}

// paintedWalls reads assets/walls/walls.txt: map character -> texture name.
func paintedWalls() map[byte]string {
	out := map[byte]string{}
	f, err := wallFS.Open("assets/walls/walls.txt")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.Fields(strings.SplitN(sc.Text(), "#", 2)[0]); len(l) == 2 && len(l[0]) == 1 {
			out[l[0][0]] = l[1]
		}
	}
	return out
}

// Flats lists the painted floor or ceiling textures (kind "floor" or "ceiling"): the NAME of every
// assets/walls/<kind>_NAME.png, which a map picks with a "floor NAME" or "ceiling NAME" line.
func Flats(kind string) []string {
	var out []string
	entries, _ := wallFS.ReadDir("assets/walls")
	for _, e := range entries {
		if n, ok := strings.CutPrefix(e.Name(), kind+"_"); ok && !strings.HasSuffix(n, "_normal.png") {
			out = append(out, strings.TrimSuffix(n, ".png"))
		}
	}
	sort.Strings(out)
	return out
}

var flatNames [2]string // the current map's floor and ceiling ("" = default)

// openSky: the ceiling is a sky_* texture, drawn as a backdrop behind everything instead of as
// ceiling tiles, panning with the view angle.
var openSky bool

// SetFlats makes floor and ceiling (names from Flats; "" = the default textures) the current
// map's. Hosts call it on every map change; it holds across GenTextures.
func SetFlats(floor, ceiling string) {
	flatNames = [2]string{floor, ceiling}
	openSky = strings.HasPrefix(ceiling, "sky")
	if defFloor != nil {
		texFloor, texCeil = FlatTexture("floor", floor), FlatTexture("ceiling", ceiling)
	}
}

// FlatTexture returns the floor or ceiling texture NAME, or the default one for "" or an unknown name.
func FlatTexture(kind, name string) *ebiten.Image {
	if name != "" {
		if t := wallTexture(kind + "_" + name); t != nil {
			return t
		}
	}
	if kind == "floor" {
		return defFloor
	}
	return defCeil
}

package blobber

import (
	"bufio"
	"bytes"
	"embed"
	"image"
	"image/png"
	"os"
	"strings"

	"gpu"
)

//go:embed assets/walls/*.png assets/walls/walls.txt
var wallFS embed.FS

var wallCache = map[string]*gpu.Image{}

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
// replaces the generated one for its material character.
func wallTexture(name string) *gpu.Image {
	if t, ok := wallCache[name]; ok {
		return t
	}
	img := decodeWall(name)
	if img == nil {
		wallCache[name] = nil
		return nil
	}
	t := gpu.NewImageFromImage(img)
	wallCache[name] = t
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

var flatNames [2]string // the current map's floor and ceiling ("" = default)

// openSky: the ceiling is a sky_* texture, drawn as a backdrop behind everything instead of as
// ceiling tiles, panning with the view angle.
var openSky bool

// SetFlats makes floor and ceiling (names from Flats; "" = the default textures) the current
// map's. Hosts call it on every map change; it holds across GenTextures.
func SetFlats(floor, ceiling string) {
	if c := os.Getenv("BLOBBER_CEILING"); c != "" { // phlan-poc: try a ceiling (e.g. cave) on any map
		ceiling = c
	}
	flatNames = [2]string{floor, ceiling}
	openSky = strings.HasPrefix(ceiling, "sky")
	worldChanged()
	if defFloor != nil {
		texFloor, texCeil = FlatTexture("floor", floor), FlatTexture("ceiling", ceiling)
	}
}

// FlatTexture returns the floor or ceiling texture NAME, or the default one for "" or an unknown name.
func FlatTexture(kind, name string) *gpu.Image {
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

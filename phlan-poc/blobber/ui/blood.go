package ui

import (
	"bytes"
	_ "embed"
	"image/png"
	"math/rand"

	"blobber"

	"github.com/hajimehoshi/ebiten/v2"
)

// The blood splatter of physical hits (assets/blood.png, generated with Gemini, 64 px pixel art).
//
//go:embed assets/blood.png
var bloodPNG []byte

var blood = func() *ebiten.Image {
	img, err := png.Decode(bytes.NewReader(bloodPNG))
	if err != nil {
		panic(err)
	}
	return ebiten.NewImageFromImage(img)
}()

const splatFrames = 40 // how long a splatter stays, fading over the last half

// splat is one splatter: on character who's portrait (0-based), or on the monster ahead (who < 0).
type splat struct {
	who, dmg, at int
	flipX, flipY bool
}

var splats []splat

// Bleed shows a blood splatter for a physical hit of dmg: over the monster in the party's cell
// (who < 0) or over character who's (0-based) portrait. More damage, bigger splatter.
func Bleed(who, dmg int) {
	splats = append(splats, splat{who, dmg, frameNo, rand.Intn(2) == 0, rand.Intn(2) == 0})
}

// splatScale is the splatter's size in blood.png widths: 1 damage is small, about 12 fills the
// view (portraits: 12 about covers the picture).
func splatScale(dmg int, max float64) float64 {
	return min(max, max*(0.25+0.75*float64(dmg)/12))
}

// drawSplats draws the live splatters on screen; portrait is card i's portrait centre and scale.
func drawSplats(screen *ebiten.Image, portrait func(i int) (x, y, k float64)) {
	live := splats[:0]
	for _, s := range splats {
		age := frameNo - s.at
		if age >= splatFrames {
			continue
		}
		live = append(live, s)
		var x, y, sc float64
		if s.who < 0 {
			x, y = float64(blobber.W)/2, float64(blobber.H)*0.45
			sc = splatScale(s.dmg, float64(blobber.H)*0.7/float64(blood.Bounds().Dx()))
		} else {
			var k float64
			x, y, k = portrait(s.who)
			sc = splatScale(s.dmg, k*PortraitSize*1.3/float64(blood.Bounds().Dx()))
		}
		sc = max(1, float64(int(sc))) // whole multiples: crisp pixels
		b := blood.Bounds()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(-float64(b.Dx())/2, -float64(b.Dy())/2)
		fx, fy := 1.0, 1.0
		if s.flipX {
			fx = -1
		}
		if s.flipY {
			fy = -1
		}
		op.GeoM.Scale(sc*fx, sc*fy)
		op.GeoM.Translate(x, y)
		if fade := splatFrames - age; fade < splatFrames/2 {
			op.ColorScale.ScaleAlpha(float32(fade) / (splatFrames / 2))
		}
		op.Filter = ebiten.FilterNearest
		screen.DrawImage(blood, op)
	}
	splats = live
}

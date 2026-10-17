package blobber

import (
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"path"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/draw"
)

// PxPerTile is every painted monster's pixel density: Scaled resamples to it, so a small monster
// and a giant look equally sharp, and how big one stands comes only from its size.
const PxPerTile = 1024

// Sheet is a monster's pictures: an idle pose, plus attack and hurt poses when it has them.
// Size is how tall it stands in tiles (0 = 0.6). A sheet from Scaled is drawn at PxPerTile instead,
// as wide as its pictures are; other sheets (debug sprites) are squeezed to at most max(1, Size) wide.
type Sheet struct {
	Idle, Attack, Hurt *ebiten.Image
	Size               float64
	src                [3]image.Image // decoded idle, attack, hurt, for Scaled
	density            bool           // drawn at PxPerTile
}

// Scaled returns the sheet resampled so its tallest pose stands size tiles tall at PxPerTile:
// all poses, aligned at their bottom centre, are trimmed to one shared box (transparent margins go)
// and scaled by one factor.
func (s *Sheet) Scaled(size float64) *Sheet {
	out := *s
	out.Size = size
	if s.src[0] == nil {
		return &out
	}
	if size == 0 {
		size = 0.6
	}
	// Poses may differ in canvas size: they line up at their bottom centre (feet on the floor),
	// and one box around all of them is cut from each.
	at := func(img image.Image) image.Point { b := img.Bounds(); return image.Pt(b.Min.X+b.Dx()/2, b.Max.Y) }
	var box image.Rectangle
	for _, img := range s.src {
		if img != nil {
			box = box.Union(opaqueBounds(img).Sub(at(img)))
		}
	}
	f := size * PxPerTile / float64(box.Dy())
	w, h := max(1, int(float64(box.Dx())*f+0.5)), max(1, int(float64(box.Dy())*f+0.5))
	for i, dst := range []**ebiten.Image{&out.Idle, &out.Attack, &out.Hurt} {
		if img := s.src[i]; img != nil {
			pad := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy())) // the pose placed in the shared box
			draw.Draw(pad, pad.Bounds(), img, box.Min.Add(at(img)), draw.Src)
			rgba := image.NewRGBA(image.Rect(0, 0, w, h))
			draw.CatmullRom.Scale(rgba, rgba.Bounds(), pad, pad.Bounds(), draw.Src, nil)
			*dst = ebiten.NewImageFromImage(rgba)
		}
	}
	out.density = true
	return &out
}

// opaqueBounds is the smallest rectangle holding every pixel that is not (nearly) transparent.
func opaqueBounds(img image.Image) image.Rectangle {
	b, r := img.Bounds(), image.Rectangle{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0x0800 {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if r.Empty() {
		return b
	}
	return r
}

// PoseTime is how long an attack or hurt pose shows before idle returns (or a dying monster goes).
const PoseTime = 0.45

// LoadSheet reads dir/idle.png and the optional dir/attack.png and dir/hurt.png from fsys.
func LoadSheet(fsys fs.FS, dir string) (*Sheet, error) {
	var src [3]image.Image
	load := func(i int, name string) (*ebiten.Image, error) {
		f, err := fsys.Open(path.Join(dir, name+".png"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("%s/%s.png: %v", dir, name, err)
		}
		src[i] = img
		return ebiten.NewImageFromImage(img), nil
	}
	idle, err := load(0, "idle")
	if err != nil {
		return nil, err
	}
	s := &Sheet{Idle: idle}
	s.Attack, _ = load(1, "attack")
	s.Hurt, _ = load(2, "hurt")
	s.src = src
	return s, nil
}

package main

import (
	"image"
	"image/color"
	"image/png"
	"os"

	"por/dax"
)

// tiledump DIR NAME OUT.png: a frame sheet, 8 to a row, at 4x, each value as a grey/hue.
func main() {
	l, _ := dax.NewLoader(os.Args[1])
	im := dax.ParseImage(l.Block(os.Args[2], 1), false, 0)
	w, h := im.Width*8, im.Height
	out := image.NewRGBA(image.Rect(0, 0, 8*(w+2)*4, (im.Count/8+1)*(h+2)*4))
	for f := 0; f < im.Count; f++ {
		for i, p := range im.Frame(f) {
			c := color.RGBA{p * 37, p * 91, p * 13, 255}
			if p%16 == 12 {
				c = color.RGBA{255, 0, 255, 255}
			}
			for d := range 16 {
				out.Set((f%8*(w+2)+i%w)*4+d%4, (f/8*(h+2)+i/w)*4+d/4, c)
			}
		}
	}
	fh, _ := os.Create(os.Args[3])
	png.Encode(fh, out)
	fh.Close()
}

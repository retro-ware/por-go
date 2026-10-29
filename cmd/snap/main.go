// Command snap renders engine screens to PNG without a window (dev check).
//
//	snap DATA OUTDIR [title|party|view [SLOT-NAME]]
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	"por/ega"
	"por/engine"
	"por/pal"
)

func dump(s *ega.Screen, name string) {
	img := image.NewRGBA(image.Rect(0, 0, pal.W, pal.H))
	for i, c := range s.Pix {
		img.Set(i%pal.W, i/pal.W, s.RGBA(c))
	}
	f, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}
	png.Encode(f, img)
	f.Close()
	if len(s.Overlays) > 0 {
		dumpHi(s, strings.TrimSuffix(name, ".png")+"_hi.png")
	}
}

// dumpHi writes the screen 4x with its overlays at that resolution, as a
// window that size shows them (box-averaged).
func dumpHi(s *ega.Screen, name string) {
	const k = 4
	img := image.NewRGBA(image.Rect(0, 0, pal.W*k, pal.H*k))
	for y := range pal.H * k {
		for x := range pal.W * k {
			img.Set(x, y, s.RGBA(s.Pix[y/k*pal.W+x/k]))
		}
	}
	for _, o := range s.Overlays {
		b := o.Part()
		w, h := o.At.Dx()*k, o.At.Dy()*k
		for y := range h {
			for x := range w {
				x0, x1 := b.Min.X+x*b.Dx()/w, b.Min.X+max((x+1)*b.Dx()/w, x*b.Dx()/w+1)
				y0, y1 := b.Min.Y+y*b.Dy()/h, b.Min.Y+max((y+1)*b.Dy()/h, y*b.Dy()/h+1)
				var r, g, bl, n uint32
				for sy := y0; sy < y1; sy++ {
					for sx := x0; sx < x1; sx++ {
						cr, cg, cb, _ := o.Img.At(sx, sy).RGBA()
						r, g, bl, n = r+cr>>8, g+cg>>8, bl+cb>>8, n+1
					}
				}
				img.Set(o.At.Min.X*k+x, o.At.Min.Y*k+y, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255})
			}
		}
	}
	f, err := os.Create(name)
	if err != nil {
		log.Fatal(err)
	}
	png.Encode(f, img)
	f.Close()
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: snap DATA OUTDIR [title|party|view [SLOT-NAME]]")
	}
	data, out := os.Args[1], os.Args[2]
	mode := "title"
	if len(os.Args) > 3 {
		mode = os.Args[3]
	}
	scr := ega.New()
	e, err := engine.New(data, scr)
	if err != nil {
		log.Fatal(err)
	}
	switch mode {
	case "title":
		e.InitGraphics()
		e.DrawPicture(0, 0, e.LoadPic("TITLE", 1), 0)
		dump(scr, out+"/title1.png")
		e.DrawPicture(0, 0, e.LoadPic("TITLE", 2), 0)
		dump(scr, out+"/title2.png")
		scr.PushKey(13)
		e.Credits()
		dump(scr, out+"/credits.png")
	case "party":
		e.InitGraphics()
		go e.PartyMenu()
		time.Sleep(300 * time.Millisecond)
		dump(scr, out+"/party.png")
		scr.PushKey('E')
		time.Sleep(300 * time.Millisecond)
		dump(scr, out+"/party_exit.png")
	case "game":
		// snap DATA OUT game SLOT KEYS: load a saved game (a slot name; a letter
		// L is the slot GAME L) through the party
		// menu, then type KEYS, one shot per key: ^ v < > arrows, | Enter,
		// @ # $ gamepad A/Y/X, [ ] ( ) d-pad up/down/left/right, !N F1..F8, and the
		// mouse: %cX,Y; click at text cell X,Y, %hX,Y; point there, %u %d
		// wheel up/down, %r right button, _ waits; %e is Esc, %l Ctrl-F. SLOT "-" stays in the
		// party menu. Each shot's hint line goes to game_NN.txt.
		slot, keys := "A", ""
		if len(os.Args) > 4 {
			slot = os.Args[4]
		}
		if len(os.Args) > 5 {
			keys = os.Args[5]
		}
		e.Init()
		if len(os.Args) > 6 {
			e.DevAction = e.DevScene(os.Args[6])
		}
		if len(slot) == 1 && slot != "-" {
			slot = "GAME " + slot // a lettered save, copied into its slot
		}
		go e.Play()
		if slot != "-" {
			i := e.SlotIndex(slot)
			if i < 0 {
				log.Fatalf("no saved game %q", slot)
			}
			scr.PushKey('L')
			time.Sleep(500 * time.Millisecond) // the slot screen up first
			for ; i > 0; i-- {
				scr.PushKey(0x5000)
			}
			scr.PushKey(13)
		}
		time.Sleep(1500 * time.Millisecond)
		dump(scr, out+"/game_00.png")
		shot := 0
		rs := []rune(keys)
		for j := 0; j < len(rs); j++ {
			k := rs[j]
			code := uint16(k)
			if k == '%' && j+1 < len(rs) {
				j++
				var x, y int
				cell := func() {
					n, _ := fmt.Sscanf(string(rs[j+1:]), "%d,%d;", &x, &y)
					if n != 2 {
						log.Fatalf("bad mouse token at %q", string(rs[j-1:]))
					}
					for rs[j] != ';' {
						j++
					}
				}
				switch rs[j] {
				case 'c':
					cell()
					code = engine.MouseKey(engine.MouseLeft, x, y)
				case 'h':
					cell()
					scr.SetMouse(x, y, true)
					code = 0
				case 'u':
					code = engine.MouseKey(engine.MouseWheelUp, 0, 0)
				case 'd':
					code = engine.MouseKey(engine.MouseWheelDown, 0, 0)
				case 'r':
					code = engine.MouseKey(engine.MouseRight, 0, 0)
				case 'e':
					code = 27
				case 'l':
					code = 0x06 // Ctrl-F: next font
				}
			}
			switch k {
			case '^':
				code = 0x4800
			case 'v':
				code = 0x5000
			case '<':
				code = 0x4B00
			case '>':
				code = 0x4D00
			case '|':
				code = 13
			case '_':
				code = 0 // just wait
			case '{':
				code = 0x4700 // Home: previous member
			case '}':
				code = 0x4F00 // End: next member
			case '[':
				code = engine.KeyPadUp
			case ']':
				code = engine.KeyPadDown
			case '(':
				code = engine.KeyPadLeft
			case ')':
				code = engine.KeyPadRight
			case '@':
				code = engine.KeyPadSelect
			case '#':
				code = engine.KeyPadMinimap
			case '$':
				code = engine.KeyPadOverview
			case '!':
				j++
				code = engine.KeyF1 + uint16(rs[j]-'1')<<8 // !N: F1..F8
			}
			if code != 0 {
				scr.PushKey(code)
			}
			time.Sleep(700 * time.Millisecond)
			shot++
			dump(scr, fmt.Sprintf("%s/game_%02d.png", out, shot))
			os.WriteFile(fmt.Sprintf("%s/game_%02d.txt", out, shot), []byte(scr.Hint()+"\n"), 0o644)
		}
	case "view":
		slot := "GAME A"
		if len(os.Args) > 4 {
			slot = os.Args[4]
		}
		e.Init()
		e.LoadSlotNamed(slot)
		for _, v := range e.DevViews() {
			v.Draw()
			dump(scr, out+"/view_"+v.Name+".png")
		}
	}
}

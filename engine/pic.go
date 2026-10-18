package engine

import (
	"por/dax"
	"por/ega"
	"strconv"
)

// symFix = DS:2722 / coab symbol_set_fix: first symbol id of each set.
var symFix = [5]int{0x001, 0x02E, 0x074, 0x0BA, 0x100}

// LoadSymbolSet = ovr35:003B — load 8X8D<n>.DAX block into set index,
// masked on colour 13.
func (e *Engine) LoadSymbolSet(set, block int) {
	if set < 0 || set > 4 {
		return
	}
	e.sym[set] = e.LoadPicMasked(e.set.file("8X8D"), block, 0xD)
	if e.sym[set] == nil {
		e.MessageRow24b(0xF, "Unable to load "+e.set.file("8x8D")+"-"+strconv.Itoa(block))
		e.Exit()
	}
}

// LoadPicMasked = 0464:0111 with the mask flag set.
func (e *Engine) LoadPicMasked(name string, id int, mask byte) *dax.Image {
	if e.Trace != nil {
		e.Trace("0464:0111", "LoadPic", name, id, 1, int(mask), "")
	}
	b := e.L.Block(name, id)
	if b == nil {
		return nil
	}
	return dax.ParseImage(b, true, mask)
}

// LoadPic = 0464:0111 (no masking).
func (e *Engine) LoadPic(name string, id int) *dax.Image {
	if e.Trace != nil {
		e.Trace("0464:0111", "LoadPic", name, id, 0, 0, "")
	}
	b := e.L.Block(name, id)
	if b == nil {
		return nil
	}
	return dax.ParseImage(b, false, 0)
}

// DrawPicture = 0464:1951 draw_picture(x, y, pic): blit frame at cell (x,y).
func (e *Engine) DrawPicture(x, y int, im *dax.Image, frame int) {
	if e.Trace != nil {
		e.Trace("0464:1951", "DrawPicture", x, y, picID(im, frame))
	}
	if im == nil || frame >= im.Count {
		return
	}
	d := im.Frame(frame)
	w := im.Width * 8
	e.S.Lock()
	defer e.S.Unlock()
	e.blit(x*8, y*8, w, im.Height, d, -1, false)
}

// blit writes h rows of w pixels at pixel (x0,y0) with ega.Set's rules (16
// draws as 0, 17..AmigaBase-1 are skipped, off-screen is clipped), leaving
// out pixels equal to mask; with page the drawing page gets the raw pixel.
func (e *Engine) blit(x0, y0, w, h int, d []byte, mask int, page bool) {
	cx0, cx1 := max(-x0, 0), min(ega.W-x0, w) // columns on screen
	for r := 0; r < h; r++ {
		y := y0 + r
		for c := 0; c < w; c++ {
			p := d[r*w+c]
			if int(p) == mask {
				continue
			}
			if page {
				e.pageSet(x0+c-8, y-8, p)
			}
			if c < cx0 || c >= cx1 || y < 0 || y >= ega.H {
				continue
			}
			if p == 16 {
				p = 0
			}
			if p < 16 || p >= ega.AmigaBase {
				e.S.Pix[y*ega.W+x0+c] = p
			}
		}
	}
}

// Put8x8Symbol = ovr35:015D Put8x8Symbol(x, y, id) without overlay: copy
// the symbol's frame from its set and draw it at cell (x,y).
func (e *Engine) Put8x8Symbol(x, y, id int) { e.put8x8(x, y, id, false, 0) }

// Put8x8SymbolMasked = ovr35:015D with the overlay flag (0464:08D4): the
// set's mask colour is left transparent.
func (e *Engine) Put8x8SymbolMasked(x, y, id int) { e.put8x8(x, y, id, true, 1) }

// Put8x8SymbolPage = ovr35:015D with the page flag and mode 0: drawn
// opaque into the drawing page (the area map).
func (e *Engine) Put8x8SymbolPage(x, y, id int) { e.put8x8(x, y, id, true, 0) }

func (e *Engine) put8x8(x, y, id int, masked bool, mode int) {
	set := -1
	switch {
	case id >= 0x001 && id <= 0x02D:
		set = 0
	case id >= 0x02E && id <= 0x073:
		set = 1
	case id >= 0x074 && id <= 0x0B9:
		set = 2
	case id >= 0x0BA && id <= 0x0FF:
		set = 3
	case id >= 0x100 && id <= 0x11E:
		set = 4
	}
	if set < 0 || e.sym[set] == nil {
		return
	}
	if masked {
		e.DrawPictureMasked(x, y, e.sym[set], id-symFix[set], mode)
	} else {
		e.DrawPicture(x, y, e.sym[set], id-symFix[set])
	}
}

// DrawPictureMasked = 0464:08D4 (x, y, frame, mode): like DrawPicture;
// with mode bit 0 and a masked picture the mask pixels are not drawn.
func (e *Engine) DrawPictureMasked(x, y int, im *dax.Image, frame, mode int) {
	if e.Trace != nil {
		px, py := x, y
		if e.pageDraw {
			px, py = x-1, y-1
		}
		e.Trace("0464:08D4", "DrawPictureMasked", px, py, frame, mode, picID(im, 0), "")
	}
	masked := mode&1 != 0
	if im == nil || frame >= im.Count {
		return
	}
	d := im.Frame(frame)
	w := im.Width * 8
	e.S.Lock()
	defer e.S.Unlock()
	m := -1
	if masked {
		m = im.Mask
	}
	e.blit(x*8, y*8, w, im.Height, d, m, e.pageDraw)
}

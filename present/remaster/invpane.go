// invpane.go holds the inventory's item lists: tabs, panes, prices, the item icons and the text and fill primitives they draw with.

package remaster

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"por/engine"
	"por/pal"
	"strconv"
	"strings"
)

// tabNames are the party inventory's tabs: all, then by engine.ItemKind.
var tabNames = [...]string{"All", "Weapons", "Armor", "Misc"}

// shown is the part of items the pane's tab shows.
func (pn *pane) shown(items []engine.InvItem) []engine.InvItem {
	kinds := [...]engine.ItemKind{0, engine.KindWeapon, engine.KindArmor, engine.KindMisc}
	if pn.tab == 0 {
		return items
	}
	var out []engine.InvItem
	for _, it := range items {
		if it.Kind == kinds[pn.tab] {
			out = append(out, it)
		}
	}
	return out
}

// drawTabs draws the pane's tabs in its title row from x.
func (inv *inventory) drawTabs(pn *pane, x int, busy bool) {
	for i, t := range tabNames {
		pn.tabs[i] = image.Rect(x-2, 6, x+8*len(t)+2, 18)
		switch {
		case i == pn.tab:
			inv.text(t, x, 8, white)
		case inv.hover.In(pn.tabs[i]) && !busy:
			inv.text(t, x, 8, green)
		default:
			inv.text(t, x, 8, gray)
		}
		x += 8*len(t) + 8
	}
}

// pane is a scrolled list of items: the party inventory or the shop's goods.
type pane struct {
	top, rows, n int             // the first line shown, the lines shown, the lines
	seen         int             // the highlight the list last scrolled to
	at           image.Rectangle // the lines' area
	scroll       image.Rectangle // the scroll bar
	tab          int             // its tab (tabNames)
	tabs         [len(tabNames)]image.Rectangle
}

// clamp keeps the lines shown inside the list.
func (pn *pane) clamp() { pn.top = max(0, min(pn.top, pn.n-pn.rows)) }

// drawPane draws items as lines of icon and name (and price) from x0 to x1,
// listTop to bottom, the highlighted one (sel) in blue, with a scroll bar
// when they do not fit.
func (inv *inventory) drawPane(pn *pane, items []engine.InvItem, sel int, ware, priced, busy bool, x0, x1, bottom int) {
	pn.n, pn.rows = len(items), max(1, (bottom-listTop)/rowH)
	for i, it := range items { // keep a newly highlighted line in sight (the wheel scrolls freely)
		if it.Index == sel && sel != pn.seen {
			pn.top = min(pn.top, i)
			pn.top = max(pn.top, i-pn.rows+1)
		}
	}
	pn.seen = sel
	pn.clamp()
	tr := x1 // the lines' right edge: the scroll bar's left
	if pn.n > pn.rows {
		tr -= 10
	}
	pn.at = image.Rect(x0, listTop, tr, listTop+pn.rows*rowH)
	pw := 0 // the prices' column
	if priced {
		for _, it := range items {
			pw = max(pw, 8*len(price(it.Price))+8)
		}
	}
	chars := (tr - x0 - rowH - 4 - pw) / 8
	for i := pn.top; i < pn.n && i < pn.top+pn.rows; i++ {
		it := &items[i]
		y := listTop + (i-pn.top)*rowH
		r := image.Rect(x0, y, tr, y+rowH)
		if it.Index == sel || inv.hover.In(r) && !busy {
			pal.SelectText(r, inv.fill)
		}
		inv.drawIcon(it, image.Rect(x0, y, x0+rowH, y+rowH))
		tc := green
		if it.Wrong {
			tc = red
		}
		inv.text(upTo(it.Name, chars), x0+rowH+4, y+(rowH-8)/2, tc)
		if priced {
			m := price(it.Price)
			inv.text(m, tr-4-8*len(m), y+(rowH-8)/2, col(14))
		}
		inv.lines = append(inv.lines, hit{r: r, item: *it, ware: ware, whole: len(strings.TrimSpace(it.Name)) <= chars})
	}
	if pn.n == 0 {
		inv.text("Nothing", x0+rowH+4, listTop+(rowH-8)/2, dark)
	}
	pn.scroll = image.Rectangle{}
	if pn.n > pn.rows {
		pn.scroll = image.Rect(x1-6, listTop, x1, listTop+pn.rows*rowH)
		scrollBar{pn.scroll, pn.n, pn.rows, pn.top}.draw(inv.fill)
	}
}

// price is every price and purse the screens show: "12g", with " 5c" only
// when there is copper.
func price(m engine.Money) string {
	s := strconv.Itoa(m.Gold) + "g"
	if m.Copper != 0 {
		s += " " + strconv.Itoa(m.Copper) + "c"
	}
	return s
}

// withGold is a price list's title with the party's gold after it.
func withGold(title string, gold int) string {
	return title + " (party " + price(engine.Money{Gold: gold}) + ")"
}

// wareAt is the shop's ware with Index i, nil if none.
func wareAt(in *engine.Inventory, i int) *engine.InvItem {
	for k := range in.Wares {
		if in.Wares[k].Index == i {
			return &in.Wares[k]
		}
	}
	return nil
}

func decodePNG(b []byte) image.Image {
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return im
}

// readIcons parses items.rec: records of "field: value" lines between
// blank lines, comments starting with #.
func readIcons(rec string) map[string]int {
	m := map[string]int{}
	name := ""
	s := bufio.NewScanner(strings.NewReader(rec))
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), ":")
		v = strings.TrimSpace(v)
		switch {
		case !ok || strings.HasPrefix(k, "#"):
		case k == "internal_name":
			name = v
		case k == "icon":
			if n, err := strconv.Atoi(v); err == nil && name != "" {
				m[name] = n
			}
		}
	}
	return m
}

func (inv *inventory) load() {
	if inv.doll != nil {
		return
	}
	inv.doll, inv.atlas = decodePNG(dollPNG), decodePNG(atlasPNG)
	inv.icons = readIcons(itemsRec)
}

// icon is the atlas cell of an item: the key "TT:word" for one of its
// name words (or a word of one), else "TT" for its type; -1 for none.
func (inv *inventory) icon(it *engine.InvItem) int {
	t := fmt.Sprintf("%02X", it.Type)
	for _, w := range it.Words {
		if n, ok := inv.icons[t+":"+w]; ok {
			return n
		}
		for _, f := range strings.Fields(w) {
			if n, ok := inv.icons[t+":"+f]; ok {
				return n
			}
		}
	}
	if n, ok := inv.icons[t]; ok {
		return n
	}
	return -1
}

// iconImage is the item's atlas cell (nil: none).
func (inv *inventory) iconImage(it *engine.InvItem) image.Image {
	inv.load()
	n := inv.icon(it)
	if n < 0 {
		return nil
	}
	at := image.Pt(n%atlasCols*atlasCell, n/atlasCols*atlasCell)
	return inv.atlas.(interface {
		SubImage(image.Rectangle) image.Image
	}).SubImage(image.Rectangle{at, at.Add(image.Pt(atlasCell, atlasCell))})
}

// drawIcon draws the item's atlas cell, unscaled, in the middle of r (cut to r).
func (inv *inventory) drawIcon(it *engine.InvItem, r image.Rectangle) {
	n := inv.icon(it)
	if n < 0 {
		inv.text("?", r.Min.X+r.Dx()/2-4, r.Min.Y+r.Dy()/2-4, gray)
		return
	}
	at := r.Min.Add(image.Pt(r.Dx()-atlasCell, r.Dy()-atlasCell).Div(2))
	cell := image.Rectangle{at, at.Add(image.Pt(atlasCell, atlasCell))}.Intersect(r)
	draw.Draw(inv.px, cell, inv.atlas, image.Pt(n%atlasCols*atlasCell, n/atlasCols*atlasCell).Add(cell.Min.Sub(at)), draw.Over)
}

// text draws s in the game's font at (x, y).
func (inv *inventory) text(s string, x, y int, c color.RGBA) { drawText(inv.px, inv.glyph, s, x, y, c) }

func drawText(dst *image.RGBA, glyph func(byte) [8]byte, s string, x, y int, c color.RGBA) {
	for i := 0; i < len(s); i++ {
		g := glyph(s[i])
		for gy := 0; gy < 8; gy++ {
			for b := 0; b < 8; b++ {
				if g[gy]&(0x80>>b) != 0 {
					dst.SetRGBA(x+i*8+b, y+gy, c)
				}
			}
		}
	}
}

func (inv *inventory) fill(r image.Rectangle, c color.RGBA) {
	draw.Draw(inv.px, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// upTo is s in at most n characters.
func upTo(s string, n int) string { return s[:min(len(s), max(0, n))] }

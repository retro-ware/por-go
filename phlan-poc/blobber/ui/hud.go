package ui

import (
	"fmt"
	"image"

	"blobber"
	"blobber/font"
	"blobber/party"
	"blobber/sprites"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	hitFrames      = 70                   // how long a damage icon stays on a portrait
	portraitX, pY0 = 8, 8                 // just inside the frame's 8 px border
	frameH         = 8 + PortraitSize + 8 // the frame holds only the portrait
	barY           = frameH + 1           // HP and SP bars below it, as wide as the frame
	cardW, cardH   = PortraitSize + 2*portraitX, barY + 2*11
	cardGap        = 2 // between columns
	rowGap         = 8 // between a card's bars and the next portrait
)

type hitMark struct {
	kind string
	n, t int
}

var (
	frameNo int
	hits    = map[int]hitMark{} // 0-based character -> last hit
	cards   []*ebiten.Image
)

// Active is the character whose combat action is being asked for; its frame is outlined in gold
// while a fight is on. The host sets it.
var Active *party.Character

// Hit shows a damage icon of the given kind over character who's (1-based) portrait.
// It matches party.Party.OnHit.
func Hit(who, n int, kind string) {
	hits[who-1] = hitMark{kind, n, frameNo}
	if kind == "physical" {
		Bleed(who-1, n)
	}
}

// card is character i's rectangle on screen, in device pixels, for a party of n: a column at the
// right edge below the minimap, two columns when one does not fit, then shrunk until they do.
func card(i, n int) image.Rectangle {
	mm := blobber.MinimapRect()
	top, avail := mm.Max.Y+4, blobber.H-mm.Max.Y-8
	k, cols := K(cw, ch), 1
	for k > 1 || cols == 1 {
		if rows := (n + cols - 1) / cols; rows*(cardH+rowGap)*k <= avail {
			break
		}
		if cols == 1 {
			cols = 2
		} else {
			k--
		}
	}
	w, h := (cardW+cardGap)*k, (cardH+rowGap)*k
	right := mm.Max.X // flush with the minimap's right edge
	x := right - (cols-i%cols)*w + cardGap*k
	y := top + i/cols*h
	return image.Rect(x, y, x+cardW*k, y+cardH*k)
}

// PartyAt returns the 0-based character whose card contains screen point x,y, or -1.
func PartyAt(p *party.Party, x, y int) int {
	for i := range p.Chars {
		if image.Pt(x, y).In(card(i, len(p.Chars))) {
			return i
		}
	}
	return -1
}

// DrawParty draws one framed card per character down the right edge: portrait, HP and SP bars below it,
// and for a moment after a hit the damage-type icon and amount over the portrait.
func DrawParty(screen *ebiten.Image, p *party.Party) {
	frameNo++
	k := K(cw, ch)
	for len(cards) < len(p.Chars) {
		cards = append(cards, ebiten.NewImage(cardW, cardH))
	}
	for i, c := range p.Chars {
		d := cards[i]
		d.Clear()
		drawCard(d, i, c)
		r := card(i, len(p.Chars))
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(k), float64(k))
		op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
		screen.DrawImage(d, op)
	}
	drawSplats(screen, func(i int) (float64, float64, float64) {
		r := card(i, len(p.Chars))
		return float64(r.Min.X + (portraitX+PortraitSize/2)*k), float64(r.Min.Y + (pY0+PortraitSize/2)*k), float64(k)
	})
}

func drawCard(d *ebiten.Image, i int, c *party.Character) {
	if c == Active && blobber.Fighting() {
		GoldFrame(d, rc(0, 0, cardW, frameH))
	} else {
		DarkFrame(d, rc(0, 0, cardW, frameH))
	}
	img := Portrait(c)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(portraitX, pY0)
	dead := c.HP <= 0 || c.Has["condition:dead"]
	if dead {
		op.ColorScale.Scale(0.35, 0.35, 0.35, 1)
	}
	h, hit := hits[i]
	age := frameNo - h.t
	hit = hit && age < hitFrames
	if hit && age < 10 {
		op.ColorScale.Scale(1, 0.5, 0.5, 1) // brief red flash
	}
	d.DrawImage(img, op)

	cond := ""
	for _, k := range party.Conditions {
		if c.Has["condition:"+k] {
			cond = k
			break
		}
	}
	if dead {
		cond = "dead"
	}
	if cond != "" { // no room for the name: only a condition is spelled out, over the portrait
		Print(d, cond, portraitX+1, pY0+PortraitSize-12, gold)
	}
	hp := GreenFill
	if c.HP*4 < c.MaxHP {
		hp = RedFill
	}
	Bar(d, -5, barY, cardW+10, c.HP, c.MaxHP, hp) // the caps' art is inset 5 px; this lines it up with the frame
	if c.MaxSP > 0 {
		Bar(d, -5, barY+11, cardW+10, c.SP, c.MaxSP, BlueFill)
	}
	if hit {
		drawHit(d, h, age)
	}
}

// drawHit puts the kind's icon over the portrait, rising and fading out at the end.
func drawHit(d *ebiten.Image, h hitMark, age int) {
	ic := sprites.Get("hit_" + h.kind)
	if ic == nil {
		ic = sprites.Get("hit_physical")
	}
	b := ic.Bounds()
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(float64(portraitX+PortraitSize/2-b.Dx()), float64(pY0+PortraitSize/2-b.Dy()-age/12))
	if fade := hitFrames - age; fade < 20 {
		op.ColorScale.ScaleAlpha(float32(fade) / 20)
	}
	d.DrawImage(ic, op)
	if h.kind != "dead" {
		font.Print(d, fmt.Sprintf("-%d", h.n), portraitX+PortraitSize-14, pY0+PortraitSize-16)
	}
}

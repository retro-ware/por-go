package ui

import (
	"blobber"
	"image"
	"strings"

	"blobber/font"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Talk is an NPC speaking: portrait, name and wrapped text, with Continue or, when Ask, Yes / No.
// Done gets the answer (true for Continue) once the window closes.
type Talk struct {
	Who  string // NPC id from the registry; unknown ids show the id as name and no portrait
	Text string
	Ask  bool
	Done func(yes bool)

	page, hover int
	sel         int // highlighted button
	canvas
}

const (
	talkTextX, talkTextW = 150, 296
	talkLines            = 7
)

var talkBox = rc(16, 150, 448, 206)

// buttons: label and box, right-aligned at the bottom of the window.
func (t *Talk) buttons() ([]string, []image.Rectangle) {
	labels := []string{"Continue"}
	if t.Ask && t.last() {
		labels = []string{"Yes", "No"}
	}
	var boxes []image.Rectangle
	for i := range labels {
		boxes = append(boxes, rc(talkBox.Max.X-20-(len(labels)-i)*84, talkBox.Max.Y-34, 80, 19))
	}
	return labels, boxes
}

func (t *Talk) lines() []string { return wrap(t.Text, talkTextW) }
func (t *Talk) last() bool      { return (t.page+1)*talkLines >= len(t.lines()) }

// answer moves on: the next page of a long speech, else closes with yes.
func (t *Talk) answer(yes bool) bool {
	if !t.last() {
		t.page++
		return true
	}
	if t.Done != nil {
		t.Done(yes)
	}
	return false
}

func (t *Talk) Update() bool {
	p := blobber.KeyJustPressed
	ask := t.Ask && t.last()
	labels, boxes := t.buttons()
	if p(ebiten.KeyLeft) || p(ebiten.KeyUp) || p(ebiten.KeyRight) || p(ebiten.KeyDown) {
		t.sel = (t.sel + 1) % len(labels)
	}
	t.sel %= len(labels)
	switch {
	case p(ebiten.KeyEnter) || p(ebiten.KeySpace):
		return t.answer(labels[t.sel] != "No")
	case p(ebiten.KeyEscape):
		return t.answer(!ask)
	}
	x, y := t.mouse()
	t.hover = -1
	for i, b := range boxes {
		if in(x, y, b) {
			t.hover, t.sel = i, i
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				return t.answer(labels[i] != "No")
			}
		}
	}
	return true
}

func (t *Talk) Draw(screen *ebiten.Image) {
	d := t.begin()
	Frame(d, talkBox)
	npc, ok := NPCs[t.Who]
	if !ok {
		npc.Name = t.Who
	}
	DarkFrame(d, rc(32, 166, 2*PortraitSize+16, 2*PortraitSize+16)) // 8 px border
	if img := PortraitID(npc.Portrait); img != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(40, 174)
		d.DrawImage(img, op)
	}
	Print(d, npc.Name, talkTextX, 168, gold)
	ls := t.lines()
	for i := t.page * talkLines; i < min(len(ls), (t.page+1)*talkLines); i++ {
		Print(d, ls[i], talkTextX, 190+(i-t.page*talkLines)*16, light)
	}
	labels, boxes := t.buttons()
	for i, b := range boxes {
		s := Normal
		if i == t.sel {
			s = Hover
		}
		Button(d, b.Min.X, b.Min.Y, b.Dx(), labels[i], s)
	}
	t.present(screen)
}

// wrap breaks s into lines at most w pixels wide; "|" forces a break.
func wrap(s string, w int) []string {
	var out []string
	for _, para := range strings.Split(s, "|") {
		line := ""
		for _, word := range strings.Fields(para) {
			if line != "" && font.Width(line+" "+word) > w {
				out = append(out, line)
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		out = append(out, line)
	}
	return out
}

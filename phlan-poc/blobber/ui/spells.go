package ui

import (
	"blobber"
	"fmt"
	"image"
	"strings"

	"blobber/font"
	"blobber/party"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Spellbook lists the caster's spells; picking one casts it and closes the book so the effect shows.
type Spellbook struct {
	P            *party.Party
	char         int
	msg          string
	hover, hovBt int
	sel          int // the highlighted spell
	canvas
}

var spellBox = rc(24, 40, 432, 290)

const spellY = 96

// NewSpellbook opens on the first character who knows a spell.
func NewSpellbook(p *party.Party) *Spellbook {
	b := &Spellbook{P: p, hover: -1, hovBt: -1}
	for i, c := range p.Chars {
		if len(b.known(c)) > 0 {
			b.char = i
			break
		}
	}
	return b
}

func (b *Spellbook) known(c *party.Character) []party.Spell {
	var out []party.Spell
	for _, s := range party.Spells {
		if c.Knows(s.ID) {
			out = append(out, s)
		}
	}
	return out
}

func spellRow(i int) image.Rectangle {
	return rc(spellBox.Min.X+20, spellY+i*24-3, spellBox.Dx()-40, 22)
}

var exitBtn = rc(spellBox.Max.X-104, spellBox.Max.Y-40, 80, 19)

// cast casts spell i; false (closing the book) when it worked.
func (b *Spellbook) cast(i int) bool {
	ks := b.known(b.P.Chars[b.char])
	if i >= len(ks) {
		return true
	}
	m, err := b.P.Cast(b.P.Chars[b.char], ks[i].ID)
	if err != nil {
		b.msg = err.Error()
		return true
	}
	b.msg = m
	return false
}

func (b *Spellbook) Update() bool {
	p := blobber.KeyJustPressed
	if p(ebiten.KeyEscape) || p(ebiten.KeyC) {
		return false
	}
	if p(ebiten.KeyTab) || p(ebiten.KeyRight) || p(ebiten.KeyPageDown) {
		b.char, b.sel = (b.char+1)%len(b.P.Chars), 0
	}
	if p(ebiten.KeyLeft) || p(ebiten.KeyPageUp) {
		b.char, b.sel = (b.char+len(b.P.Chars)-1)%len(b.P.Chars), 0
	}
	if n := len(b.known(b.P.Chars[b.char])); n > 0 { // up/down pick, Enter casts
		if blobber.KeyRepeated(ebiten.KeyUp) {
			b.sel = (b.sel + n - 1) % n
		}
		if blobber.KeyRepeated(ebiten.KeyDown) {
			b.sel = (b.sel + 1) % n
		}
		if p(ebiten.KeyEnter) || p(ebiten.KeySpace) {
			return b.cast(b.sel % n)
		}
	}
	for i := range 9 {
		if p(ebiten.KeyF1+ebiten.Key(i)) && i < len(b.P.Chars) {
			b.char = i
		}
	}
	x, y := b.mouse()
	sx, sy := ebiten.CursorPosition()
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if i := PartyAt(b.P, sx, sy); i >= 0 && left {
		b.char, b.msg = i, ""
	}
	b.hover, b.hovBt = -1, -1
	for i := range b.known(b.P.Chars[b.char]) {
		if in(x, y, spellRow(i)) {
			b.hover = i
			if left {
				return b.cast(i)
			}
		}
	}
	if n := len(b.known(b.P.Chars[b.char])); b.hover < 0 && n > 0 { // no mouse over a spell: the cursor's
		b.hover = b.sel % n
	} else if b.hover >= 0 {
		b.sel = b.hover
	}
	if in(x, y, exitBtn) {
		b.hovBt = 0
		if left {
			return false
		}
	}
	return true
}

func (b *Spellbook) Draw(screen *ebiten.Image) {
	d := b.begin()
	Frame(d, spellBox)
	c := b.P.Chars[b.char]
	Title(d, "Spells of "+c.Name+" the "+strings.Title(c.Class), cw/2, spellBox.Min.Y+14)
	sp := fmt.Sprintf("SP %d / %d", c.SP, c.MaxSP)
	Print(d, sp, spellBox.Max.X-24-font.Width(sp), spellY-20, gold)
	ks := b.known(c)
	switch {
	case !c.CanLearn():
		Print(d, c.Name+" knows no magic.", spellBox.Min.X+24, spellY, tan)
	case len(ks) == 0:
		Print(d, "No spells yet. The Mage Guild teaches them.", spellBox.Min.X+24, spellY, tan)
	}
	for i, s := range ks {
		r := spellRow(i)
		if i == b.hover {
			fillRect(d, r, selFill)
		}
		col := light
		if c.SP < s.SP {
			col = dim
		}
		Print(d, fmt.Sprintf("%d) %s", i+1, s.Name), r.Min.X+4, r.Min.Y+4, col)
		Print(d, fmt.Sprintf("%d SP", s.SP), r.Min.X+112, r.Min.Y+4, gold)
		Print(d, s.Desc, r.Min.X+160, r.Min.Y+4, tan)
	}
	Print(d, b.msg, spellBox.Min.X+24, spellBox.Max.Y-62, light)
	s := Normal
	if b.hovBt == 0 {
		s = Hover
	}
	Button(d, exitBtn.Min.X, exitBtn.Min.Y, exitBtn.Dx(), "Exit", s)
	b.present(screen)
}

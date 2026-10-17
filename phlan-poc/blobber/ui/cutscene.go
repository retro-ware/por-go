package ui

import (
	"blobber"
	"bufio"
	"fmt"
	"io"
	"strings"

	"blobber/font"
	"blobber/sprites"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Page is one screen of a cutscene: an optional big sprite and some lines of text.
type Page struct {
	Picture string
	Lines   []string
}

// Cutscene is a full-screen slideshow. Space, Enter, F or a click turns the page; Esc skips it.
type Cutscene struct {
	Pages []Page
	i     int
	Done  func() // optional, called when it closes
	canvas
}

// ParseCutscene reads pages separated by blank lines. "@name" sets the page's sprite, "#" lines are comments.
func ParseCutscene(r io.Reader) []Page {
	var pages []Page
	cur := Page{}
	flush := func() {
		if cur.Picture != "" || len(cur.Lines) > 0 {
			pages = append(pages, cur)
		}
		cur = Page{}
	}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		switch {
		case l == "":
			flush()
		case strings.HasPrefix(l, "#"):
		case strings.HasPrefix(l, "@"):
			cur.Picture = l[1:]
		default:
			cur.Lines = append(cur.Lines, l)
		}
	}
	flush()
	return pages
}

func (c *Cutscene) Update() bool {
	p := blobber.KeyJustPressed
	if p(ebiten.KeyEscape) {
		c.i = len(c.Pages)
	} else if p(ebiten.KeySpace) || p(ebiten.KeyEnter) || p(ebiten.KeyF) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		c.i++
	}
	if c.i >= len(c.Pages) {
		if c.Done != nil {
			c.Done()
		}
		return false
	}
	return true
}

func (c *Cutscene) Draw(screen *ebiten.Image) {
	screen.Fill(ink)
	if c.i >= len(c.Pages) {
		return
	}
	pg := c.Pages[c.i]
	d := c.begin()
	Parchment(d, rc(40, 20, cw-80, ch-40))
	y := 120
	if img := sprites.Get(pg.Picture); img != nil {
		k := 96 / max(img.Bounds().Dx(), img.Bounds().Dy())
		b := img.Bounds()
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(k), float64(k))
		op.GeoM.Translate(float64((cw-b.Dx()*k)/2), 44)
		d.DrawImage(img, op)
		y = 44 + b.Dy()*k + 20
	}
	for _, l := range pg.Lines {
		Print(d, l, (cw-font.Width(l))/2, y, ink)
		y += 16
	}
	foot := fmt.Sprintf("%d / %d    Space: next   Esc: skip", c.i+1, len(c.Pages))
	Print(d, foot, (cw-font.Width(foot))/2, ch-48, charcoal)
	c.present(screen)
}

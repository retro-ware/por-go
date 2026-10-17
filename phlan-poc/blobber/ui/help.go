package ui

import (
	"blobber"
	"blobber/font"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// HelpScreen is the "?" overlay: a short introduction and every hotkey, grouped.
type HelpScreen struct{ canvas }

const helpIntro = "Lead your party through the dungeon: search walls, talk to people, " +
	"spend gold in town. The mouse works in every window."

// helpKeys: sections in two columns.
var helpKeys = [2][]struct {
	title string
	keys  [][2]string
}{{
	{"Moving", [][2]string{{"W / Up", "step forward"}, {"S / Down", "step back"}, {"A  D", "step left, right"}, {"Q  E", "turn left, right"}}},
	{"Acting", [][2]string{{"F", "search, take, use"}, {"I", "inventory"}, {"C", "cast a spell"}, {"J", "journal"}, {"M", "map"}, {"R", "rest"}, {"Space", "wait a turn"}}},
	{"Game", [][2]string{{"Esc", "menu, quit"}, {"F5  F9", "save, load"}, {"G", "god mode (debug)"}, {"?", "this help"}}},
}, {
	{"Party", [][2]string{{"F1 - F6", "pick a character"}, {"", "in menus and shops"}}},
	{"Windows", [][2]string{{"Enter", "choose, continue"}, {"Up  Down", "select"}, {"Y  N", "yes, no"}, {"Esc", "close"}, {"Tab", "next character"}}},
	{"Debug", [][2]string{{"P", "party sheet"}, {"F12", "texture atlas"}, {"K", "kill monsters ahead"}, {"N", "spawn a monster"}}},
}}

func (h *HelpScreen) Update() bool {
	p := blobber.KeyJustPressed
	return !(p(ebiten.KeyEscape) || p(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft))
}

func (h *HelpScreen) Draw(screen *ebiten.Image) {
	d := h.begin()
	Frame(d, rc(0, 0, cw, ch))
	Title(d, "HELP", cw/2, 14)
	for i, l := range wrap(helpIntro, cw-56) {
		Print(d, l, 28, 42+i*15, light)
	}
	for c, sections := range helpKeys {
		x, y := 28+c*226, 84
		for _, sec := range sections {
			Print(d, sec.title, x, y, gold)
			y += 18
			for _, k := range sec.keys {
				if k[0] != "" {
					DarkFrame(d, rc(x-4, y-3, 80, 17))
					Print(d, k[0], x+2, y, gold)
				}
				Print(d, k[1], x+84, y, light)
				y += 17
			}
			y += 8
		}
	}
	t := "Press ? or Esc to close"
	Print(d, t, cw/2-font.Width(t)/2, ch-30, tan)
	h.present(screen)
}

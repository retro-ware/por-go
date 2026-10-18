package engine

// Title = ovr02:0626 title_screen.
func (e *Engine) Title() {
	e.Music(1) // the Amiga intro, 0x2B580
	defer e.Music(0)
	e.DrawPicture(0, 0, e.LoadPic("TITLE", 1), 0)
	e.WaitSeconds(5)
	if !e.Cfg.FullIntro {
		return
	}
	e.DrawPicture(0, 0, e.LoadPic("TITLE", 2), 0)
	end := e.Ticks() + 500
	var acc int
	for {
		if e.Cfg.Graphics == 'E' { // [4A1E] == 1: lightning on palette entry 1
			acc += e.Random(4)
			if acc > 50000 {
				acc = 0
				n := e.Random(3) + 1
				for i := 0; i < n; i++ {
					for _, f := range [...]struct{ c, ms int }{{9, 150}, {11, 100}, {15, 50}, {11, 100}, {9, 150}} {
						e.SetPaletteColor(f.c, 1)
						e.Delay(f.ms)
					}
					e.SetPaletteColor(1, 1)
				}
			}
		}
		if e.keyWaiting() || e.Ticks() >= end {
			break
		}
		e.idle()
	}
	e.ClearKeyboard()
	e.Credits()
}

// Credits = ovr02:0206.
func (e *Engine) Credits() {
	e.DrawBoxTitled(1, 1, winRight, 3, 0, colBlack, "")
	e.DrawBoxTitled(1, 5, winRight, 23, 0, colBlack, "")
	for _, t := range creditLines {
		e.DisplayString(t.x, t.y, t.c, t.s)
	}
	e.WaitSeconds(10)
}

var creditLines = []struct {
	x, y, c int
	s       string
}{
	{5, 1, 0xa, "scenario created by:"},
	{0x1a, 1, 0xe, "tsr, inc."},
	{0xf, 2, 0xa, "jim ward"},
	{2, 3, 0xa, "david cook,steve winter,mike breault"},
	{1, 5, 0xa, "game created by:"},
	{0x12, 5, 0xe, "ssi special projects"},
	{0xe, 6, 0xb, "version 1.3"},
	{2, 7, 0xe, "programming:"},
	{0x14, 7, 0xe, "original"},
	{2, 8, 0xb, "scot bayless"},
	{0x14, 8, 0xe, "programming:"},
	{2, 9, 0xb, "brad myers"},
	{0x14, 9, 0xb, "keith brors"},
	{2, 0xa, 0xb, "russ brown"},
	{0x14, 0xa, 0xb, "brad myers"},
	{2, 0xb, 0xb, "ted greer"},
	{2, 0xd, 0xe, "graphic arts:"},
	{0x14, 0xd, 0xe, "encounter coding:"},
	{2, 0xe, 0xb, "tom wahl"},
	{0x14, 0xe, 0xb, "paul murray"},
	{2, 0xf, 0xb, "fred butts"},
	{0x14, 0xf, 0xb, "russ brown"},
	{2, 0x10, 0xb, "darla marasco"},
	{0x14, 0x10, 0xb, "victor penman"},
	{2, 0x11, 0xb, "susan halbleib"},
	{0x14, 0x11, 0xb, "dave shelley"},
	{2, 0x13, 0xe, "project manager"},
	{0x14, 0x13, 0xe, "developer:"},
	{2, 0x14, 0xb, "victor penman"},
	{0x14, 0x14, 0xb, "george mac donald"},
	{2, 0x16, 0xe, "testing:"},
	{0xb, 0x16, 0xb, "joel billings,steve salyer"},
	{2, 0x17, 0xb, "james kucera,robert daly,rick white"},
}

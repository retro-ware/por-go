package ui

// The skin: Hypnobius' "Dark Ages UI" tilesheet (assets/darkages.png, 32x32 tiles). Its licence
// allows use in games but not redistributing the pack, so keep the PNG out of public repos.
//
// Every window is drawn into a cw x ch canvas in sheet pixels and scaled up by a whole factor.

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"

	"blobber"
	"blobber/font"

	"github.com/hajimehoshi/ebiten/v2"
)

//go:embed assets/darkages.png
var sheetPNG []byte

var sheet = func() *ebiten.Image {
	img, err := png.Decode(bytes.NewReader(sheetPNG))
	if err != nil {
		panic(err)
	}
	return ebiten.NewImageFromImage(img)
}()

// Palette from the pack's README.
var (
	gold     = color.RGBA{0xdc, 0xc4, 0x7c, 255}
	tan      = color.RGBA{0x9f, 0x88, 0x73, 255}
	ink      = color.RGBA{0x15, 0x20, 0x2f, 255}
	light    = color.RGBA{0xe8, 0xe0, 0xc8, 255}
	charcoal = color.RGBA{0x2e, 0x32, 0x2a, 255}
	shade    = color.RGBA{0, 0, 0, 255}
)

// Canvas size of every full window, in sheet pixels.
const cw, ch = 480, 360

// Button and slot states.
const (
	Normal = iota
	Hover  // gold, crowned
	Off    // greyed
)

func rc(x, y, w, h int) image.Rectangle { return image.Rect(x, y, x+w, y+h) }

// blit draws sheet region src stretched onto dst region d.
func blit(dst *ebiten.Image, src, d image.Rectangle) {
	if src.Empty() || d.Empty() {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(d.Dx())/float64(src.Dx()), float64(d.Dy())/float64(src.Dy()))
	op.GeoM.Translate(float64(d.Min.X), float64(d.Min.Y))
	dst.DrawImage(sheet.SubImage(src).(*ebiten.Image), op)
}

// tile repeats src across d (clipping the last copy) instead of stretching it.
func tile(dst *ebiten.Image, src, d image.Rectangle) {
	for y := d.Min.Y; y < d.Max.Y; y += src.Dy() {
		for x := d.Min.X; x < d.Max.X; x += src.Dx() {
			w, h := min(src.Dx(), d.Max.X-x), min(src.Dy(), d.Max.Y-y)
			blit(dst, rc(src.Min.X, src.Min.Y, w, h), rc(x, y, w, h))
		}
	}
}

// nine draws src as a nine-slice with border b onto d: fixed corners, tiled edges, stretched middle.
func nine(dst *ebiten.Image, src image.Rectangle, b int, d image.Rectangle) {
	sx := [4]int{src.Min.X, src.Min.X + b, src.Max.X - b, src.Max.X}
	sy := [4]int{src.Min.Y, src.Min.Y + b, src.Max.Y - b, src.Max.Y}
	dx := [4]int{d.Min.X, d.Min.X + b, d.Max.X - b, d.Max.X}
	dy := [4]int{d.Min.Y, d.Min.Y + b, d.Max.Y - b, d.Max.Y}
	for j := range 3 {
		for i := range 3 {
			s, t := image.Rect(sx[i], sy[j], sx[i+1], sy[j+1]), image.Rect(dx[i], dy[j], dx[i+1], dy[j+1])
			if i == 1 && j == 1 {
				blit(dst, s, t)
			} else {
				tile(dst, s, t)
			}
		}
	}
}

// hslice stretches src to width w keeping caps of width cap at each end, and, if mid > 0,
// a fixed-width centre piece (a crown) that must not be stretched.
func hslice(dst *ebiten.Image, src image.Rectangle, cap, mid, x, y, w int) {
	h := src.Dy()
	sy := src.Min.Y
	blit(dst, rc(src.Min.X, sy, cap, h), rc(x, y, cap, h))
	blit(dst, rc(src.Max.X-cap, sy, cap, h), rc(x+w-cap, y, cap, h))
	if mid == 0 {
		blit(dst, rc(src.Min.X+cap, sy, src.Dx()-2*cap, h), rc(x+cap, y, w-2*cap, h))
		return
	}
	scx, dcx := src.Min.X+src.Dx()/2-mid/2, x+w/2-mid/2
	blit(dst, rc(scx, sy, mid, h), rc(dcx, y, mid, h))
	blit(dst, image.Rect(src.Min.X+cap, sy, scx, sy+h), image.Rect(x+cap, y, dcx, y+h))
	blit(dst, image.Rect(scx+mid, sy, src.Max.X-cap, sy+h), image.Rect(dcx+mid, y, x+w-cap, y+h))
}

// ---------- pieces ----------

// Frame is the ornate gold window frame.
func Frame(dst *ebiten.Image, d image.Rectangle) { nine(dst, rc(0, 0, 96, 96), 32, d) }

// DarkFrame is the plain dark knotwork frame, for panels inside windows and the HUD.
func DarkFrame(dst *ebiten.Image, d image.Rectangle) { nine(dst, rc(210, 18, 60, 60), 14, d) }

// GoldFrame is the gold twin of DarkFrame (the big gold knotwork panel), for highlighting.
func GoldFrame(dst *ebiten.Image, d image.Rectangle) { nine(dst, rc(2, 2, 92, 92), 16, d) }

// Parchment is the scroll paper used for signs and cutscenes; text on it is ink coloured.
func Parchment(dst *ebiten.Image, d image.Rectangle) { nine(dst, rc(96, 0, 96, 96), 16, d) }

// Button draws a button of width w whose 19 px body starts at y (the crown of a Hover button rises 8 px above it)
// and centres label on it.
func Button(dst *ebiten.Image, x, y, w int, label string, state int) {
	mid := 0
	if state == Hover {
		mid = 28
	}
	hslice(dst, rc([]int{0, 128, 64}[state], 96, 64, 32), 12, mid, x, y-8, w)
	col := light
	if state == Off {
		col = tan
	}
	Print(dst, label, x+(w-font.Width(label))/2, y+3, col)
}

// Tab draws a tab of width w whose 15 px body starts at y.
func Tab(dst *ebiten.Image, x, y, w int, label string, state int) {
	mid := 0
	if state == Hover {
		mid = 16
	}
	hslice(dst, rc([]int{288, 352, 320}[state], 96, 32, 32), 6, mid, x, y-10, w)
	col := light
	if state != Hover {
		col = tan
	}
	Print(dst, label, x+(w-font.Width(label))/2, y+1, col)
}

// Slot draws a 32x32 item slot cell at x,y.
func Slot(dst *ebiten.Image, x, y, state int) {
	blit(dst, rc([]int{192, 256, 224}[state], 96, 32, 32), rc(x, y, 32, 32))
}

// Bar kinds for StatBar.
var (
	RedFill   = rc(199, 176, 82, 4)
	BlueFill  = rc(296, 144, 82, 4)
	GreenFill = rc(295, 176, 82, 4)
)

// StatBar draws the crowned bar frame, width w, with its top-left tile corner at x,y, filled to v/maxV.
// The visible bar body runs from y+7 to y+22.
func StatBar(dst *ebiten.Image, x, y, w, v, maxV int, fill image.Rectangle) {
	hslice(dst, rc(192, 128, 96, 32), 10, 32, x, y, w)
	if maxV > 0 && v > 0 {
		fw := (w - 14) * min(v, maxV) / maxV
		blit(dst, fill, rc(x+7, y+16, max(1, fw), fill.Dy()))
	}
}

// Icons from the sheet, 32x32 cells.
var (
	CloseIcon = rc(64, 224, 32, 32)
	WarnIcon  = rc(96, 224, 32, 32)
	LeftIcon  = rc(0, 256, 32, 32)
	RightIcon = rc(96, 256, 32, 32)
)

func Icon(dst *ebiten.Image, src image.Rectangle, x, y int) {
	blit(dst, src, rc(x, y, src.Dx(), src.Dy()))
}

// Title prints s centred at y between two flourishes, like the pack's window headers.
func Title(dst *ebiten.Image, s string, cx, y int) {
	blit(dst, rc(288, 256, 96, 32), rc(cx-96, y-10, 192, 32)) // divider with diamond, stretched wide
	Print(dst, s, cx-3*len(s), y+12, gold)
}

// ---------- text ----------

// Print draws text in the game font.
func Print(dst *ebiten.Image, s string, x, y int, col color.Color) { font.Draw(dst, s, x, y, col) }

// ---------- canvas ----------

// canvas is a window's offscreen cw x ch image, shown centred at the largest whole scale that fits.
type canvas struct {
	img       *ebiten.Image
	k, ox, oy int
}

// begin returns the cleared canvas.
func (c *canvas) begin() *ebiten.Image {
	if c.img == nil {
		c.img = ebiten.NewImage(cw, ch)
	}
	c.img.Clear()
	return c.img
}

func (c *canvas) present(screen *ebiten.Image) {
	c.k = K(cw, ch)
	c.ox, c.oy = (blobber.W-cw*c.k)/2, max(0, (blobber.H-ch*c.k)/2)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(c.k), float64(c.k))
	op.GeoM.Translate(float64(c.ox), float64(c.oy))
	screen.DrawImage(c.img, op)
}

// mouse is the cursor in canvas pixels (-1,-1 before the first draw).
func (c *canvas) mouse() (int, int) {
	if c.k == 0 {
		return -1, -1
	}
	x, y := ebiten.CursorPosition()
	return (x - c.ox) / c.k, (y - c.oy) / c.k
}

// K is the whole scale at which a w x h canvas fits the window.
func K(w, h int) int { return max(1, min(blobber.W/w, blobber.H/h)) }

func in(x, y int, r image.Rectangle) bool { return image.Pt(x, y).In(r) }

// Bar is a StatBar without the crown, 12 px tall, for tight spots like the HUD.
func Bar(dst *ebiten.Image, x, y, w, v, maxV int, fill image.Rectangle) {
	const sy, h = 141, 11 // rows below the crown
	blit(dst, rc(192, sy, 10, h), rc(x, y, 10, h))
	blit(dst, rc(278, sy, 10, h), rc(x+w-10, y, 10, h))
	blit(dst, rc(205, sy, 12, h), rc(x+10, y, w-20, h))
	if maxV > 0 && v > 0 {
		fw := (w - 14) * min(v, maxV) / maxV
		blit(dst, fill, rc(x+7, y+3, max(1, fw), fill.Dy()))
	}
}

// ---------- the engine's own panels ----------

// Install makes the engine draw its prompt, settings menu, debug panel, signs and minimap with this skin.
func Install() {
	blobber.Window = Window
	blobber.SignImage = SignImage
	blobber.BoardImage = BoardImage
	blobber.Panel = Panel
	blobber.PanelBorder = func() int { return 14 * K(cw, ch) } // DarkFrame's 14 px border
	blobber.Help = &HelpScreen{}
}

var winCanvas = ebiten.NewImage(cw*2, ch*2)

// Window draws a framed text window. sel highlights a line (-1 for none). bottom puts it above the HUD
// (top left during melee), otherwise it is centred.
func Window(screen *ebiten.Image, title string, lines []string, sel int, bottom bool) {
	w := font.Width(title) + 120
	for _, l := range lines {
		w = max(w, font.Width(l)+48)
	}
	top := 24
	if title != "" {
		top = 44
	}
	h := top + 16*len(lines) + 24
	k := K(cw, ch)
	w, h = min(w, blobber.W/k), min(h, blobber.H/k)
	d := winCanvas.SubImage(rc(0, 0, w, h)).(*ebiten.Image)
	d.Clear()
	Frame(d, rc(0, 0, w, h))
	if title != "" {
		Title(d, title, w/2, 14)
	}
	for i, l := range lines {
		y := top + 16*i
		if i == sel {
			fillRect(d, rc(14, y-1, w-28, 16), selFill)
		}
		Print(d, l, 24, y, light)
	}
	x, y := (blobber.W-w*k)/2, (blobber.H-h*k)/2
	switch {
	case bottom && blobber.Fighting(): // combat menus sit top left, clear of the monster in front
		x, y = 8, 24
	case bottom:
		y = blobber.H - 8*k - h*k
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(k), float64(k))
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(d, op)
}

// BoardImage renders a shop sign: pale letters on a dark oak board with a carved rim.
func BoardImage(lines []string) *ebiten.Image {
	w := 64
	for _, l := range lines {
		w = max(w, font.Width(l)+40)
	}
	h := max(40, 16*len(lines)+24)
	img := ebiten.NewImage(w, h)
	oak, grain, rim, edge := color.RGBA{0x5e, 0x3c, 0x1e, 255}, color.RGBA{0x4c, 0x2f, 0x16, 255}, color.RGBA{0x2e, 0x1b, 0x0c, 255}, color.RGBA{0x8a, 0x5e, 0x32, 255}
	img.Fill(oak)
	for y := 5; y < h-4; y += 6 { // plank grain
		fillRect(img, rc(4, y, w-8, 1), grain)
	}
	for _, r := range []image.Rectangle{rc(0, 0, w, 3), rc(0, h-3, w, 3), rc(0, 0, 3, h), rc(w-3, 0, 3, h)} {
		fillRect(img, r, rim)
	}
	for _, r := range []image.Rectangle{rc(3, 3, w-6, 1), rc(3, 3, 1, h-6)} { // a lit inner edge
		fillRect(img, r, edge)
	}
	for i, l := range lines {
		Print(img, l, (w-font.Width(l))/2, 12+16*i, color.RGBA{0xf2, 0xdf, 0xaa, 255})
	}
	return img
}

// SignImage renders world text as ink on a parchment plaque.
func SignImage(lines []string) *ebiten.Image {
	w := 48
	for _, l := range lines {
		w = max(w, font.Width(l)+32)
	}
	h := max(48, 16*len(lines)+28)
	img := ebiten.NewImage(w, h)
	Parchment(img, rc(0, 0, w, h))
	for i, l := range lines {
		Print(img, l, (w-font.Width(l))/2, 14+16*i, ink)
	}
	return img
}

var panelCanvas = ebiten.NewImage(512, 512)

// Panel draws a dark framed backdrop over screen rectangle r (device pixels), e.g. behind the minimap.
func Panel(screen *ebiten.Image, r image.Rectangle) {
	k := K(cw, ch)
	w, h := max(32, r.Dx()/k), max(32, r.Dy()/k)
	d := panelCanvas.SubImage(rc(0, 0, w, h)).(*ebiten.Image)
	d.Clear()
	DarkFrame(d, rc(0, 0, w, h))
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(k), float64(k))
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	screen.DrawImage(d, op)
}

// Blobber: a first-person grid crawler ("gridder") in pure 2D, Deepnight style.
// No 3D pipeline: every surface is a flat quad, transformed by hand, painted far-to-near.
// The texture shader adds Deepnight's look (look.go): stepped darkness into fog, light maps, glows.
package blobber

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"gpu"
	"gpu/vector"
)

var (
	W, H  = 640, 480 // set by Layout to the real window size in device pixels
	focal = float64(W) * focalW
)

// phlan-poc: Pool of Radiance's projection, measured from its view (88 px square; a wall right ahead
// 56 px wide, one square further 24 px): the eye 0.75 squares from the wall ahead, a focal length of
// 42/88 of the square view (in the 4:3 window that square is 3/4 of the width), walls 64/56 as tall
// as wide, standing on the floor 0.5 below the eye.
const (
	focalW  = 42.0 / 88 * 0.75
	wallTop = 0.5 - 64.0/56
)

const (
	near    = 0.05
	eyeBack = 0.25 // camera offset behind the tile center, in tiles (<0.5 keeps it inside the tile)
	radius  = 8    // max ray depth in cells
	tex     = 128  // texture size
)

var level []string // set by Run

// LoadMap reads a map file: one row per line, '.' floor, other characters walls by material.
// Lines starting with a lowercase word and a space ("object ...", "item ...") are not grid rows;
// they are returned in extra for the host to interpret.
func LoadMap(r io.Reader) (rows, extra []string, err error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimRight(l, "\r")
		switch {
		case l == "":
		case l[0] >= 'a' && l[0] <= 'z' && strings.Contains(l, " "):
			extra = append(extra, l)
		default:
			rows = append(rows, l)
		}
	}
	return rows, extra, nil
}

// Object is a static billboard standing in a cell: map props, levers, items on the floor.
// Size is its longest side in tiles. Objects outside the map are hidden.
type Object struct {
	X, Y int
	Img  *gpu.Image
	Size float64
	At   *[2]float64 // phlan-poc: an exact floor position (a tree on a square's edge or corner) instead of the cell's centre
	// phlan-poc: Front > 0: while the party stands on its square it stands this far in front of the square's
	// centre, whichever way they face, as PoR paints a thing in its own square (Kuto's well)
	Front float64
	Sink  float64     // phlan-poc: its foot this far below the floor (a ladder down a well)
	Ring  *[3]float64 // phlan-poc: a hole in the floor: half width, half depth, rim width; Img is the rim's stones
}

// texBlack (phlan-poc) fills the opening of a Ring.
var texBlack *gpu.Image

// Objects is the host-maintained list of objects on the current map.
var Objects []*Object

// Overlay is a full-screen modal (inventory, ...). Update returns false to close it.
// While one is open the party cannot move; tweens still finish.
type Overlay interface {
	Update() bool
	Draw(screen *gpu.Image)
}

var Modal Overlay

// Help, if set, is shown over everything while "?" is toggled on; Update returning false closes it.
var Help Overlay

// Keys binds extra keys to host actions; checked while no prompt or modal is open.
var Keys = map[gpu.Key]func(){}

// Prompt is a modal question: a menu of options picked with up/down and Enter, or free text
// with Enter. Escape answers "". Movement is blocked while one is up; signs and tweens are untouched.
//
// Option lines are written "  <value>  <label>" (value a number, F<number> or y/n); the menu shows
// only the label and answers with the value. A "(y/n)" line becomes the options Yes and No.
type Prompt struct {
	Lines []string
	Text  bool // free text entry
	input string
	on    func(answer string)
	sel   int // the highlighted option
}

// optionLine matches "  1  New game", "  F2  Bob", "  y  Yes".
var optionLine = regexp.MustCompile(`^\s+F?([0-9]+|[yn])\s\s+(\S.*)$`)

type option struct {
	line         int
	value, label string
}

func (p *Prompt) options() []option {
	var out []option
	for i, l := range p.Lines {
		if m := optionLine.FindStringSubmatch(l); m != nil {
			out = append(out, option{i, m[1], m[2]})
		}
	}
	return out
}

// keys are the options' hotkeys (Hotkeys of their labels).
func (p *Prompt) keys() []byte {
	var labels []string
	for _, o := range p.options() {
		labels = append(labels, o.label)
	}
	return Hotkeys(labels)
}

var prompt *Prompt

func ShowPrompt(lines []string, text bool, on func(string)) {
	var ls []string
	for _, l := range lines {
		if l == "(y/n)" {
			ls = append(ls, "  y  Yes", "  n  No")
			continue
		}
		ls = append(ls, l)
	}
	prompt = &Prompt{Lines: ls, Text: text, on: on}
}

func (p *Prompt) update() {
	done := func(a string) { prompt = nil; p.on(a) }
	if KeyJustPressed(gpu.KeyEscape) {
		done("")
		return
	}
	if p.Text {
		p.input += string(inputChars())
		if KeyJustPressed(gpu.KeyBackspace) && len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
		}
		if KeyJustPressed(gpu.KeyEnter) {
			done(p.input)
		}
		return
	}
	opts := p.options()
	if len(opts) == 0 { // a notice: any confirm closes it
		if KeyJustPressed(gpu.KeyEnter) || KeyJustPressed(gpu.KeySpace) {
			done("")
		}
		return
	}
	if KeyRepeated(gpu.KeyUp) {
		p.sel = (p.sel + len(opts) - 1) % len(opts)
	}
	if KeyRepeated(gpu.KeyDown) {
		p.sel = (p.sel + 1) % len(opts)
	}
	for i, k := range p.keys() { // a hotkey picks its option
		if k != 0 && KeyJustPressed(gpu.KeyA+gpu.Key(k-'A')) {
			done(opts[i].value)
			return
		}
	}
	if KeyJustPressed(gpu.KeyEnter) || KeyJustPressed(gpu.KeySpace) {
		done(opts[p.sel%len(opts)].value)
	}
}

func (p *Prompt) draw(screen *gpu.Image) {
	lines := append([]string{}, p.Lines...)
	sel := -1
	keys := make([]byte, len(lines))
	opts := p.options()
	for i, o := range opts {
		if i == p.sel {
			sel = o.line
		}
		lines[o.line] = "   " + o.label
		keys[o.line] = p.keys()[i]
	}
	if p.Text {
		lines = append(lines, "> "+p.input+"_")
	}
	if Window != nil {
		Window(screen, "", lines, keys, sel, true)
		return
	}
	bw, bh := float32(W)*0.6, float32(16*len(lines)+24)
	bx, by := (float32(W)-bw)/2, float32(H)-bh-40
	vector.DrawFilledRect(screen, bx, by, bw, bh, color.RGBA{0, 0, 0, 210}, false)
	vector.StrokeRect(screen, bx, by, bw, bh, 2, color.RGBA{200, 180, 120, 255}, false)
	for i, l := range lines {
		Print(screen, l, int(bx)+12, int(by)+12+i*16)
	}
}

// DebugLines, if set, supplies the text of the debug panel toggled with P.
// OnPromptAnswer, if set, supplies extra lines (the character list) for "who will" prompts;
// hosts also use it as the signal that prompt UI is available.
var (
	DebugLines     func() []string
	HUD            func(screen *gpu.Image) // drawn over the scene, under prompts and modals
	OnClick        func(x, y int) bool     // left click / tap in screen pixels; true = handled, no walking
	OnPromptAnswer func() []string

	// Skin hooks; nil keeps the plain built-in look.
	Window     func(screen *gpu.Image, title string, lines []string, keys []byte, sel int, bottom bool) // text window; keys[i] (0 none) is lines[i]'s hotkey
	SignImage  func(lines []string) *gpu.Image                                                          // world text plaque
	BoardImage func(lines []string) *gpu.Image                                                          // shop sign board (ShowSignAbove)
)

// OnMove is called after every step or turn with the new position and facing (0=N 1=E 2=S 3=W).
// OnUse is called when the player presses F (search / interact) on the current tile.
var OnMove, OnUse func(x, y, dir int)

// OnTeleportMenu runs when the menu's "Teleport to level" entry is chosen.
var OnTeleportMenu func()

// sign is world text: a self-lit billboard sitting as far ahead of its cell center as the eye sits behind.
var sign struct {
	top       bool // on the wall, top of the cell (shop sign) rather than mid-cell at eye height
	x, y, dir int
	tex       *gpu.Image
}

// ShowSign puts lines of text into the world at cell (x, y), on the dir side. ClearSign removes it.
func ShowSign(x, y, dir int, lines []string) {
	if SignImage != nil {
		sign.x, sign.y, sign.dir, sign.tex = x, y, dir, SignImage(lines)
		return
	}
	w := 8
	for _, l := range lines {
		w = max(w, Width(l)+8)
	}
	img := gpu.NewImage(w, 16*len(lines)+8)
	for i, l := range lines {
		Print(img, l, 4, 4+16*i)
	}
	sign.x, sign.y, sign.dir, sign.tex = x, y, dir, img
}

// ShowSignAbove is ShowSign for a shop sign: flat on the wall ahead, in the top of the cell.
func ShowSignAbove(x, y, dir int, lines []string) {
	ShowSign(x, y, dir, lines)
	if BoardImage != nil {
		sign.tex = BoardImage(lines)
	}
	sign.top = true
}

func ClearSign() { sign.tex, sign.top = nil, false }

// Map cells: '.' is floor, anything else is a wall whose character picks the material (see mats).
func wall(x, y int) bool {
	ch := Cell(x, y)
	return ch != '.' && ch != Door
}

// Door is the map character of an open doorway: a passable gap in a wall.
// Locked is a door that is still a wall; scripts open it with `altercell X Y D`.
const Door, Locked = 'D', 'L'

// Faces (phlan-poc) gives single wall faces their own material: key {x, y, side} is the face of floor
// cell x, y towards dirs[side]. Drawn even when that neighbour is passable (a door to walk through).
// A face can be made of parts, each covering T0..T1 of the side (left to right as seen) with U0..U1
// of its texture, so one texture can span several cells.
var NoOverlay = os.Getenv("BLOBBER_SHOT") != "" && os.Getenv("BLOBBER_OVERLAY") == "" // phlan-poc: no minimap, text or party in screenshots

var Faces = map[[3]int][]FacePart{}

// Edges (phlan-poc) are thin walls: key {x, y, side} blocks stepping out of cell x, y towards
// dirs[side]. With Faces they give a map edge walls of no thickness, one-sided like Pool of Radiance's.
var Edges = map[[3]int]bool{}

// NoMap (phlan-poc) are faces the minimap leaves out: objects the party walks through, like graves.
var NoMap = map[[3]int]bool{}

// blocked: a wall cell at x+dx, y+dy, or a thin wall on the way there.
func blocked(x, y, dx, dy int) bool {
	for i, d := range dirs {
		if d == [2]int{dx, dy} && Edges[[3]int{x, y, i}] {
			return true
		}
	}
	return wall(x+dx, y+dy)
}

// Water (phlan-poc) are the cells drawn as water instead of floor; so is everything off the map.
var Water = map[[2]int]bool{}

// Sea (phlan-poc): water off the map even without water squares on it (a river along the map's edge).
var Sea bool

// Pond (phlan-poc) are cells drawn as water that are not sea: no water off the map for them.
var Pond = map[[2]int]bool{}

// phlan-poc: a pond square's pool is a rounded square (a superellipse, as PoR paints it) of half width pondH,
// joined to a neighbouring pond square by a neck of half width pondNeck; a damp bank pondMudW wide around it
const pondH, pondNeck, pondMudW = 0.46, 0.36, 0.12

// pondEdge is the point of the pool's outline at angle t, from the square's centre.
func pondEdge(t float64) (float64, float64) {
	c, s := math.Cos(t), math.Sin(t)
	return pondH * math.Copysign(math.Sqrt(math.Abs(c)), c), pondH * math.Copysign(math.Sqrt(math.Abs(s)), s)
}

var texPondWater *gpu.Image // the sea's water, lighter: PoR's ponds are a bright blue

var mudFor map[[2]int]bool         // the Pond map mudTiles were made for
var mudTiles map[[2]int]*gpu.Image // phlan-poc: per cell, its share of the bank around the ponds

// pondMud is cell's share of the mud bank around the map's ponds: alpha from the distance to the whole
// outline (discs on the pond cells, joined by strips as wide), so a pond over several cells gets one bank.
func pondMud(cell [2]int) *gpu.Image {
	if len(Pond) == 0 {
		return nil
	}
	if mudTiles == nil || reflect.ValueOf(mudFor).Pointer() != reflect.ValueOf(Pond).Pointer() {
		mudFor, mudTiles = Pond, map[[2]int]*gpu.Image{}
		dist := func(x, z float64) float64 { // to the outline, <0 inside
			d := math.Inf(1)
			for p := range Pond {
				cx, cz := float64(p[0])+0.5, float64(p[1])+0.5
				d = math.Min(d, math.Pow(math.Pow(x-cx, 4)+math.Pow(z-cz, 4), 0.25)-pondH)
				for _, s := range [][2]float64{{1, 0}, {0, 1}} {
					if Pond[[2]int{p[0] + int(s[0]), p[1] + int(s[1])}] { // the neck: a box from centre to centre
						a, b := (x-cx)*s[0]+(z-cz)*s[1], math.Abs((x-cx)*s[1]+(z-cz)*s[0])
						d = math.Min(d, math.Hypot(math.Max(0, math.Max(-a, a-1)), math.Max(0, b-pondNeck))+math.Min(0, math.Max(math.Max(-a, a-1), b-pondNeck)))
					}
				}
			}
			return d
		}
		const px = 32
		for p := range Pond {
			for dz := -1; dz <= 1; dz++ {
				for dx := -1; dx <= 1; dx++ {
					c := [2]int{p[0] + dx, p[1] + dz}
					if _, done := mudTiles[c]; done {
						continue
					}
					img := image.NewRGBA(image.Rect(0, 0, px, px))
					for j := 0; j < px; j++ {
						for i := 0; i < px; i++ {
							d := dist(float64(c[0])+(float64(i)+0.5)/px, float64(c[1])+(float64(j)+0.5)/px)
							t := math.Max(0, math.Min(1, d/pondMudW))
							a := (1 - t*t*(3-2*t)) * 0.45 // damp earth at the water, a soft fade out (smoothstep)
							img.Set(i, j, color.RGBA{uint8(46 * a), uint8(30 * a), uint8(16 * a), uint8(255 * a)})
						}
					}
					mudTiles[c] = gpu.NewImageFromImage(img)
				}
			}
		}
	}
	return mudTiles[cell]
}

func inMap(x, y int) bool { return y >= 0 && y < len(level) && x >= 0 && x < len(level[y]) }

// texWater, texClouds (phlan-poc): floor_phlan_water and ceiling_phlan_clouds, loaded on first use.
var texWater, texClouds *gpu.Image

type FacePart struct {
	Mat            byte
	T0, T1, U0, U1 float64
	Tex            string // phlan-poc: a painted texture by name (assets/walls/TEX.png), instead of Mat
}

var texMats = map[string]*mat{} // phlan-poc: FacePart.Tex materials, loaded on first use

func texMat(name string) *mat {
	m, ok := texMats[name]
	if !ok {
		if t := wallTexture(name); t != nil {
			m = &mat{img: t, lit: wallTexture(name + "_light")}
		}
		texMats[name] = m
	}
	return m
}

// WallChars lists every map character the renderer knows a texture for, for editor palettes.
func WallChars() []byte {
	var out []byte
	for ch := range mats {
		out = append(out, ch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// material returns the texture of the wall at (x, y); unknown chars use stone.
func material(x, y int) *mat {
	if y >= 0 && y < len(level) && x >= 0 && x < len(level[y]) {
		if m := mats[level[y][x]]; m != nil {
			return m
		}
	}
	return mats['#']
}

// ---------- startup texture generation (the "fake 3D" trick) ----------

var (
	rng  = rand.New(rand.NewSource(7))
	bump = 1.0 // heightmap strength baked into the textures
)

// genStone bakes a heightmap of blocks into a lit texture. lx,ly is the baked light direction,
// so a side-facing wall gets its grooves shaded as if seen from the corridor.
func genStone(bw, bh int, offset bool, base, mortar [3]float64, lx, ly float64) *gpu.Image {
	h := make([]float64, tex*tex)
	noise := make([]float64, tex*tex)
	for y := 0; y < tex; y++ {
		for x := 0; x < tex; x++ {
			ox := 0
			if offset && (y/bh)%2 == 1 {
				ox = bw / 2
			}
			bx, by := (x+ox)%bw, y%bh
			d := math.Min(math.Min(float64(bx), float64(bw-1-bx)), math.Min(float64(by), float64(bh-1-by)))
			h[y*tex+x] = math.Max(0, math.Min(1, (d-3)/4)) + rng.Float64()*0.08
			noise[y*tex+x] = 0.85 + 0.3*rng.Float64()
		}
	}
	px := image.NewRGBA(image.Rect(0, 0, tex, tex))
	at := func(x, y int) float64 { return h[((y+tex)%tex)*tex+(x+tex)%tex] }
	for y := 0; y < tex; y++ {
		for x := 0; x < tex; x++ {
			gx, gy := at(x+1, y)-at(x-1, y), at(x, y+1)-at(x, y-1)
			shade := math.Max(0.3, math.Min(1.3, 0.85+0.7*bump*(gx*lx+gy*ly)))
			c := base
			if at(x, y) < 0.3 {
				c = mortar
			}
			n := noise[y*tex+x]
			px.Set(x, y, color.RGBA{u8(c[0] * n * shade), u8(c[1] * n * shade), u8(c[2] * n * shade), 255})
		}
	}
	img := gpu.NewImageFromImage(px)
	return img
}

func u8(v float64) uint8 { return uint8(math.Max(0, math.Min(255, v))) }

func genGrass() *gpu.Image {
	img := image.NewRGBA(image.Rect(0, 0, tex, tex))
	for i := 0; i < 40; i++ {
		x0, lean, hgt := 4+rng.Float64()*56, rng.Float64()*16-8, 20+rng.Float64()*36
		g := color.RGBA{uint8(40 + rng.Intn(60)), uint8(120 + rng.Intn(100)), uint8(30 + rng.Intn(40)), 255}
		for t := 0.0; t < hgt; t++ {
			img.Set(int(x0+lean*t*t/(hgt*hgt)), tex-1-int(t), g)
		}
	}
	return gpu.NewImageFromImage(img)
}

func genSolid(c color.RGBA) *gpu.Image { i := gpu.NewImage(8, 8); i.Fill(c); return i }

var (
	stone                                         = [3]float64{150, 130, 110}
	mortarC                                       = [3]float64{80, 72, 70}
	texFloor, texCeil                             *gpu.Image
	defFloor, defCeil                             *gpu.Image // before a map picks its own
	texGrass, texLantern, texFirefly, texFireball *gpu.Image
	texGlow                                       *gpu.Image // 64x64 white radial falloff, premultiplied
	texBolt                                       *gpu.Image
	atlas                                         []struct {
		name string
		img  *gpu.Image
	}
)

// Materials: map character -> base and mortar colours. The editor palette mirrors this table.
// MatColors lists the wall materials by map character: base and mortar colour.
var MatColors = map[byte][2][3]float64{
	'#': {stone, mortarC},
	'B': {{150, 70, 55}, {90, 60, 50}},  // brick
	'M': {{90, 120, 70}, {50, 65, 45}},  // mossy stone
	'W': {{130, 95, 55}, {70, 50, 30}},  // wood
	'I': {{95, 100, 115}, {50, 52, 60}}, // iron
}

var mats = map[byte]*mat{}

// Material returns the generated front texture of a wall character (nil if unknown). Call GenTextures first.
func Material(ch byte) *gpu.Image {
	if m := mats[ch]; m != nil {
		return m.img
	}
	return nil
}

// GenTextures bakes all textures; Run does this itself, the editor calls it directly.
func GenTextures() { genTextures() }

func genTextures() {
	rng.Seed(7)
	worldChanged()
	atlas = nil
	stoneMat := func(bw, bh int, offset bool, c [2][3]float64) *mat {
		return &mat{
			img:   genStone(bw, bh, offset, c[0], c[1], 0.3, -1),
			right: genStone(bw, bh, offset, c[0], c[1], 1, -0.4),  // wall on our left, face points right
			left:  genStone(bw, bh, offset, c[0], c[1], -1, -0.4), // wall on our right, face points left
		}
	}
	for ch, c := range MatColors {
		mats[ch] = stoneMat(64, 32, true, c)
	}
	door := stoneMat(16, 128, false, MatColors['W']) // vertical planks
	mats[Door], mats[Locked] = door, door
	painted := map[byte]string{'#': "stone"} // painted textures, each with an optional NAME_light.png
	for ch, name := range paintedWalls() {
		painted[ch] = name
	}
	for ch, name := range painted {
		if t := wallTexture(name); t != nil {
			mats[ch] = &mat{img: t, lit: wallTexture(name + "_light")}
		}
	}
	texFloor = genStone(64, 64, false, [3]float64{115, 110, 105}, [3]float64{60, 58, 55}, 0.4, -1)
	texCeil = genStone(64, 64, false, [3]float64{70, 66, 72}, [3]float64{40, 38, 42}, 0.4, 1)
	if t := wallTexture("floor"); t != nil { // painted floor and ceiling, when assets/walls has them
		texFloor = t
	}
	if t := wallTexture("ceiling"); t != nil {
		texCeil = t
	}
	defFloor, defCeil = texFloor, texCeil
	SetFlats(flatNames[0], flatNames[1])
	texGrass = genGrass()
	glow := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			d := math.Hypot(float64(x)-31.5, float64(y)-31.5) / 32
			k := uint8(255 * math.Pow(math.Max(0, 1-d), 2))
			glow.Set(x, y, color.RGBA{k, k, k, k})
		}
	}
	texGlow = gpu.NewImageFromImage(glow)
	texLantern = genSolid(color.RGBA{255, 220, 150, 255})
	texFirefly = genSolid(color.RGBA{200, 255, 180, 255})
	texFireball = genSolid(color.RGBA{255, 90, 30, 255})
	texBolt = genSolid(color.RGBA{170, 210, 255, 255})
	atlas = append(atlas, []struct {
		name string
		img  *gpu.Image
	}{{"stone", mats['#'].img}, {"floor", texFloor}, {"ceil", texCeil}, {"grass", texGrass}}...)
	for ch, m := range mats {
		if ch != '#' {
			atlas = append(atlas, struct {
				name string
				img  *gpu.Image
			}{string(ch), m.img})
		}
	}
}

// ---------- entities ----------

// Entity kinds for Spawn.
const (
	Grass    = iota // static billboard
	Lantern         // drifting warm light
	Firefly         // small flickering light
	Fireball        // glowing bolt flying straight until it hits a wall or a monster
)

type ent struct {
	kind        int
	x, y, z     float64
	hx, hz      float64 // home
	phase       float64
	vx, vz      float64 // cells/sec, Fireball only
	dmg         int     // Fireball only: damage to the monster it hits
	size, light float64
	lr, lg, lb  float64
	tex         *gpu.Image
}

var ents []*ent

// Spawn adds an entity of kind Grass, Lantern or Firefly at cell (x, y).
func Spawn(kind, x, y int) {
	e := &ent{kind: kind, hx: float64(x) + 0.5, hz: float64(y) + 0.5, phase: rng.Float64() * 6.28}
	e.x, e.z = e.hx, e.hz
	switch kind {
	case Grass:
		e.y, e.size, e.tex = 0.5-0.3, 0.6, texGrass
	case Lantern:
		e.y, e.size, e.tex, e.light, e.lr, e.lg, e.lb = -0.1, 0.12, texLantern, 0.9, 0.5, 0.7, 1
	case Firefly:
		e.y, e.size, e.tex, e.light, e.lr, e.lg, e.lb = 0.1, 0.04, texFirefly, 0.25, 0.6, 1, 0.5
	}
	ents = append(ents, e)
}

func updateEnts(t float64) {
	live := ents[:0]
	for _, e := range ents {
		switch e.kind {
		case Fireball:
			e.x, e.z = e.x+e.vx/60, e.z+e.vz/60
			e.light = 1.2 + 0.3*math.Sin(t*25+e.phase)
			cx, cz := int(math.Floor(e.x)), int(math.Floor(e.z))
			if wall(cx, cz) || HitMonster(cx, cz, e.dmg) {
				continue
			}
		case Lantern: // en_FloatingLight: patrols in a straight line, turns back at walls, bobs
			if e.vx == 0 && e.vz == 0 {
				d := dirs[int(e.phase)%4]
				e.vx, e.vz = float64(d[0])*0.6, float64(d[1])*0.6
			}
			nx, nz := e.x+e.vx/60, e.z+e.vz/60
			if wall(int(math.Floor(nx+math.Copysign(0.3, e.vx))), int(math.Floor(nz+math.Copysign(0.3, e.vz)))) {
				e.vx, e.vz = -e.vx, -e.vz
			} else {
				e.x, e.z = nx, nz
			}
			e.y = -0.1 + 0.04*math.Cos(e.phase+t*9)
		case Firefly:
			e.x = e.hx + 0.35*math.Sin(t*1.3+e.phase)*math.Cos(t*0.7+e.phase)
			e.z = e.hz + 0.35*math.Cos(t*1.1+e.phase)
			e.y = 0.1 + 0.15*math.Sin(t*2.1+e.phase)
			e.light = 0.25 + 0.2*math.Sin(t*5+e.phase*3)
		}
		live = append(live, e)
	}
	ents = live
	lightLeft = max(0, lightLeft-1.0/60)
}

// lightLeft is how many more seconds a Light spell brightens the torch.
var lightLeft float64

// Light brightens the party's light for secs seconds.
func Light(secs float64) { lightLeft = secs }

// OnKill, if set, is called when a monster dies.
var OnKill func(m Monster)

// HitMonster deals dmg to the first monster in cell (x, y); it reports whether one was there.
func HitMonster(x, y, dmg int) bool {
	a := actorAt(x, y)
	if a == nil {
		return false
	}
	hit(a, dmg)
	return true
}

// HitMelee deals dmg to the i-th monster of Melee().
func HitMelee(i, dmg int) {
	for _, a := range actors {
		if a.x == game.x && a.y == game.y && !a.dying {
			if i == 0 {
				hit(a, dmg)
				return
			}
			i--
		}
	}
}

func hit(a *Actor, dmg int) {
	if a.hp -= dmg; a.hp > 0 {
		a.play("hurt")
		return
	}
	if OnKill != nil {
		OnKill(a.state())
	}
	if a.sheet.Hurt != nil { // the hurt pose shows, then updateActors drops it
		a.dying = true
		a.play("hurt")
		return
	}
	for i, b := range actors {
		if b == a {
			actors = append(actors[:i], actors[i+1:]...)
			break
		}
	}
}

// MonsterAt names the monster standing in cell (x, y), or "".
func MonsterAt(x, y int) string {
	if a := actorAt(x, y); a != nil {
		return a.name
	}
	return ""
}

func actorAt(x, y int) *Actor {
	for _, a := range actors {
		if a.x == x && a.y == y && !a.dying {
			return a
		}
	}
	return nil
}

// Bolt launches a glowing projectile coloured r,g,b from the party in the facing direction,
// flying speed cells/sec, dealing dmg to the first monster it reaches.
func Bolt(dmg int, speed, r, g, b float64) {
	gm := game
	d := dirs[gm.dir]
	tex := texFireball
	if b > r {
		tex = texBolt
	}
	ents = append(ents, &ent{kind: Fireball, x: gm.tx, y: 0, z: gm.tz, vx: float64(d[0]) * speed, vz: float64(d[1]) * speed,
		dmg: dmg, size: 0.15, light: 1.2, lr: r, lg: g, lb: b, tex: tex, phase: rng.Float64() * 6.28})
}

// ---------- actors ----------

// Actor is a monster: a lit billboard standing on the floor of a cell. It acts once per party turn.
type Actor struct {
	name              string
	x, y              int     // cell
	fx, fz, tx, tz, t float64 // tween from/to and progress
	hp                int
	sheet             *Sheet
	pose              *gpu.Image // attack or hurt, shown for poseLeft seconds
	poseLeft          float64
	dying             bool // showing its last hurt pose; gone when it ends
}

// play shows the attack or hurt pose for PoseTime, if the monster has it.
func (a *Actor) play(name string) {
	p := map[string]*gpu.Image{"attack": a.sheet.Attack, "hurt": a.sheet.Hurt}[name]
	if p != nil {
		a.pose, a.poseLeft = p, PoseTime
	}
}

// img is the picture to draw this frame.
func (a *Actor) img() *gpu.Image {
	if a.poseLeft > 0 {
		return a.pose
	}
	return a.sheet.Idle
}

// animate runs the pose clock; it reports true when a dying actor's last pose is over.
func (a *Actor) animate(dt float64) bool {
	a.poseLeft -= dt
	return a.dying && a.poseLeft <= 0
}

var actors []*Actor

// Monster is the saved state of one monster: its table name, cell and hit points left.
type Monster struct {
	Name string
	X, Y int
	HP   int
}

func (a *Actor) state() Monster { return Monster{a.name, a.x, a.y, a.hp} }

// Monsters lists the population's living monsters, for saving.
func (p *Population) Monsters() []Monster {
	var out []Monster
	for _, a := range p.actors {
		if !a.dying {
			out = append(out, a.state())
		}
	}
	return out
}

// Live is the current map's population (not swapped out).
func Live() *Population { return &Population{actors, ents, seen} }

// ClearMonsters removes every monster from the current map; decorations stay.
func ClearMonsters() { actors = nil }

// Population is the set of monsters and decorations of one map, and the cells the party has seen.
type Population struct {
	actors []*Actor
	ents   []*ent
	seen   map[[2]int]bool
}

var seen = map[[2]int]bool{}

// Seen lists the cells of the population's map the party has seen, for saving.
func (p *Population) Seen() [][2]int {
	var out [][2]int
	for c := range p.seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][1] < out[j][1] || out[i][1] == out[j][1] && out[i][0] < out[j][0] })
	return out
}

// MarkSeen restores seen cells after a load.
func MarkSeen(cells [][2]int) {
	for _, c := range cells {
		seen[c] = true
	}
}

// Len counts monsters and decorations.
func (p *Population) Len() int { return len(p.actors) + len(p.ents) }

// SwapPopulation makes next the live population and returns the previous one (nil next = empty).
// Hosts keep one per map so monsters and decorations stay where they belong across teleports.
func SwapPopulation(next *Population) *Population {
	prev := &Population{actors, ents, seen}
	if next == nil {
		next = &Population{seen: map[[2]int]bool{}}
	}
	if next.seen == nil {
		next.seen = map[[2]int]bool{}
	}
	actors, ents, seen = next.actors, next.ents, next.seen
	return prev
}

// SpawnActor places monster name, drawn with img (e.g. a decoded PNG), at cell (x, y) with hp hit points.
func SpawnActor(name string, img image.Image, x, y, hp int) {
	spawnActor(&Actor{name: name, x: x, y: y, t: 1, sheet: &Sheet{Idle: gpu.NewImageFromImage(img)}, hp: hp})
}

// SpawnSheet places an animated monster.
func SpawnSheet(name string, s *Sheet, x, y, hp int) {
	spawnActor(&Actor{name: name, x: x, y: y, t: 1, sheet: s, hp: hp})
}

func spawnActor(a *Actor) {
	x, y := a.x, a.y
	a.tx, a.tz = float64(x)+0.5, float64(y)+0.5
	a.fx, a.fz = a.tx, a.tz
	actors = append(actors, a)
}

func (a *Actor) pos() (float64, float64) {
	e := a.t * a.t * (3 - 2*a.t)
	return a.fx + (a.tx-a.fx)*e, a.fz + (a.tz-a.fz)*e
}

// updateActors only advances movement tweens and animations; decisions happen in monstersTurn.
func (g *Game) updateActors(dt float64) {
	live := actors[:0]
	for _, a := range actors {
		if a.t < 1 {
			a.t = math.Min(1, a.t+dt/0.3)
		}
		if !a.animate(dt) {
			live = append(live, a)
		}
	}
	actors = live
}

// Turn hooks. OnTurn runs after every party action that costs a turn (step, wait, combat round), before
// the monsters move. OnMelee runs after the monsters moved whenever one shares the party's cell: the
// host then plays a combat round and calls EndTurn when it is over. Monsters only fight in melee.
var (
	OnTurn  func()
	OnMelee func()
)

// Fighting reports whether a monster shares the party's cell (melee: no stepping, turning is fine).
func Fighting() bool { return game != nil && actorAt(game.x, game.y) != nil }

// Melee lists the monsters sharing the party's cell.
func Melee() []Monster {
	var out []Monster
	for _, a := range actors {
		if a.x == game.x && a.y == game.y && !a.dying {
			out = append(out, a.state())
		}
	}
	return out
}

// Strike shows the attack pose of the first monster called name in the party's cell.
func Strike(name string) {
	for _, a := range actors {
		if a.x == game.x && a.y == game.y && !a.dying && a.name == name {
			a.play("attack")
			return
		}
	}
}

// Flee moves the party to a random cell, nearest first, that no monster can see; false if there is none.
func Flee() bool {
	g := game
	from := [2]int{g.x, g.y}
	dist := map[[2]int]int{from: 0}
	var best [][2]int
	for q := [][2]int{from}; len(q) > 0; q = q[1:] { // BFS over open floor
		c := q[0]
		if len(best) > 0 && dist[c] > dist[best[0]] {
			break
		}
		if c != from && actorAt(c[0], c[1]) == nil && !seenByMonster(c[0], c[1]) {
			best = append(best, c)
		}
		for _, d := range dirs {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if _, ok := dist[n]; !ok && !wall(n[0], n[1]) && Cell(n[0], n[1]) != 0 {
				dist[n] = dist[c] + 1
				q = append(q, n)
			}
		}
	}
	if len(best) == 0 {
		return false
	}
	c := best[rng.Intn(len(best))]
	SetMap(level, c[0], c[1])
	return true
}

// seenByMonster reports whether any monster has a straight, wall-free line to cell (x, y): every cell
// the segment between the two cell centres touches is open (a corner touch counts as both neighbours).
func seenByMonster(x, y int) bool {
	for _, a := range actors {
		if !a.dying && lineClear(a.x, a.y, x, y) {
			return true
		}
	}
	return false
}

// lineClear walks the grid cells along the centre-to-centre segment (supercover line).
func lineClear(x0, y0, x1, y1 int) bool {
	dx, dy := abs(x1-x0), abs(y1-y0)
	sx, sy := sgn(x1-x0), sgn(y1-y0)
	x, y := x0, y0
	for ix, iy := 0, 0; ix < dx || iy < dy; {
		switch d := (1+2*ix)*dy - (1+2*iy)*dx; {
		case d == 0: // through a corner: both side cells must be open
			if wall(x+sx, y) || wall(x, y+sy) {
				return false
			}
			x, y, ix, iy = x+sx, y+sy, ix+1, iy+1
		case d < 0:
			x, ix = x+sx, ix+1
		default:
			y, iy = y+sy, iy+1
		}
		if wall(x, y) {
			return false
		}
	}
	return true
}

// Sight is how far (in steps) a monster notices the party and chases it.
var Sight = 8

// Invisible makes monsters ignore the party: they wander and never attack (debug, G key).
var Invisible bool

// EndTurn passes a turn from outside the engine, e.g. after a spell is cast.
func EndTurn() { game.endTurn() }

type timer struct {
	at float64
	f  func()
}

// timers are After's pending calls; while any is pending the party takes no input.
var timers []timer

// After calls f sec seconds from now, holding the party's input until then: pacing for turns that
// play out by themselves, like auto combat.
func After(sec float64, f func()) {
	if sec <= 0 { // at once
		f()
		return
	}
	timers = append(timers, timer{game.time + sec, f})
}

// endTurn passes a turn: hosts tick the clock, then every monster acts.
func (g *Game) endTurn() {
	if OnTurn != nil {
		OnTurn()
	}
	g.monstersTurn()
	if OnMelee != nil && Fighting() {
		OnMelee()
	}
}

// monstersTurn: monsters in melee stay, near ones step towards (and into) the party's cell, the rest wander.
// ponytail: greedy chase, no pathfinding; monsters stall behind a corner. BFS when that shows.
func (g *Game) monstersTurn() {
	for _, a := range actors {
		if a.dying {
			continue
		}
		dx, dy := g.x-a.x, g.y-a.y
		dist := abs(dx) + abs(dy)
		if Invisible && dist > 0 { // a monster already in melee stays and fights
			dist = Sight + 1
		}
		var try [][2]int
		switch {
		case dist == 0: // in melee: it strikes during the host's round
			continue
		case dist <= Sight:
			sx, sy := sgn(dx), sgn(dy)
			try = [][2]int{{sx, 0}, {0, sy}}
			if abs(dy) > abs(dx) {
				try[0], try[1] = try[1], try[0]
			}
		default:
			try = [][2]int{dirs[rng.Intn(4)]}
		}
		for _, d := range try {
			nx, ny := a.x+d[0], a.y+d[1]
			if d == [2]int{} || wall(nx, ny) || crowdAt(nx, ny) >= MaxPerCell {
				continue
			}
			a.fx, a.fz, a.x, a.y, a.t = a.tx, a.tz, nx, ny, 0
			a.tx, a.tz = float64(nx)+0.5, float64(ny)+0.5
			break
		}
	}
}

// MaxPerCell is how many monsters fit in one cell (formation places them); a full cell blocks the rest.
const MaxPerCell = 3

// crowdAt counts the living monsters in cell (x, y).
func crowdAt(x, y int) int {
	n := 0
	for _, a := range actors {
		if a.x == x && a.y == y && !a.dying {
			n++
		}
	}
	return n
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sgn(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// ---------- game state ----------

type Game struct {
	x, y, dir int
	// smooth movement tween
	fx, fz, fa, tx, tz, ta float64 // from/to pos & angle
	t, sway                float64 // tween progress, roll direction
	moving                 bool
	queued                 gpu.Key
	next                   []Camera // follow mode: the host's moves still to animate, in order
	hold, asked            bool     // follow mode: the host's move key is held; its next move was asked for
	// a step (not a turn) moves at speed v (steps per second) instead of easing, and flows into the
	// next one while walkKey is held, so walking doesn't slow down at every cell
	walk    bool
	v       float64
	walkKey gpu.Key
	cool    float64 // without smooth movement: seconds until a held key moves again
	time    float64
	// settings
	dither, smooth, menu, atlasView, help, quit bool
	page                                        string    // the Esc menu's settings page shown, "" the root
	confirm                                     *question // the yes/no the Esc menu asks, or nil
	lastPoint                                   int       // the Esc menu line the mouse was last on (-1 none)
	debug                                       bool
	follow                                      bool // driven by a View: moves come from the host, never the keyboard
	torch                                       float64
	sel                                         int
	scene                                       *gpu.Image
	shader                                      *gpu.Shader      // the dither pass
	meshShader                                  *gpu.MeshShader  // the 3D faces (mesh.go)
	world                                       *world           // the map's standing geometry
	dyn                                         []gpu.MeshVertex // the frame's moving faces, and their buffer
	wallRanges                                  []wallRange      // the walls in view, by group
	dyn2                                        *gpu.Mesh
	bills                                       []bill
	glows                                       []glowSrc
	uni                                         []float32 // the frame's uniforms
	lights                                      []*ent    // the entities giving light
}

var dirs = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

func (g *Game) startMove(nx, ny, ndir int, sway float64) {
	g.fx, g.fz, g.fa = g.tx, g.tz, g.ta
	g.walk = nx != g.x || ny != g.y
	g.x, g.y, g.dir = nx, ny, ndir
	g.tx, g.tz = float64(nx)+0.5, float64(ny)+0.5
	g.ta = float64(ndir) * math.Pi / 2
	for g.ta-g.fa > math.Pi {
		g.ta -= 2 * math.Pi
	}
	for g.ta-g.fa < -math.Pi {
		g.ta += 2 * math.Pi
	}
	g.t, g.moving, g.sway = 0, true, sway
	if OnMove != nil {
		OnMove(nx, ny, ndir)
	}
}

func (g *Game) act(k gpu.Key) {
	if g.moving {
		g.queued = k // ponytail: one-slot input queue, ring buffer if players mash keys
		return
	}
	f, r := dirs[g.dir], dirs[(g.dir+1)%4]
	step := func(dx, dy int, sway float64) {
		nx, ny := g.x+dx, g.y+dy
		switch {
		case blocked(g.x, g.y, dx, dy), Fighting(): // no stepping out of melee
		default:
			g.startMove(nx, ny, g.dir, sway)
			g.walkKey = k
			g.endTurn()
		}
	}
	switch k {
	case gpu.KeyW, gpu.KeyUp:
		step(f[0], f[1], 0)
	case gpu.KeyS, gpu.KeyDown:
		step(-f[0], -f[1], 0)
	case gpu.KeyA:
		step(-r[0], -r[1], -1)
	case gpu.KeyD:
		step(r[0], r[1], 1)
	case gpu.KeyQ, gpu.KeyLeft:
		g.startMove(g.x, g.y, (g.dir+3)%4, 0)
		g.walkKey = k
	case gpu.KeyE, gpu.KeyRight:
		g.startMove(g.x, g.y, (g.dir+1)%4, 0)
		g.walkKey = k
	}
}

func (g *Game) Update() error {
	g.time += 1.0 / 60
	updateEnts(g.time)
	g.updateActors(1.0 / 60)
	for i := 0; i < len(timers); i++ {
		if t := timers[i]; g.time >= t.at {
			timers = slices.Delete(timers, i, i+1)
			t.f() // may add timers of its own
			i--
		}
	}
	ctx := byte('x')
	if !g.menu && prompt == nil && Modal == nil && !g.help {
		ctx = 'w'
	}
	pollPads(ctx)
	if KeyJustPressed(gpu.KeyF12) {
		g.atlasView = !g.atlasView
	}
	if KeyJustPressed(gpu.KeyP) {
		g.debug = !g.debug
	}
	// "?" toggles the help screen over everything, except while typing an answer
	if Help != nil && (prompt == nil || !prompt.Text) && strings.ContainsRune(string(inputChars()), '?') {
		g.help = !g.help
		return nil
	}
	if g.help {
		g.help = Help.Update()
		g.tween()
		return nil
	}
	if len(timers) > 0 && prompt == nil && Modal == nil {
		g.tween()
		return nil
	}
	if prompt != nil {
		if Fighting() { // a combat round is asking: turning is still allowed
			for _, k := range []gpu.Key{gpu.KeyQ, gpu.KeyE, gpu.KeyLeft, gpu.KeyRight} {
				if KeyJustPressed(k) {
					g.act(k)
				}
			}
		}
		prompt.update()
		g.tween()
		return nil
	}
	if m := Modal; m != nil {
		if !m.Update() && Modal == m { // closing may have opened the next window
			Modal = nil
		}
		g.tween()
		return nil
	}
	for k, f := range Keys {
		if KeyJustPressed(k) {
			f()
		}
	}
	if KeyJustPressed(gpu.KeyEscape) || gpu.IsMouseButtonJustPressed(gpu.MouseButtonRight) {
		if !g.back() { // Esc in a page goes back to the root
			g.menu, g.sel = !g.menu, 0 // Esc opens the menu (Quit is in it) and closes it again
		}
	}
	if g.menu {
		if g.updateMenu() {
			return gpu.ErrTermination
		}
	} else {
		if KeyJustPressed(gpu.KeySpace) && !g.moving && !Fighting() { // wait a turn
			g.endTurn()
		}
		if KeyJustPressed(gpu.KeyF) && !g.moving && OnUse != nil {
			OnUse(g.x, g.y, g.dir)
		}
		for _, k := range []gpu.Key{gpu.KeyW, gpu.KeyS, gpu.KeyA, gpu.KeyD, gpu.KeyQ, gpu.KeyE, gpu.KeyUp, gpu.KeyDown, gpu.KeyLeft, gpu.KeyRight} {
			if KeyJustPressed(k) || (!g.moving && g.cool <= 0 && KeyPressed(k)) { // held key = continuous
				g.act(k)
			}
		}
		// mouse / touch: middle third walks, sides turn
		px, py := -1, -1
		if gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft) {
			px, py = gpu.CursorPosition()
		}
		for _, id := range gpu.AppendJustPressedTouchIDs(nil) {
			px, py = gpu.TouchPosition(id)
		}
		if px >= 0 && OnClick != nil && OnClick(px, py) {
			px = -1
		}
		switch {
		case px < 0:
		case px < W/3:
			g.act(gpu.KeyQ)
		case px > 2*W/3:
			g.act(gpu.KeyE)
		default:
			g.act(gpu.KeyW)
		}
	}
	g.tween()
	return nil
}

// walking and turning: top speeds in steps / quarter turns per second, and acceleration / braking
const walkSpeed, turnSpeed, walkAccel = 5.0, 7.0, 70.0 // a turn (13 frames) a little quicker than a step (16)

// snapRepeat is how often a held key moves without smooth movement, in seconds.
const snapRepeat = 0.15

// tween advances a step or turn by one frame: full speed while its key is held (and, for a step, the
// next cell is free), flowing on into the next one; otherwise braking to stop exactly on the cell or
// facing. Without smooth movement a move is done at once.
func (g *Game) tween() {
	const dt = 1.0 / 60
	g.cool -= dt
	if !g.moving {
		return
	}
	if !g.smooth { // snap: done at once; a held key repeats after a pause
		g.t, g.v, g.moving, g.cool = 1, 0, false, snapRepeat
		return
	}
	sp := math.Max(minSpeed, math.Min(maxSpeed, Settings.Speed))
	top, acc := turnSpeed*sp, walkAccel*sp*sp // acceleration scales with speed², so the ease takes the same share of a step
	if g.walk {
		top = walkSpeed * sp
	}
	d := [2]int{int(math.Round(g.tx - g.fx)), int(math.Round(g.tz - g.fz))}
	on := !g.follow && Settings.Fluid && KeyPressed(g.walkKey) && prompt == nil && Modal == nil && (!g.walk || !Fighting() && !blocked(g.x, g.y, d[0], d[1]))
	if g.follow { // the host's next move is known (or, fluid, on its way): flow into it, faster while they pile up
		on = len(g.next) > 0 || Settings.Fluid && g.hold && (!g.walk || !blocked(g.x, g.y, d[0], d[1]))
		top *= float64(1 + len(g.next)/2)
	}
	if !on && g.v*g.v >= 2*acc*(1-g.t) { // brake: v²/2a is the distance braking takes
		g.v = math.Max(0.5, g.v-acc*dt)
	} else {
		g.v = math.Min(top, g.v+acc*dt)
	}
	g.t += g.v * dt
	if g.t < 1 {
		return
	}
	g.moving = false
	if on { // straight on, carrying the overshoot and speed into the next step
		over, v := g.t-1, g.v
		if g.follow && len(g.next) == 0 { // held: wait at speed for the host's next move
			g.t, g.moving = 1, true
			return
		}
		if g.follow {
			c := g.next[0]
			g.next = g.next[1:]
			g.followStep(c)
		} else {
			g.act(g.walkKey)
		}
		if g.moving {
			g.t, g.v = over, v
			return
		}
	}
	g.t, g.v = 1, 0
	if g.queued != 0 && prompt == nil {
		k := g.queued
		g.queued = 0
		g.act(k)
	}
}

// updateMenu runs the Esc menu for one frame and reports true when Quit was confirmed.
func (g *Game) updateMenu() bool {
	items := g.menuOptions()
	n := len(items)
	for i, k := range menuKeys(items) { // a hotkey picks its line
		if k != 0 && KeyJustPressed(gpu.KeyA+gpu.Key(k-'A')) {
			g.sel = i
			items[i].change(g, 0)
			return g.quit
		}
	}
	if KeyRepeated(gpu.KeyUp) {
		g.sel = (g.sel + n - 1) % n
	}
	if KeyRepeated(gpu.KeyDown) {
		g.sel = (g.sel + 1) % n
	}
	g.sel %= n
	d, ok := 0, KeyJustPressed(gpu.KeyEnter) || KeyJustPressed(gpu.KeySpace)
	if g.page != "" && g.confirm == nil { // Left and Right step a page's settings
		switch {
		case KeyJustPressed(gpu.KeyLeft):
			d, ok = -1, true
		case KeyJustPressed(gpu.KeyRight):
			d, ok = 1, true
		}
	}
	if !ok {
		return false
	}
	items[g.sel].change(g, d)
	if g.page != "" {
		Settings.Smooth, Settings.Dither, Settings.Fullscreen, Settings.Torch = g.smooth, g.dither, gpu.IsFullscreen(), g.torch
		if OnSettings != nil {
			OnSettings(Settings)
		}
	}
	return g.quit
}

// back is Esc in the menu: a page or a question goes back to the root (true), the root closes (false).
func (g *Game) back() bool {
	if g.page == "" && g.confirm == nil {
		return false
	}
	g.page, g.confirm, g.sel = "", nil, 0
	return true
}

// menuLine is one line of the Esc menu: change runs it (d: 0 Enter, -1 Left, 1 Right); value, when
// set, is shown after the label.
type menuLine struct {
	label  string
	value  func(g *Game) string
	change func(g *Game, d int)
}

func flip(p *bool) func(*Game, int) { return func(*Game, int) { *p = !*p } }

// step moves v by s within lo..hi: Left down, Right up, Enter up and round to lo past hi.
func step(v, lo, hi, s float64, d int) float64 {
	switch {
	case d < 0:
		v -= s
	case v+s > hi+s/2:
		if d == 0 {
			return lo
		}
	default:
		v += s
	}
	return math.Round(math.Max(lo, math.Min(hi, v))/s) * s
}

// UISteps are the sizes the host's screen elements come in: fractions down, whole multiples up.
var UISteps = []float64{0.125, 0.25, 0.5, 1, 2, 3, 4}

// uiLine is a Graphics line stepping *v through UISteps (Enter: round to the smallest past the largest).
func uiLine(label string, v *float64) menuLine {
	return menuLine{label, func(*Game) string {
		if *v < 1 {
			return fmt.Sprintf("1/%.0fx", 1 / *v)
		}
		return fmt.Sprintf("%.0fx", *v)
	}, func(_ *Game, d int) {
		i := slices.Index(UISteps, *v)
		if i < 0 {
			i = 3
		}
		switch {
		case d < 0:
			i = max(i-1, 0)
		case d > 0:
			i = min(i+1, len(UISteps)-1)
		default:
			i = (i + 1) % len(UISteps)
		}
		*v = UISteps[i]
	}}
}

// bar draws v in lo..hi (step s) as a slider.
func bar(v, lo, hi, s float64) string {
	i, n := int(math.Round((v-lo)/s)), int(math.Round((hi-lo)/s))
	return strings.Repeat("#", i+1) + strings.Repeat("-", n-i)
}

// volumeLine is an Audio line stepping *v through 0..1 in tenths, as a slider.
func volumeLine(label string, v *float64) menuLine {
	return menuLine{label, func(*Game) string { return bar(*v, 0, 1, 0.1) + fmt.Sprintf(" %3.0f%%", *v*100) },
		func(_ *Game, d int) { *v = step(*v, 0, 1, 0.1, d) }}
}

func onOffOf(p *bool) func(*Game) string { return func(*Game) string { return onOff(*p) } }

// menuOptions are the lines of the menu page shown: the root, a settings page, or a yes/no question.
func (g *Game) menuOptions() []menuLine {
	if g.confirm != nil {
		do := g.confirm.do
		return []menuLine{{label: "No", change: func(g *Game, _ int) { g.back() }},
			{label: "Yes", change: func(g *Game, _ int) { g.back(); do() }}}
	}
	switch g.page {
	case "Graphics":
		return []menuLine{
			{"Fullscreen", func(*Game) string { return onOff(gpu.IsFullscreen()) }, func(*Game, int) { gpu.SetFullscreen(!gpu.IsFullscreen()) }},
			{"Dithered darkness", func(g *Game) string { return onOff(g.dither) }, func(g *Game, _ int) { g.dither = !g.dither }},
			{"Torch intensity", func(g *Game) string { return fmt.Sprintf("%.1f", g.torch) }, func(g *Game, d int) { g.torch = step(g.torch, 0.5, 1.5, 0.5, d) }},
			uiLine("Portraits", &Settings.Cards),
			uiLine("Minimap size", &Settings.Map),
			uiLine("Picture box", &Settings.Pic),
			uiLine("Menus", &Settings.Menu),
			uiLine("Text box", &Settings.Text),
		}
	case "Audio":
		return []menuLine{
			{"Voice-over", func(*Game) string { return onOff(Voice(false)) }, func(*Game, int) { Voice(true) }},
			volumeLine("Music volume", &Settings.Music),
			volumeLine("Effects volume", &Settings.Effects),
			volumeLine("Voice volume", &Settings.Voice),
		}
	case "Gameplay":
		return []menuLine{
			{"Smooth movement", func(g *Game) string { return onOff(g.smooth) }, func(g *Game, _ int) { g.smooth = !g.smooth }},
			{"Fluid movement", onOffOf(&Settings.Fluid), flip(&Settings.Fluid)},
			{"Walk speed", func(*Game) string {
				return bar(Settings.Speed, minSpeed, maxSpeed, 0.25) + fmt.Sprintf(" %.2fx", Settings.Speed)
			}, func(_ *Game, d int) { Settings.Speed = step(Settings.Speed, minSpeed, maxSpeed, 0.25, d) }},
			{"Minimap", func(*Game) string { return onOff(!Settings.MapOff) }, flip(&Settings.MapOff)},
			{"Minimap opacity", func(*Game) string {
				return bar(Settings.MapAlpha, minAlpha, 1, 0.1) + fmt.Sprintf(" %3.0f%%", Settings.MapAlpha*100)
			}, func(_ *Game, d int) { Settings.MapAlpha = step(Settings.MapAlpha, minAlpha, 1, 0.1, d) }},
			{"Deskew combat maps", onOffOf(&Settings.Deskew), flip(&Settings.Deskew)},
			{"Zoom speed", func(*Game) string {
				return bar(Settings.Zoom, minZoom, maxZoom, 0.25) + fmt.Sprintf(" %.2fx", Settings.Zoom)
			}, func(_ *Game, d int) { Settings.Zoom = step(Settings.Zoom, minZoom, maxZoom, 0.25, d) }},
		}
	}
	page := func(p string) menuLine {
		return menuLine{label: p, change: func(g *Game, _ int) { g.page, g.sel = p, 0 }}
	}
	ask := func(label, q string, do func()) menuLine {
		return menuLine{label: label, change: func(g *Game, _ int) { g.confirm, g.sel = &question{q, do}, 0 }}
	}
	o := []menuLine{{label: "Resume", change: func(g *Game, _ int) { g.menu = false }}}
	if OnSaveLoad != nil {
		o = append(o, menuLine{label: "Save / Load", change: func(g *Game, _ int) { g.menu = false; OnSaveLoad() }})
	}
	o = append(o, page("Gameplay"), page("Graphics"))
	if Voice != nil {
		o = append(o, page("Audio"))
	}
	if OnTeleportMenu != nil {
		o = append(o, menuLine{label: "Teleport to level", change: func(g *Game, _ int) { g.menu = false; OnTeleportMenu() }})
	}
	if OnMainMenu != nil {
		o = append(o, ask("Back to Main Menu", "Back to the main menu? Unsaved progress is lost.", func() { g.menu = false; OnMainMenu() }))
	}
	return append(o, ask("Quit to DOS", "Quit the game? Unsaved progress is lost.", func() { g.quit = true }))
}

// question is a yes/no the menu asks before do.
type question struct {
	q  string
	do func()
}

// OnSaveLoad, when set, adds "Save / Load" to the Esc menu (the host's save slots); OnMainMenu
// adds "Back to Main Menu" (asked first).
var OnSaveLoad, OnMainMenu func()

// menuItems are the lines of the menu page shown.
// menuKeys are the Esc menu lines' hotkeys (Hotkeys).
func menuKeys(items []menuLine) []byte {
	var labels []string
	for _, o := range items {
		labels = append(labels, o.label)
	}
	return Hotkeys(labels)
}

// Hotkeys gives each label its first letter not taken by a label above (0: none left).
func Hotkeys(labels []string) []byte {
	var keys []byte
	taken := map[byte]bool{}
	for _, l := range labels {
		var k byte
		for _, c := range []byte(strings.ToUpper(l)) {
			if c >= 'A' && c <= 'Z' && !taken[c] {
				k, taken[c] = c, true
				break
			}
		}
		keys = append(keys, k)
	}
	return keys
}

func (g *Game) menuItems() []string {
	var out []string
	for _, o := range g.menuOptions() {
		s := o.label
		if o.value != nil {
			s = fmt.Sprintf("%-19s%s", o.label+":", o.value(g))
		}
		out = append(out, s)
	}
	return out
}

// ---------- rendering ----------

// camera for this frame
type cam struct{ x, z, ang, roll, fwx, fwz, rtx, rtz float64 }

func (g *Game) camera() cam {
	e := g.t // tween eases by speed
	if !g.smooth {
		e = 1
	}
	c := cam{x: g.fx + (g.tx-g.fx)*e, z: g.fz + (g.tz-g.fz)*e, ang: g.fa + (g.ta-g.fa)*e}
	if g.moving && g.smooth {
		s := g.v / (walkSpeed * Settings.Speed) // a strafe leans with its speed: in as it starts, steady, out as it stops
		c.ang += 0.025 * s * g.sway
		c.roll = 0.012 * s * g.sway
	}
	c.fwx, c.fwz = math.Sin(c.ang), -math.Cos(c.ang)
	c.rtx, c.rtz = math.Cos(c.ang), math.Sin(c.ang)
	c.x, c.z = c.x-c.fwx*eyeBack, c.z-c.fwz*eyeBack // eye sits behind tile center: side exits visible, turns orbit the center
	return c
}

// visibleCells casts one DDA ray per 2 screen columns across the frustum (plus a margin for sway/roll);
// every open cell a ray crosses before hitting a wall is visible. Frustum + occlusion culling in one loop.
// ponytail: cached per camera; rays can skip a far cell seen through a 1-cell gap, cast more if that shows.
var visCache struct {
	c     cam
	cells map[[2]int]bool
}

func visibleCells(c cam) map[[2]int]bool {
	if visCache.cells != nil && visCache.c == c {
		return visCache.cells
	}
	cells := map[[2]int]bool{{int(math.Floor(c.x)), int(math.Floor(c.z))}: true}
	for sx := -W/2 - 40; sx <= W/2+40; sx += 2 {
		k := float64(sx) / focal
		dx, dz := c.fwx+c.rtx*k, c.fwz+c.rtz*k
		// DDA from (c.x,c.z) along (dx,dz)
		cx, cz := int(math.Floor(c.x)), int(math.Floor(c.z))
		stepX, stepZ := 1, 1
		tdx, tdz := math.Abs(1/dx), math.Abs(1/dz)
		tx, tz := (float64(cx+1)-c.x)*tdx, (float64(cz+1)-c.z)*tdz
		if dx < 0 {
			stepX, tx = -1, (c.x-float64(cx))*tdx
		}
		if dz < 0 {
			stepZ, tz = -1, (c.z-float64(cz))*tdz
		}
		depth := 2 * radius
		if openSky { // daylight hides nothing: rays run until they hit a wall or the map edge
			depth = len(level) + len(level[0])
		}
		for i := 0; i < depth; i++ {
			if tx < tz {
				cx, tx = cx+stepX, tx+tdx
			} else {
				cz, tz = cz+stepZ, tz+tdz
			}
			if wall(cx, cz) && inMap(cx, cz) { // phlan-poc: off the map the view runs on over water
				break
			}
			cells[[2]int{cx, cz}] = true
		}
	}
	visCache.c, visCache.cells = c, cells
	return cells
}

// drawSky fills the upper half with the ceiling texture as a panorama: the picture and its mirror
// image side by side make one full turn, so there is no seam and the sky need not tile.
// ponytail: roll is ignored, the horizon stays at H/2.
// Night (phlan-poc): the open sky is the night sky and its stars, the world lit by moonlight; T switches it
// (PoR has a day and night cycle). BLOBBER_NIGHT=1 starts at night.
var Night = os.Getenv("BLOBBER_NIGHT") != ""

var nightTint = []float32{0.42, 0.48, 0.72}

// texNightSky, texStars (phlan-poc): ceiling_sky_night and ceiling_night_stars, loaded with texClouds.
var texNightSky, texStars *gpu.Image

func (g *Game) drawSky(dst *gpu.Image, c cam) {
	if texClouds != nil && Night {
		b := texNightSky.Bounds()
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(float64(W)/float64(b.Dx()), float64(H)/2/float64(b.Dy()))
		dst.DrawImage(texNightSky, op)
		return
	}
	if texClouds != nil { // phlan-poc: the ceiling texture is a gradient, stretched; the cloud layer moves
		b := texCeil.Bounds()
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(float64(W)/float64(b.Dx()), float64(H)/2/float64(b.Dy()))
		dst.DrawImage(texCeil, op)
		return
	}
	b := texCeil.Bounds()
	s := float64(H) / 2 / float64(b.Dy())
	tw := float64(b.Dx()) * s
	turn := math.Mod(c.ang/(2*math.Pi)+1, 1) // 0..1 around the compass
	x := -turn * 2 * tw
	for i := 0; x < float64(W); i, x = i+1, x+tw {
		op := &gpu.DrawImageOptions{}
		if i%2 == 1 {
			op.GeoM.Scale(-s, s)
			op.GeoM.Translate(tw, 0)
		} else {
			op.GeoM.Scale(s, s)
		}
		op.GeoM.Translate(x, 0)
		dst.DrawImage(texCeil, op)
	}
}

// markSeen marks the cells c sees, and the walls around them, for the maps.
func markSeen(c cam) {
	for cell := range visibleCells(c) {
		seen[cell] = true
		for _, d := range dirs { // and the walls around it
			seen[[2]int{cell[0] + d[0], cell[1] + d[1]}] = true
		}
	}
}

func (g *Game) Draw(screen *gpu.Image) {
	defer maybeShot(screen)
	if g.atlasView {
		g.drawAtlas(screen)
		return
	}
	c := g.camera()
	markSeen(c)
	g.draw3D(screen, c)
	if NoOverlay { // phlan-poc: comparison shots show the 3D view alone (HUD still runs: BLOBBER_OPEN hooks it)
		if HUD != nil {
			HUD(gpu.NewImage(W, H))
		}
		return
	}
	Print(screen, fmt.Sprintf("%.0f fps  pos %d,%d facing %c   ? help", gpu.ActualFPS(), g.x, g.y, "NESW"[g.dir]), 4, 0)
	if HUD != nil && !gpu.Skip("hud") {
		HUD(screen)
	}
	if g.time < msgUntil {
		Print(screen, msg, (W-Width(msg))/2, 16)
	}
	if g.menu {
		g.drawMenu(screen)
	}
	if prompt != nil {
		prompt.draw(screen)
	}
	if Modal != nil {
		Modal.Draw(screen)
	}
	if g.help {
		Help.Draw(screen)
	}
	if g.debug && DebugLines != nil && Window != nil {
		Window(screen, "PARTY", DebugLines(), nil, -1, false)
	} else if g.debug && DebugLines != nil {
		lines := DebugLines()
		vector.DrawFilledRect(screen, 0, 20, float32(W), float32(16*len(lines)+8), color.RGBA{0, 0, 0, 180}, false)
		for i, l := range lines {
			Print(screen, l, 6, 24+i*16)
		}
	}
}

// BLOBBER_SHOT=file.png saves the 30th frame and quits: a way to check the look without a person at the keys.
var shotFrames int

func maybeShot(screen *gpu.Image) {
	path := os.Getenv("BLOBBER_SHOT")
	if path == "" {
		return
	}
	if shotFrames++; shotFrames < 30 {
		return
	}
	img := image.NewRGBA(screen.Bounds())
	screen.ReadPixels(img.Pix)
	f, err := os.Create(path)
	if err == nil {
		png.Encode(f, img)
		f.Close()
	}
	os.Exit(0)
}

func onOff(b bool) string {
	if b {
		return "ON"
	}
	return "OFF"
}

func (g *Game) drawMenu(screen *gpu.Image) {
	items, title := g.menuItems(), ""
	switch {
	case g.confirm != nil:
		title = g.confirm.q
	case g.page != "":
		title = strings.ToUpper(g.page)
	}
	b := screen.Bounds() // the game dims under the menu
	vector.DrawFilledRect(screen, 0, 0, float32(b.Dx()), float32(b.Dy()), color.NRGBA{A: 0xA0}, false)
	if Window != nil {
		keys := menuKeys(g.menuOptions())
		Window(screen, title, items, keys, g.sel, false)
		return
	}
	vector.DrawFilledRect(screen, float32(W/2-130), float32(H/2-60), 260, 152, color.RGBA{0, 0, 0, 200}, false)
	Print(screen, title, W/2-120, H/2-50)
	for i, s := range items {
		cur := "  "
		if i == g.sel {
			cur = "> "
		}
		Print(screen, cur+s, W/2-120, H/2-20+i*16)
	}
}

func (g *Game) drawAtlas(screen *gpu.Image) {
	screen.Fill(color.RGBA{30, 30, 40, 255})
	Print(screen, "Texture atlas (F12 to return). Bump details baked from the heightmap per facing.", 4, 0)
	for i, a := range atlas {
		op := &gpu.DrawImageOptions{}
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(float64(20+i*140%560), float64(40+(i*140/560)*170))
		screen.DrawImage(a.img, op)
		Print(screen, a.name, 20+i*140%560, 40+(i*140/560)*170+132)
	}
}

// formation is the i-th of n monsters sharing a cell's offset from the cell's spot, sideways (right) and
// back, seen from the party: two side by side, three with the middle one a step behind (so they
// overlap in a stable order). MaxPerCell keeps it at 3.
func formation(n, i int) [2]float64 {
	switch {
	case n == 2:
		return [][2]float64{{-0.22, 0}, {0.22, 0.05}}[min(i, 1)]
	case n >= 3:
		return [][2]float64{{-0.3, 0}, {0, 0.15}, {0.3, 0.02}}[min(i, 2)]
	}
	return [2]float64{}
}

func (g *Game) Layout(ow, oh int) (int, int) {
	sc := gpu.Monitor().DeviceScaleFactor()
	g.resize(int(float64(ow)*sc), int(float64(oh)*sc))
	return W, H
}

// resize makes the view w by h pixels.
func (g *Game) resize(w, h int) {
	if w != W || h != H {
		W, H, focal = w, h, float64(w)*focalW
		g.scene = gpu.NewImage(W, H)
	}
}

// Ordered 4x4 Bayer dither with 8 brightness levels: retro darkness without smooth gradients.
const ditherSrc = `
fn bayer2(x: f32, y: f32) -> f32 { return 2.0 * x + 3.0 * y - 4.0 * x * y; }

fn Fragment(dstPos: vec4f, srcPos: vec2f, color: vec4f, custom: vec4f) -> vec4f {
	let c = imageSrc0At(srcPos);
	let x = floor(dstPos.x);
	let y = floor(dstPos.y);
	let b = 4.0 * bayer2(x % 2.0, y % 2.0) + bayer2(floor(x / 2.0) % 2.0, floor(y / 2.0) % 2.0);
	let t = (b + 0.5) / 16.0;
	let levels = 8.0;
	return vec4f(floor(c.rgb * levels + t) / levels, 1.0);
}
`

// Run opens the window and plays the given map ('#' walls, anything else floor) from cell (x, y)
// facing dir (0 north, 1 east, 2 south, 3 west). Call Spawn before Run to place entities.
var game *Game

// SetMap swaps the level and places the party at (x, y) without a tween, keeping the facing.
// Used by Run and by script teleports.
func SetMap(lvl []string, x, y int) {
	level = lvl
	visCache.cells = nil
	worldChanged()
	if game == nil {
		game = &Game{t: 1}
	}
	g := game
	g.x, g.y = x, y
	g.tx, g.tz = float64(x)+0.5, float64(y)+0.5
	g.fx, g.fz, g.fa = g.tx, g.tz, g.ta
	g.t, g.moving, g.queued = 1, false, 0
	ClearSign()
}

// SetCell rewrites one map cell ('.' floor or a wall material character).
func SetCell(x, y int, ch byte) {
	if y < 0 || y >= len(level) || x < 0 || x >= len(level[y]) {
		return
	}
	row := []byte(level[y])
	row[x] = ch
	level[y] = string(row)
	visCache.cells = nil
	worldChanged()
}

// Cell returns the map character at x,y, or 0 outside the map.
func Cell(x, y int) byte {
	if y < 0 || y >= len(level) || x < 0 || x >= len(level[y]) {
		return 0
	}
	return level[y][x]
}

// Pos returns the party's cell and facing.
func Pos() (x, y, dir int) { return game.x, game.y, game.dir }

// Face turns the party to dir without a tween.
func Face(dir int) {
	g := game
	g.dir = dir
	g.ta = float64(dir) * math.Pi / 2
	g.fa = g.ta
}

// Message shows a line of text at the bottom of the view for a few seconds (combat and status text).
func Message(s string) { msg, msgUntil = s, game.time+3 }

var (
	msg      string
	msgUntil float64
)

// Options are the settings on the Esc menu's pages.
type Options struct {
	Smooth, Fluid, Dither, Fullscreen bool // Fluid: a held key walks or turns on without slowing at each cell
	Torch                             float64
	Speed                             float64 // walk and turn speed, minSpeed to maxSpeed times the normal
	MapOff                            bool    // the host's minimap: hidden
	MapAlpha                          float64 // the minimap's opacity, minAlpha to 1
	Cards, Map, Pic, Menu, Text       float64 // the host's portraits, minimap, picture box, menus and text box: one of UISteps times their normal size (the host shrinks them as far as the screen needs)
	Deskew                            bool    // the host's combat maps on a square grid (walls straight) instead of the original's perspective
	Zoom                              float64 // the host's battle camera: how far one wheel step zooms, minZoom to maxZoom times 15 percent
	Music, Effects, Voice             float64 // the host's volumes, 0 to 1
}

const minAlpha = 0.2

const minSpeed, maxSpeed = 0.5, 2.0

const minZoom, maxZoom = 0.25, 2.0

// Settings are the options Run starts with; set them before Run (from a settings file).
// OnSettings is called with the new options whenever the player changes one.
var (
	Settings   = Options{Smooth: true, Fluid: true, Dither: true, Fullscreen: true, Torch: 1, Speed: 1, MapAlpha: 1, Deskew: true, Zoom: 0.5, Cards: 1, Map: 1, Pic: 1, Menu: 1, Text: 1, Music: 1, Effects: 1, Voice: 1}
	OnSettings func(Options)
	// Voice, when set, adds the Audio page with its "Voice-over" line (it toggles
	// the voice first when toggle, then reports whether it is on) and the
	// Music, Effects and Voice volume sliders.
	Voice func(toggle bool) bool
)

func Run(lvl []string, x, y, dir int) error {
	g, err := newGame(dir)
	if err != nil {
		return err
	}
	SetMap(lvl, x, y)
	if Settings.Fullscreen {
		gpu.SetFullscreen(true)
	}
	gpu.SetWindowSize(W, H)
	gpu.SetWindowTitle("Blobber")
	if os.Getenv("BLOBBER_SHOT") != "" { // phlan-poc: shots never take focus, so the keyboard stays with the user
		gpu.SetRunnableOnUnfocused(true)
		return gpu.RunGameWithOptions(g, &gpu.RunGameOptions{InitUnfocused: true})
	}
	return gpu.RunGame(g)
}

// newGame makes the textures and shaders and the one Game, facing dir.
func newGame(dir int) (*Game, error) {
	genTextures()
	for _, e := range ents {
		switch e.kind {
		case Grass:
			e.tex = texGrass
		case Lantern:
			e.tex = texLantern
		case Firefly:
			e.tex = texFirefly
		}
	}
	tsh, err := gpu.NewMeshShader(meshSrc)
	if err != nil {
		return nil, err
	}
	sh, err := gpu.NewShader([]byte(ditherSrc))
	if err != nil {
		return nil, err
	}
	g := &Game{dither: Settings.Dither, smooth: Settings.Smooth, torch: Settings.Torch, scene: gpu.NewImage(W, H), shader: sh, meshShader: tsh}
	game = g
	g.ta = float64(dir) * math.Pi / 2
	g.dir = dir
	return g, nil
}

func inputChars() []rune { // phlan-poc: none while shooting
	if shooting {
		return nil
	}
	return gpu.AppendInputChars(nil)
}

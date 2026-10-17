// Blobber: a first-person grid crawler ("gridder") in pure 2D, Deepnight style.
// No 3D pipeline: every surface is a flat quad, transformed by hand, painted far-to-near.
// The texture shader adds Deepnight's look (look.go): stepped darkness into fog, light maps, glows.
package blobber

import (
	"blobber/font"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"math/rand"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
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
	Img  *ebiten.Image
	Size float64
}

// Objects is the host-maintained list of objects on the current map.
var Objects []*Object

// Overlay is a full-screen modal (inventory, ...). Update returns false to close it.
// While one is open the party cannot move; tweens still finish.
type Overlay interface {
	Update() bool
	Draw(screen *ebiten.Image)
}

var Modal Overlay

// Help, if set, is shown over everything while "?" is toggled on; Update returning false closes it.
var Help Overlay

// Keys binds extra keys to host actions; checked while no prompt or modal is open.
var Keys = map[ebiten.Key]func(){}

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
	if KeyJustPressed(ebiten.KeyEscape) {
		done("")
		return
	}
	if p.Text {
		p.input += string(inputChars())
		if KeyJustPressed(ebiten.KeyBackspace) && len(p.input) > 0 {
			p.input = p.input[:len(p.input)-1]
		}
		if KeyJustPressed(ebiten.KeyEnter) {
			done(p.input)
		}
		return
	}
	opts := p.options()
	if len(opts) == 0 { // a notice: any confirm closes it
		if KeyJustPressed(ebiten.KeyEnter) || KeyJustPressed(ebiten.KeySpace) {
			done("")
		}
		return
	}
	if KeyRepeated(ebiten.KeyUp) {
		p.sel = (p.sel + len(opts) - 1) % len(opts)
	}
	if KeyRepeated(ebiten.KeyDown) {
		p.sel = (p.sel + 1) % len(opts)
	}
	if KeyJustPressed(ebiten.KeyEnter) || KeyJustPressed(ebiten.KeySpace) {
		done(opts[p.sel%len(opts)].value)
	}
}

func (p *Prompt) draw(screen *ebiten.Image) {
	lines := append([]string{}, p.Lines...)
	sel := -1
	for i, o := range p.options() {
		if i == p.sel {
			sel = o.line
		}
		lines[o.line] = "   " + o.label
	}
	if p.Text {
		lines = append(lines, "> "+p.input+"_")
	}
	if Window != nil {
		Window(screen, "", lines, sel, true)
		return
	}
	bw, bh := float32(W)*0.6, float32(16*len(lines)+24)
	bx, by := (float32(W)-bw)/2, float32(H)-bh-40
	vector.DrawFilledRect(screen, bx, by, bw, bh, color.RGBA{0, 0, 0, 210}, false)
	vector.StrokeRect(screen, bx, by, bw, bh, 2, color.RGBA{200, 180, 120, 255}, false)
	for i, l := range lines {
		font.Print(screen, l, int(bx)+12, int(by)+12+i*16)
	}
}

// DebugLines, if set, supplies the text of the debug panel toggled with P.
// OnPromptAnswer, if set, supplies extra lines (the character list) for "who will" prompts;
// hosts also use it as the signal that prompt UI is available.
// TileMarks, if set, returns the special tiles of the current map for the minimap:
// bit 1 = automatic event, bit 2 = use (searchable) event.
var (
	DebugLines     func() []string
	HUD            func(screen *ebiten.Image) // drawn over the scene, under prompts and modals
	OnClick        func(x, y int) bool        // left click / tap in screen pixels; true = handled, no walking
	OnPromptAnswer func() []string
	TileMarks      func() map[[2]int]int

	// Skin hooks; nil keeps the plain built-in look.
	Window      func(screen *ebiten.Image, title string, lines []string, sel int, bottom bool) // text window
	SignImage   func(lines []string) *ebiten.Image                                             // world text plaque
	BoardImage  func(lines []string) *ebiten.Image                                             // shop sign board (ShowSignAbove)
	Panel       func(screen *ebiten.Image, r image.Rectangle)                                  // backdrop, e.g. the minimap
	PanelBorder func() int                                                                     // device px Panel's frame covers on each side
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
	tex       *ebiten.Image
}

// ShowSign puts lines of text into the world at cell (x, y), on the dir side. ClearSign removes it.
func ShowSign(x, y, dir int, lines []string) {
	if SignImage != nil {
		sign.x, sign.y, sign.dir, sign.tex = x, y, dir, SignImage(lines)
		return
	}
	w := 8
	for _, l := range lines {
		w = max(w, font.Width(l)+8)
	}
	img := ebiten.NewImage(w, 16*len(lines)+8)
	for i, l := range lines {
		font.Print(img, l, 4, 4+16*i)
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

func inMap(x, y int) bool { return y >= 0 && y < len(level) && x >= 0 && x < len(level[y]) }

// texWater, texClouds (phlan-poc): floor_phlan_water and ceiling_phlan_clouds, loaded on first use.
var texWater, texClouds *ebiten.Image

type FacePart struct {
	Mat            byte
	T0, T1, U0, U1 float64
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
func genStone(bw, bh int, offset bool, base, mortar [3]float64, lx, ly float64) *ebiten.Image {
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
	img := ebiten.NewImage(tex, tex)
	at := func(x, y int) float64 { return h[((y+tex)%tex)*tex+(x+tex)%tex] }
	defer addSlopes(img, tex, tex, func(x, y int) float64 { return h[y*tex+x] })
	for y := 0; y < tex; y++ {
		for x := 0; x < tex; x++ {
			gx, gy := at(x+1, y)-at(x-1, y), at(x, y+1)-at(x, y-1)
			shade := math.Max(0.3, math.Min(1.3, 0.85+0.7*bump*(gx*lx+gy*ly)))
			c := base
			if at(x, y) < 0.3 {
				c = mortar
			}
			n := noise[y*tex+x]
			img.Set(x, y, color.RGBA{u8(c[0] * n * shade), u8(c[1] * n * shade), u8(c[2] * n * shade), 255})
		}
	}
	return img
}

func u8(v float64) uint8 { return uint8(math.Max(0, math.Min(255, v))) }

func genGrass() *ebiten.Image {
	img := ebiten.NewImage(tex, tex)
	for i := 0; i < 40; i++ {
		x0, lean, hgt := 4+rng.Float64()*56, rng.Float64()*16-8, 20+rng.Float64()*36
		g := color.RGBA{uint8(40 + rng.Intn(60)), uint8(120 + rng.Intn(100)), uint8(30 + rng.Intn(40)), 255}
		for t := 0.0; t < hgt; t++ {
			img.Set(int(x0+lean*t*t/(hgt*hgt)), tex-1-int(t), g)
		}
	}
	return img
}

func genSolid(c color.RGBA) *ebiten.Image { i := ebiten.NewImage(8, 8); i.Fill(c); return i }

var (
	stone                                         = [3]float64{150, 130, 110}
	mortarC                                       = [3]float64{80, 72, 70}
	texFloor, texCeil                             *ebiten.Image
	defFloor, defCeil                             *ebiten.Image // before a map picks its own
	texGrass, texLantern, texFirefly, texFireball *ebiten.Image
	texGlow                                       *ebiten.Image // 64x64 white radial falloff, premultiplied
	texBolt                                       *ebiten.Image
	atlas                                         []struct {
		name string
		img  *ebiten.Image
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
func Material(ch byte) *ebiten.Image {
	if m := mats[ch]; m != nil {
		return m.img
	}
	return nil
}

// GenTextures bakes all textures; Run does this itself, the editor calls it directly.
func GenTextures() { genTextures() }

func genTextures() {
	rng.Seed(7)
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
	texGlow = ebiten.NewImage(64, 64)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			d := math.Hypot(float64(x)-31.5, float64(y)-31.5) / 32
			k := uint8(255 * math.Pow(math.Max(0, 1-d), 2))
			texGlow.Set(x, y, color.RGBA{k, k, k, k})
		}
	}
	texLantern = genSolid(color.RGBA{255, 220, 150, 255})
	texFirefly = genSolid(color.RGBA{200, 255, 180, 255})
	texFireball = genSolid(color.RGBA{255, 90, 30, 255})
	texBolt = genSolid(color.RGBA{170, 210, 255, 255})
	for _, e := range []struct {
		name string
		img  *ebiten.Image
	}{{"stone", mats['#'].img}, {"floor", texFloor}, {"ceil", texCeil}, {"grass", texGrass}} {
		atlas = append(atlas, e)
	}
	for ch, m := range mats {
		if ch != '#' {
			atlas = append(atlas, struct {
				name string
				img  *ebiten.Image
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
	tex         *ebiten.Image
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
		}
	}
	return
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
	pose              *ebiten.Image // attack or hurt, shown for poseLeft seconds
	poseLeft          float64
	dying             bool // showing its last hurt pose; gone when it ends
}

// play shows the attack or hurt pose for PoseTime, if the monster has it.
func (a *Actor) play(name string) {
	p := map[string]*ebiten.Image{"attack": a.sheet.Attack, "hurt": a.sheet.Hurt}[name]
	if p != nil {
		a.pose, a.poseLeft = p, PoseTime
	}
}

// img is the picture to draw this frame.
func (a *Actor) img() *ebiten.Image {
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
	spawnActor(&Actor{name: name, x: x, y: y, t: 1, sheet: &Sheet{Idle: ebiten.NewImageFromImage(img)}, hp: hp})
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
			if !Settings.SmoothMonsters {
				a.t = 1
			}
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
	queued                 ebiten.Key
	// a step (not a turn) moves at speed v (steps per second) instead of easing, and flows into the
	// next one while walkKey is held, so walking doesn't slow down at every cell
	walk    bool
	v       float64
	walkKey ebiten.Key
	cool    float64 // without smooth movement: seconds until a held key moves again
	time    float64
	// settings
	dither, smooth, menu, options, atlasView, help bool
	mapView                                        bool
	debug                                          bool
	torch                                          float64
	sel                                            int
	scene                                          *ebiten.Image
	shader, texShader                              *ebiten.Shader
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

func (g *Game) act(k ebiten.Key) {
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
	case ebiten.KeyW, ebiten.KeyUp:
		step(f[0], f[1], 0)
	case ebiten.KeyS, ebiten.KeyDown:
		step(-f[0], -f[1], 0)
	case ebiten.KeyA:
		step(-r[0], -r[1], -1)
	case ebiten.KeyD:
		step(r[0], r[1], 1)
	case ebiten.KeyQ, ebiten.KeyLeft:
		g.startMove(g.x, g.y, (g.dir+3)%4, 0)
		g.walkKey = k
	case ebiten.KeyE, ebiten.KeyRight:
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
	if !g.menu && prompt == nil && Modal == nil && !g.help && !g.mapView {
		ctx = 'w'
	}
	pollPads(ctx)
	if KeyJustPressed(ebiten.KeyF12) {
		g.atlasView = !g.atlasView
	}
	if g.mapView { // the map screen: any of M, Esc or a click closes it
		if KeyJustPressed(ebiten.KeyM) || KeyJustPressed(ebiten.KeyEscape) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.mapView = false
		}
		return nil
	}
	if KeyJustPressed(ebiten.KeyM) && prompt == nil && Modal == nil && !g.help {
		g.mapView = true
		return nil
	}
	if KeyJustPressed(ebiten.KeyP) {
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
			for _, k := range []ebiten.Key{ebiten.KeyQ, ebiten.KeyE, ebiten.KeyLeft, ebiten.KeyRight} {
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
	if KeyJustPressed(ebiten.KeyEscape) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if g.options { // Esc in Options goes back to the menu
			g.options, g.sel = false, 0
		} else {
			g.menu, g.sel = !g.menu, 0 // Esc opens the menu (Quit is in it) and closes it again
		}
	}
	if g.menu {
		if g.updateMenu() {
			return ebiten.Termination
		}
	} else {
		if KeyJustPressed(ebiten.KeySpace) && !g.moving && !Fighting() { // wait a turn
			g.endTurn()
		}
		if KeyJustPressed(ebiten.KeyF) && !g.moving && OnUse != nil {
			OnUse(g.x, g.y, g.dir)
		}
		for _, k := range []ebiten.Key{ebiten.KeyW, ebiten.KeyS, ebiten.KeyA, ebiten.KeyD, ebiten.KeyQ, ebiten.KeyE, ebiten.KeyUp, ebiten.KeyDown, ebiten.KeyLeft, ebiten.KeyRight} {
			if KeyJustPressed(k) || (!g.moving && g.cool <= 0 && KeyPressed(k)) { // held key = continuous
				g.act(k)
			}
		}
		// mouse / touch: middle third walks, sides turn
		px, py := -1, -1
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			px, py = ebiten.CursorPosition()
		}
		for _, id := range inpututil.AppendJustPressedTouchIDs(nil) {
			px, py = ebiten.TouchPosition(id)
		}
		if px >= 0 && OnClick != nil && OnClick(px, py) {
			px = -1
		}
		switch {
		case px < 0:
		case px < W/3:
			g.act(ebiten.KeyQ)
		case px > 2*W/3:
			g.act(ebiten.KeyE)
		default:
			g.act(ebiten.KeyW)
		}
	}
	g.tween()
	return nil
}

// walking and turning: top speeds in steps / quarter turns per second, and acceleration / braking
const walkSpeed, turnSpeed, walkAccel = 5.0, 3.5, 70.0

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
	on := KeyPressed(g.walkKey) && prompt == nil && Modal == nil && (!g.walk || !Fighting() && !blocked(g.x, g.y, d[0], d[1]))
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
		g.act(g.walkKey)
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

// updateMenu reports true when Quit was chosen.
func (g *Game) updateMenu() bool {
	n := len(g.menuItems())
	if KeyRepeated(ebiten.KeyUp) {
		g.sel = (g.sel + n - 1) % n
	}
	if KeyRepeated(ebiten.KeyDown) {
		g.sel = (g.sel + 1) % n
	}
	pick := KeyJustPressed(ebiten.KeyEnter) || KeyJustPressed(ebiten.KeySpace)
	if !g.options {
		switch {
		case !pick:
		case g.sel == 0:
			g.options, g.sel = true, 0
		case g.sel == 1 && OnTeleportMenu != nil:
			g.menu = false
			OnTeleportMenu()
		case g.sel == 2:
			return true
		}
		return false
	}
	if pick || KeyJustPressed(ebiten.KeyLeft) || KeyJustPressed(ebiten.KeyRight) {
		switch g.sel {
		case 0:
			g.smooth = !g.smooth
		case 1: // slider: Left slower, Right / Enter faster (Enter wraps round to the slowest)
			sp := Settings.Speed + 0.25
			if KeyJustPressed(ebiten.KeyLeft) {
				sp = math.Max(minSpeed, Settings.Speed-0.25)
			} else if sp > maxSpeed {
				sp = minSpeed
				if KeyJustPressed(ebiten.KeyRight) {
					sp = maxSpeed
				}
			}
			Settings.Speed = sp
		case 2:
			Settings.SmoothMonsters = !Settings.SmoothMonsters
		case 3:
			g.dither = !g.dither
		case 4:
			Settings.NormalMaps = !Settings.NormalMaps
		case 5:
			Settings.LightMaps = !Settings.LightMaps
		case 6:
			g.torch = math.Mod(g.torch, 1.5) + 0.5 // 0.5 → 1 → 1.5 → 0.5
		case 7:
			ebiten.SetFullscreen(!ebiten.IsFullscreen())
		}
		Settings.Smooth, Settings.Dither, Settings.Fullscreen, Settings.Torch = g.smooth, g.dither, ebiten.IsFullscreen(), g.torch
		if OnSettings != nil {
			OnSettings(Settings)
		}
	}
	return false
}

// menuItems are the Esc menu's lines, or its Options page's.
func (g *Game) menuItems() []string {
	if g.options {
		bar := int(math.Round((Settings.Speed - minSpeed) / 0.25))
		return []string{"Smooth movement:   " + onOff(g.smooth),
			"Walk speed:        " + strings.Repeat("#", bar+1) + strings.Repeat("-", int((maxSpeed-minSpeed)/0.25)-bar) + fmt.Sprintf(" %.2fx", Settings.Speed),
			"Smooth monsters:   " + onOff(Settings.SmoothMonsters), "Dithered darkness: " + onOff(g.dither), "Normal mapping:    " + onOff(Settings.NormalMaps),
			"Light maps:        " + onOff(Settings.LightMaps),
			fmt.Sprintf("Torch intensity:   %.1f", g.torch), "Fullscreen:        " + onOff(ebiten.IsFullscreen())}
	}
	return []string{"Options", "Teleport to level", "Quit game"}
}

// ---------- rendering ----------

type vert struct{ x, y, z, u, v, r, g, b float64 } // world pos, uv, light

type face struct {
	v     [4]vert
	tex   *ebiten.Image
	dist  float64
	raw   float64 // dist without the monsters' bias
	sub   int
	front bool    // a wall face turned toward us: monsters don't paint over it when it is nearer
	m     *mat    // wall material, for its light map
	inc   float64 // constant extra fade toward the fog (look.go)
	mode  int     // modeShaded, modeUnlit or modeGlow
	tint  [3]float64
	pass  int // 0 floor and ceiling, 1 light circles on them, 2 the rest, sorted far to near
}

func lerp(a, b vert, t float64) vert {
	return vert{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t, a.z + (b.z-a.z)*t, a.u + (b.u-a.u)*t, a.v + (b.v-a.v)*t,
		a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

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

// light adds the moving lights' colour at v; distance darkness is the shader's (look.go).
func (g *Game) light(v *vert) {
	v.r, v.g, v.b = 0, 0, 0
	for _, e := range ents {
		if e.light > 0 {
			d := (v.x-e.x)*(v.x-e.x) + (v.y-e.y)*(v.y-e.y) + (v.z-e.z)*(v.z-e.z)
			a := e.light / (1 + 0.5*d*d/e.light)
			v.r, v.g, v.b = v.r+a*e.lr, v.g+a*e.lg, v.b+a*e.lb
		}
	}
	v.r, v.g, v.b = math.Min(1, v.r), math.Min(1, v.g), math.Min(1, v.b)
}

var white = genSolid(color.RGBA{255, 255, 255, 255})

// drawQuad subdivides, lights, transforms, near-clips and projects one textured quad.
func (g *Game) drawQuad(dst *ebiten.Image, c cam, f *face) {
	n := f.sub
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			s0, s1, t0, t1 := float64(i)/float64(n), float64(i+1)/float64(n), float64(j)/float64(n), float64(j+1)/float64(n)
			bil := func(s, t float64) vert {
				a, b := lerp(f.v[0], f.v[1], s), lerp(f.v[3], f.v[2], s)
				v := lerp(a, b, t)
				switch f.mode {
				case modeShaded:
					g.light(&v)
				case modeGlow:
					v.r, v.g, v.b = f.tint[0], f.tint[1], f.tint[2]
				}
				// to camera space (x right, y down, z forward)
				dx, dz := v.x-c.x, v.z-c.z
				v.x, v.z = dx*c.rtx+dz*c.rtz, dx*c.fwx+dz*c.fwz
				return v
			}
			poly := []vert{bil(s0, t0), bil(s1, t0), bil(s1, t1), bil(s0, t1)}
			// clip against near plane
			var out []vert
			for k := range poly {
				a, b := poly[k], poly[(k+1)%4]
				if a.z >= near {
					out = append(out, a)
				}
				if (a.z >= near) != (b.z >= near) {
					out = append(out, lerp(a, b, (near-a.z)/(b.z-a.z)))
				}
			}
			if len(out) < 3 {
				continue
			}
			cr, sr := math.Cos(c.roll), math.Sin(c.roll)
			vs := make([]ebiten.Vertex, len(out))
			for k, v := range out {
				iz := 1 / v.z
				px, py := v.x*focal*iz, v.y*focal*iz
				// perspective-correct: u/z, v/z, 1/z in Custom0..2 (SrcX/Y get the atlas origin added, so they can't carry it)
				a := float32(1)
				if f.mode == modeGlow {
					a = 0
				}
				vs[k] = ebiten.Vertex{DstX: float32(float64(W)/2 + px*cr - py*sr), DstY: float32(float64(H)/2 + px*sr + py*cr),
					ColorR: float32(v.r), ColorG: float32(v.g), ColorB: float32(v.b), ColorA: a,
					Custom0: float32(v.u * iz), Custom1: float32(v.v * iz), Custom2: float32(iz), Custom3: float32(v.y * iz)}
			}
			var idx []uint16
			for k := 1; k+1 < len(out); k++ {
				idx = append(idx, 0, uint16(k), uint16(k+1))
			}
			dst.DrawTrianglesShader(vs, idx, g.texShader, g.faceOptions(c, f))
		}
	}
}

// drawSky fills the upper half with the ceiling texture as a panorama: the picture and its mirror
// image side by side make one full turn, so there is no seam and the sky need not tile.
// ponytail: roll is ignored, the horizon stays at H/2.
func (g *Game) drawSky(dst *ebiten.Image, c cam) {
	if texClouds != nil { // phlan-poc: the ceiling texture is a gradient, stretched; the cloud layer moves
		b := texCeil.Bounds()
		op := &ebiten.DrawImageOptions{}
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
		op := &ebiten.DrawImageOptions{}
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

func (g *Game) Draw(screen *ebiten.Image) {
	defer maybeShot(screen)
	if g.atlasView {
		g.drawAtlas(screen)
		return
	}
	c := g.camera()
	for cell := range visibleCells(c) {
		seen[cell] = true
		for _, d := range dirs { // and the walls around it
			seen[[2]int{cell[0] + d[0], cell[1] + d[1]}] = true
		}
	}
	if g.mapView {
		g.drawMap(screen, c)
		return
	}
	dst := screen
	if g.dither {
		dst = g.scene
	}
	dst.Fill(fogCol)
	if openSky {
		g.drawSky(dst, c)
		if texWater != nil { // phlan-poc: the sea runs on to the horizon past the last cells drawn
			vector.DrawFilledRect(dst, 0, float32(H)/2, float32(W), float32(H)/2, color.RGBA{40, 98, 160, 255}, false)
		}
	}

	var faces []*face
	var bias float64 // sorts faces this much nearer than their centre
	q := func(a, b, cc, d vert, t *ebiten.Image, sub int) *face {
		f := &face{v: [4]vert{a, b, cc, d}, tex: t, sub: sub, pass: 2}
		mx, mz := (a.x+cc.x)/2-c.x, (a.z+cc.z)/2-c.z
		f.raw = mx*c.fwx + mz*c.fwz
		f.dist = f.raw - bias
		if f.dist > -1 {
			faces = append(faces, f)
		}
		return f
	}
	// glow adds an additive light billboard w wide centred at (x, y, z), tinted col at alpha and
	// darkened by distance times dm; flat lays it on the floor or ceiling instead (en_LightGlow).
	glow := func(x, y, z, w float64, col [3]float64, alpha, dm float64, flat bool) {
		dx, dz := x-c.x, z-c.z
		lat, dep := dx*c.rtx+dz*c.rtz, dx*c.fwx+dz*c.fwz
		k := alpha * (1 - (1-g.torchLight(math.Sqrt(lat*lat+y*y+dep*dep)))*dm)
		tint := [3]float64{col[0] * k, col[1] * k, col[2] * k}
		r := w / 2
		var f *face
		if flat {
			f = q(vert{x - r, y, z - r, 0, 0, 0, 0, 0}, vert{x + r, y, z - r, 1, 0, 0, 0, 0}, vert{x + r, y, z + r, 1, 1, 0, 0, 0}, vert{x - r, y, z + r, 0, 1, 0, 0, 0}, texGlow, 1)
			f.pass = 1
		} else {
			hx, hz := c.rtx*r, c.rtz*r
			f = q(vert{x - hx, y - r, z - hz, 0, 0, 0, 0, 0}, vert{x + hx, y - r, z + hz, 1, 0, 0, 0, 0}, vert{x + hx, y + r, z + hz, 1, 1, 0, 0, 0}, vert{x - hx, y + r, z - hz, 0, 1, 0, 0, 0}, texGlow, 1)
		}
		f.mode, f.tint = modeGlow, tint
	}
	if texWater == nil {
		texWater, texClouds = FlatTexture("floor", "phlan_water"), FlatTexture("ceiling", "phlan_clouds")
	}
	if openSky { // phlan-poc: a cloud layer high above, so the sky moves with every step and turn
		const hgt, span, size = -6.0, 90.0, 14.0 // height, half width and cloud tile size in cells
		wind := g.time * 0.03
		x0, z0, x1, z1 := c.x-span, c.z-span, c.x+span, c.z+span
		u := func(x float64) float64 { return x/size + wind }
		f := q(vert{x0, hgt, z0, u(x0), z0 / size, 0, 0, 0}, vert{x1, hgt, z0, u(x1), z0 / size, 0, 0, 0},
			vert{x1, hgt, z1, u(x1), z1 / size, 0, 0, 0}, vert{x0, hgt, z1, u(x0), z1 / size, 0, 0, 0}, texClouds, 24)
		f.pass, f.mode = -1, modeUnlit
	}
	cells := make([][2]int, 0, len(visibleCells(c)))
	for cell := range visibleCells(c) {
		cells = append(cells, cell)
	}
	sort.Slice(cells, func(i, j int) bool { // phlan-poc: a fixed order, so equal depths sort the same every frame
		return cells[i][1] < cells[j][1] || cells[i][1] == cells[j][1] && cells[i][0] < cells[j][0]
	})
	for _, cell := range cells {
		cx, cy := cell[0], cell[1]
		x0, z0, x1, z1 := float64(cx), float64(cy), float64(cx+1), float64(cy+1)
		var f *face
		wx, wy := cx, cy
		if Water[cell] || !inMap(cx, cy) { // phlan-poc: water, its ripples drifting
			w := g.time * 0.02
			f = q(vert{x0, 0.5, z0, x0/2 + w, z0 / 2, 0, 0, 0}, vert{x1, 0.5, z0, x1/2 + w, z0 / 2, 0, 0, 0}, vert{x1, 0.5, z1, x1/2 + w, z1 / 2, 0, 0, 0}, vert{x0, 0.5, z1, x0/2 + w, z1 / 2, 0, 0, 0}, texWater, 3)
		} else {
			f = q(vert{x0, 0.5, z0, 0, 0, 0, 0, 0}, vert{x1, 0.5, z0, 1, 0, 0, 0, 0}, vert{x1, 0.5, z1, 1, 1, 0, 0, 0}, vert{x0, 0.5, z1, 0, 1, 0, 0, 0}, texFloor, 3)
		}
		f.pass = 0
		if !openSky {
			f := q(vert{x0, -0.5, z0, 0, 0, 0, 0, 0}, vert{x1, -0.5, z0, 1, 0, 0, 0, 0}, vert{x1, -0.5, z1, 1, 1, 0, 0, 0}, vert{x0, -0.5, z1, 0, 1, 0, 0, 0}, texCeil, 3)
			f.pass = 0
		}
		// wall faces on the four sides of this floor cell, textured by how they face the camera
		type side struct{ ax, az, bx, bz, nx, nz float64 }
		for i, s := range []side{{x0, z0, x1, z0, 0, 1}, {x1, z0, x1, z1, -1, 0}, {x1, z1, x0, z1, 0, -1}, {x0, z1, x0, z0, 1, 0}} {
			parts := Faces[[3]int{wx, wy, i}] // phlan-poc: a face of its own, also in front of a passable cell
			if parts == nil {
				if !wall(cx+dirs[i][0], cy+dirs[i][1]) || !inMap(cx+dirs[i][0], cy+dirs[i][1]) { // phlan-poc: no wall around the map
					continue
				}
				parts = []FacePart{{0, 0, 1, 0, 1}}
			}
			if parts != nil && (c.x-s.ax)*s.nx+(c.z-s.az)*s.nz <= 0 {
				continue // a thin wall seen from behind: one-sided, the cell beyond draws its own
			}
			var m *mat
			for _, p := range parts {
				m = mats[p.Mat]
				if m == nil {
					m = material(cx+dirs[i][0], cy+dirs[i][1])
				}
				m = m.load()
				ax, az := s.ax+(s.bx-s.ax)*p.T0, s.az+(s.bz-s.az)*p.T0
				bx, bz := s.ax+(s.bx-s.ax)*p.T1, s.az+(s.bz-s.az)*p.T1
				f := q(vert{ax, wallTop, az, p.U0, 0, 0, 0, 0}, vert{bx, wallTop, bz, p.U1, 0, 0, 0, 0}, vert{bx, 0.5, bz, p.U1, 1, 0, 0, 0}, vert{ax, 0.5, az, p.U0, 1, 0, 0, 0}, m.face(s.nx*c.rtx+s.nz*c.rtz), 3)
				f.m, f.front = m, math.Abs(s.nx*c.rtx+s.nz*c.rtz) <= 0.5
			}
			if m.lightPow > 0 && Settings.LightMaps { // a flickering glow in front of a light-mapped face
				d := dirs[i]
				k := 0.5 + 0.1*m.lightPow
				wx, wz := float64(cx+d[0])+0.5, float64(cy+d[1])+0.5
				fl := 0.8 + 0.2*math.Sin(float64(cx*7+cy*13+i)+g.time*2.4)
				glow(wx-float64(d[0])*k, 0.5-m.lightHei, wz-float64(d[1])*k, 1.5*m.lightPow, m.lightCol, 0.6*fl, 0.75, false)
			}
		}
	}
	for _, e := range ents { // billboards, only in cells a ray reached
		if _, ok := visibleCells(c)[[2]int{int(math.Floor(e.x)), int(math.Floor(e.z))}]; !ok {
			continue
		}
		if e.light > 0 { // a moving light (en_FloatingLight): its glow and light circles below and above
			col := [3]float64{e.lr, e.lg, e.lb}
			if e.kind == Fireball {
				col = [3]float64{1, 0.45, 0.15}
			}
			glow(e.x, e.y, e.z, 0.2+e.size*3, col, 1, 0.85, false)
			r := math.Min(1.5, 0.6*e.light+0.3)
			glow(e.x, 0.499, e.z, r, col, 0.30, 0.85, true)
			if !openSky {
				glow(e.x, -0.499, e.z, r, col, 0.25, 0.85, true)
			}
			continue
		}
		hx, hz := c.rtx*e.size/2, c.rtz*e.size/2
		q(vert{e.x - hx, e.y - e.size/2, e.z - hz, 0, 0, 0, 0, 0}, vert{e.x + hx, e.y - e.size/2, e.z + hz, 1, 0, 0, 0, 0},
			vert{e.x + hx, e.y + e.size/2, e.z + hz, 1, 1, 0, 0, 0}, vert{e.x - hx, e.y + e.size/2, e.z - hz, 0, 1, 0, 0, 0}, e.tex, 1)
	}
	for _, o := range Objects { // props and floor items: static floor-standing billboards
		if _, ok := visibleCells(c)[[2]int{o.X, o.Y}]; !ok || o.Img == nil {
			continue
		}
		ox, oz := float64(o.X)+0.5+c.fwx*eyeBack, float64(o.Y)+0.5+c.fwz*eyeBack
		b := o.Img.Bounds()
		k := o.Size / math.Max(float64(b.Dx()), float64(b.Dy()))
		wid, hgt := k*float64(b.Dx()), k*float64(b.Dy())
		hx, hz := c.rtx*wid/2, c.rtz*wid/2
		q(vert{ox - hx, 0.5 - hgt, oz - hz, 0, 0, 0, 0, 0}, vert{ox + hx, 0.5 - hgt, oz + hz, 1, 0, 0, 0, 0},
			vert{ox + hx, 0.5, oz + hz, 1, 1, 0, 0, 0}, vert{ox - hx, 0.5, oz - hz, 0, 1, 0, 0, 0}, o.Img, 1)
	}
	// monsters paint over the side walls of their own cell (a wide one would vanish into them); against
	// walls facing us they sort by true depth, so a corner in front of them hides them
	bias = 0.5
	crowd := map[[2]int]int{} // monsters per cell, so a group stands side by side
	for _, a := range actors {
		crowd[[2]int{a.x, a.y}]++
	}
	placed := map[[2]int]int{}
	for _, a := range actors { // monsters: floor-standing billboards
		cell := [2]int{a.x, a.y}
		slot := formation(crowd[cell], placed[cell])
		placed[cell]++
		ax, az := a.pos()
		if _, ok := visibleCells(c)[[2]int{int(math.Floor(ax)), int(math.Floor(az))}]; !ok {
			continue
		}
		if a.x == g.x && a.y == g.y { // in melee: as far in front of our centre as we stand behind it, whichever way we face
			ax, az = g.tx+c.fwx*2*eyeBack, g.tz+c.fwz*2*eyeBack
		}
		ax, az = ax-c.fwx*eyeBack, az-c.fwz*eyeBack // mirror the eye: we stand at the back of our cell, the monster at the front of its, facing us
		ax, az = ax+c.rtx*slot[0]+c.fwx*slot[1], az+c.rtz*slot[0]+c.fwz*slot[1]
		img := a.img()
		b := img.Bounds()
		size := a.sheet.Size
		if size == 0 {
			size = 0.6
		}
		k := math.Min(size/float64(b.Dy()), math.Max(1, size)/float64(b.Dx()))
		if a.sheet.density { // one pixel density for every painted monster; poses keep their own extent
			k = 1.0 / PxPerTile
		}
		wid, hgt := k*float64(b.Dx()), k*float64(b.Dy())
		hx, hz := c.rtx*wid/2, c.rtz*wid/2
		q(vert{ax - hx, 0.5 - hgt, az - hz, 0, 0, 0, 0, 0}, vert{ax + hx, 0.5 - hgt, az + hz, 1, 0, 0, 0, 0},
			vert{ax + hx, 0.5, az + hz, 1, 1, 0, 0, 0}, vert{ax - hx, 0.5, az - hz, 0, 1, 0, 0, 0}, img, 1)
	}
	bias = 0

	if sign.tex != nil {
		d := dirs[sign.dir]
		fwd, y0 := eyeBack, 0.0 // mid-cell, centred at eye height
		b := sign.tex.Bounds()
		wid, hgt := float64(b.Dx())/400, float64(b.Dy())/400 // 400 texture px per tile
		if sign.top {
			fwd, y0 = 0.48, -0.5+0.04+hgt/2 // just off the wall ahead, hanging from the top
		}
		sx, sz := float64(sign.x)+0.5+float64(d[0])*fwd, float64(sign.y)+0.5+float64(d[1])*fwd
		hx, hz := c.rtx*wid/2, c.rtz*wid/2
		q(vert{sx - hx, y0 - hgt/2, sz - hz, 0, 0, 0, 0, 0}, vert{sx + hx, y0 - hgt/2, sz + hz, 1, 0, 0, 0, 0},
			vert{sx + hx, y0 + hgt/2, sz + hz, 1, 1, 0, 0, 0}, vert{sx - hx, y0 + hgt/2, sz - hz, 0, 1, 0, 0, 0}, sign.tex, 1).mode = modeUnlit
	}
	// ponytail: painter's sort by face-center depth; z-buffer if overlap artifacts show up.
	// Floor/ceiling first: at eye height they can't occlude walls or billboards, but their centers can sort nearer.
	// The light circles on them next, so walls still cover them.
	sort.SliceStable(faces, func(i, j int) bool {
		if faces[i].pass != faces[j].pass {
			return faces[i].pass < faces[j].pass
		}
		if faces[i].front || faces[j].front {
			return faces[i].raw > faces[j].raw
		}
		return faces[i].dist > faces[j].dist
	})
	for _, f := range faces {
		g.drawQuad(dst, c, f)
	}
	if g.dither {
		op := &ebiten.DrawRectShaderOptions{}
		op.Images[0] = g.scene
		screen.DrawRectShader(W, H, g.shader, op)
	}
	if NoOverlay { // phlan-poc: comparison shots show the 3D view alone (HUD still runs: BLOBBER_OPEN hooks it)
		if HUD != nil {
			HUD(ebiten.NewImage(W, H))
		}
		return
	}
	g.drawMinimap(screen, c)
	font.Print(screen, fmt.Sprintf("%.0f fps  pos %d,%d facing %c   ? help", ebiten.ActualFPS(), g.x, g.y, "NESW"[g.dir]), 4, 0)
	if HUD != nil {
		HUD(screen)
	}
	if g.time < msgUntil {
		font.Print(screen, msg, (W-font.Width(msg))/2, 16)
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
		Window(screen, "PARTY", DebugLines(), -1, false)
	} else if g.debug && DebugLines != nil {
		lines := DebugLines()
		vector.DrawFilledRect(screen, 0, 20, float32(W), float32(16*len(lines)+8), color.RGBA{0, 0, 0, 180}, false)
		for i, l := range lines {
			font.Print(screen, l, 6, 24+i*16)
		}
	}
}

// drawMap is the full map screen (M): every seen cell of the level, walls light, floor dark, the party as an arrow.
func (g *Game) drawMap(screen *ebiten.Image, c cam) {
	screen.Fill(color.RGBA{20, 18, 24, 255})
	rows, cols := len(level), 0
	for _, r := range level {
		cols = max(cols, len(r))
	}
	cs := float32(min(W/(cols+2), H/(rows+4)))
	ox, oy := (float32(W)-cs*float32(cols))/2, (float32(H)-cs*float32(rows))/2
	for y := range rows {
		for x := range cols {
			if !seen[[2]int{x, y}] {
				continue
			}
			col := color.RGBA{50, 48, 55, 255}
			if wall(x, y) {
				col = color.RGBA{170, 160, 150, 255}
			}
			vector.DrawFilledRect(screen, ox+cs*float32(x), oy+cs*float32(y), cs-1, cs-1, col, false)
		}
	}
	if TileMarks != nil {
		for t, bits := range TileMarks() {
			if seen[t] && bits&2 != 0 {
				vector.DrawFilledCircle(screen, ox+cs*(float32(t[0])+0.5), oy+cs*(float32(t[1])+0.5), cs/4, color.RGBA{80, 200, 255, 255}, true)
			}
		}
	}
	px, py := ox+cs*float32(c.x), oy+cs*float32(c.z)
	fx, fy := float32(c.fwx)*cs/2, float32(c.fwz)*cs/2
	rx, ry := float32(c.rtx)*cs*0.3, float32(c.rtz)*cs*0.3
	yellow := color.RGBA{255, 220, 60, 255}
	vector.StrokeLine(screen, px-fx, py-fy, px+fx, py+fy, cs/4, yellow, true)
	vector.StrokeLine(screen, px+fx, py+fy, px-rx, py-ry, cs/4, yellow, true)
	vector.StrokeLine(screen, px+fx, py+fy, px+rx, py+ry, cs/4, yellow, true)
	font.Print(screen, fmt.Sprintf("Map  %d,%d   M or Esc closes", g.x, g.y), 4, 0)
}

// BLOBBER_SHOT=file.png saves the 30th frame and quits: a way to check the look without a person at the keys.
var shotFrames int

func maybeShot(screen *ebiten.Image) {
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

func (g *Game) drawMenu(screen *ebiten.Image) {
	items, title, hint := g.menuItems(), "MENU", "Enter picks, Esc closes"
	if g.options {
		title, hint = "OPTIONS", "Enter changes, Esc goes back"
	}
	if Window != nil {
		Window(screen, title, append(items, "", hint), g.sel, false)
		return
	}
	ebitenutil.DrawRect(screen, float64(W/2-130), float64(H/2-60), 260, 152, color.RGBA{0, 0, 0, 200})
	font.Print(screen, title+"  ("+hint+")", W/2-120, H/2-50)
	for i, s := range items {
		cur := "  "
		if i == g.sel {
			cur = "> "
		}
		font.Print(screen, cur+s, W/2-120, H/2-20+i*16)
	}
}

func (g *Game) drawAtlas(screen *ebiten.Image) {
	screen.Fill(color.RGBA{30, 30, 40, 255})
	font.Print(screen, "Texture atlas (F12 to return). Bump details baked from the heightmap per facing.", 4, 0)
	for i, a := range atlas {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(2, 2)
		op.GeoM.Translate(float64(20+i*140%560), float64(40+(i*140/560)*170))
		screen.DrawImage(a.img, op)
		font.Print(screen, a.name, 20+i*140%560, 40+(i*140/560)*170+132)
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

// minimapLayout is the minimap's cell size and the margin around its cells, wide enough for Panel's frame.
func minimapLayout() (cs, m float32) {
	cs = float32(5 * ebiten.Monitor().DeviceScaleFactor())
	m = 2 * cs
	if PanelBorder != nil {
		m = max(m, float32(PanelBorder())+cs/2)
	}
	return cs, m
}

// MinimapRect is the minimap's framed area on screen, in device pixels, in the top right corner.
func MinimapRect() image.Rectangle {
	const r = 7
	cs, m := minimapLayout()
	side := int(2*m + cs*(2*r+1))
	return image.Rect(W-side, 0, W, side)
}

// drawMinimap: 15x15 cells around the player top-right, walls as squares, player as an arrow along the camera.
func (g *Game) drawMinimap(screen *ebiten.Image, c cam) {
	const r = 7 // half window
	cs, m := minimapLayout()
	mr := MinimapRect()
	ox, oy := float32(mr.Min.X)+m, float32(mr.Min.Y)+m
	if Panel != nil {
		Panel(screen, MinimapRect())
	} else {
		vector.DrawFilledRect(screen, float32(mr.Min.X), float32(mr.Min.Y), float32(mr.Dx()), float32(mr.Dy()), color.RGBA{0, 0, 0, 160}, false)
	}
	for y := g.y - r; y <= g.y+r; y++ {
		for x := g.x - r; x <= g.x+r; x++ {
			if wall(x, y) && seen[[2]int{x, y}] {
				vector.DrawFilledRect(screen, ox+cs*float32(x-g.x+r), oy+cs*float32(y-g.y+r), cs, cs, color.RGBA{170, 160, 150, 255}, false)
			}
		}
	}
	for k := range Edges { // phlan-poc: thin walls as lines on the cell's side
		if !seen[[2]int{k[0], k[1]}] {
			continue
		}
		x0, y0 := ox+cs*float32(k[0]-g.x+r), oy+cs*float32(k[1]-g.y+r)
		if x0 < ox || y0 < oy || x0 >= ox+cs*(2*r+1) || y0 >= oy+cs*(2*r+1) {
			continue
		}
		ends := [4][4]float32{{0, 0, 1, 0}, {1, 0, 1, 1}, {0, 1, 1, 1}, {0, 0, 0, 1}}[k[2]]
		vector.StrokeLine(screen, x0+cs*ends[0], y0+cs*ends[1], x0+cs*ends[2], y0+cs*ends[3], cs/4, color.RGBA{170, 160, 150, 255}, false)
	}
	for k := range Edges { // phlan-poc: thin walls as lines on the cell's side
		if !seen[[2]int{k[0], k[1]}] {
			continue
		}
		x0, y0 := ox+cs*float32(k[0]-g.x+r), oy+cs*float32(k[1]-g.y+r)
		if x0 < ox || y0 < oy || x0 >= ox+cs*(2*r+1) || y0 >= oy+cs*(2*r+1) {
			continue
		}
		ends := [4][4]float32{{0, 0, 1, 0}, {1, 0, 1, 1}, {0, 1, 1, 1}, {0, 0, 0, 1}}[k[2]]
		vector.StrokeLine(screen, x0+cs*ends[0], y0+cs*ends[1], x0+cs*ends[2], y0+cs*ends[3], cs/4, color.RGBA{170, 160, 150, 255}, false)
	}
	if TileMarks != nil {
		for t, bits := range TileMarks() {
			if dx, dy := t[0]-g.x+r, t[1]-g.y+r; dx >= 0 && dx <= 2*r && dy >= 0 && dy <= 2*r {
				col := color.RGBA{255, 140, 40, 255} // automatic: orange
				if bits&2 != 0 {
					col = color.RGBA{80, 200, 255, 255} // searchable: cyan
				}
				vector.DrawFilledCircle(screen, ox+cs*(float32(dx)+0.5), oy+cs*(float32(dy)+0.5), cs/3, col, true)
			}
		}
	}
	px, py := ox+cs*float32(c.x-float64(g.x-r)), oy+cs*float32(c.z-float64(g.y-r))
	fx, fy := float32(c.fwx)*cs, float32(c.fwz)*cs // forward
	rx, ry := float32(c.rtx)*cs*0.6, float32(c.rtz)*cs*0.6
	yellow := color.RGBA{255, 220, 60, 255}
	vector.StrokeLine(screen, px-fx, py-fy, px+fx, py+fy, cs/3, yellow, true)
	vector.StrokeLine(screen, px+fx, py+fy, px-rx, py-ry, cs/3, yellow, true)
	vector.StrokeLine(screen, px+fx, py+fy, px+rx, py+ry, cs/3, yellow, true)
}

func (g *Game) Layout(ow, oh int) (int, int) {
	sc := ebiten.Monitor().DeviceScaleFactor()
	if w, h := int(float64(ow)*sc), int(float64(oh)*sc); w != W || h != H {
		W, H, focal = w, h, float64(w)*focalW
		g.scene = ebiten.NewImage(W, H)
	}
	return W, H
}

// Ordered 4x4 Bayer dither with 8 brightness levels: retro darkness without smooth gradients.
const ditherSrc = `//kage:unit pixels
package main

func bayer2(x, y float) float { return 2*x + 3*y - 4*x*y }

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	c := imageSrc0At(srcPos)
	x, y := floor(dstPos.x), floor(dstPos.y)
	b := 4*bayer2(mod(x, 2), mod(y, 2)) + bayer2(mod(floor(x/2), 2), mod(floor(y/2), 2))
	t := (b + 0.5) / 16
	levels := 8.0
	return vec4(floor(c.rgb*levels+t)/levels, 1)
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

// Options are the settings on the Options page.
type Options struct {
	Smooth, Dither, Fullscreen, SmoothMonsters, NormalMaps, LightMaps bool
	Torch                                                             float64
	Speed                                                             float64 // walk and turn speed, minSpeed to maxSpeed times the normal
}

const minSpeed, maxSpeed = 0.5, 2.0

// Settings are the options Run starts with; set them before Run (from a settings file).
// OnSettings is called with the new options whenever the player changes one.
var (
	Settings   = Options{Smooth: true, Dither: true, SmoothMonsters: true, NormalMaps: true, LightMaps: true, Torch: 1, Speed: 1}
	OnSettings func(Options)
)

func Run(lvl []string, x, y, dir int) error {
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
	tsh, err := ebiten.NewShader([]byte(texSrc))
	if err != nil {
		return err
	}
	sh, err := ebiten.NewShader([]byte(ditherSrc))
	if err != nil {
		return err
	}
	if Settings.Fullscreen {
		ebiten.SetFullscreen(true)
	}
	g := &Game{dither: Settings.Dither, smooth: Settings.Smooth, torch: Settings.Torch, scene: ebiten.NewImage(W, H), shader: sh, texShader: tsh}
	game = g
	g.ta = float64(dir) * math.Pi / 2
	g.dir = dir
	SetMap(lvl, x, y)
	ebiten.SetWindowSize(W, H)
	ebiten.SetWindowTitle("Blobber")
	if os.Getenv("BLOBBER_SHOT") != "" { // phlan-poc: shots never take focus, so the keyboard stays with the user
		ebiten.SetRunnableOnUnfocused(true)
		return ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{InitUnfocused: true})
	}
	return ebiten.RunGame(g)
}

func inputChars() []rune { // phlan-poc: none while shooting
	if shooting {
		return nil
	}
	return ebiten.AppendInputChars(nil)
}

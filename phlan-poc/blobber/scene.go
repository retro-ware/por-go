package blobber

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Scene is what a map shows: its grid and the lines of its .map file that
// only concern drawing (thin walls, water, ponds, props, floor and ceiling).
// A host keeps one per map and Shows it on entering; the lines that carry
// game state (objects, items, monsters, events) are the host's.
type Scene struct {
	Rows           []string
	Faces          map[[3]int][]FacePart
	Edges, NoMap   map[[3]int]bool
	Water, Pond    map[[2]int]bool
	Sea            bool
	Floor, Ceiling string    // painted assets/walls/floor_NAME.png / ceiling_NAME.png ("" = default)
	Props          []*Object // billboards and wells, fixed in place
}

// ParseScene reads a .map file: the scene, and the lines it left for the host.
//
//	face X Y SIDE CHAR [T0 T1 U0 U1]   the face of floor cell X, Y towards SIDE (0 N .. 3 W) in material
//	                                  CHAR, or in the painted texture CHAR when it is a name (see Faces)
//	edge X Y SIDE                      a thin wall (see Edges)
//	nomap X Y SIDE                     a face left off the minimap (see NoMap)
//	water X Y / pond X Y / sea edge    water squares, pond squares, water off the map (see Water, Pond, Sea)
//	prop TEX X Y SIZE [FRONT [SINK]]   a billboard at an exact floor position (see Object)
//	well RIMTEX X Y HALFW HALFD RIMW [FRONT]   an oval hole in the floor (see Object.Ring)
//	floor NAME / ceiling NAME          painted floor and ceiling
func ParseScene(r io.Reader) (*Scene, []string, error) {
	rows, extra, err := LoadMap(r)
	if err != nil {
		return nil, nil, err
	}
	s := &Scene{Rows: rows, Faces: map[[3]int][]FacePart{}, Edges: map[[3]int]bool{}, NoMap: map[[3]int]bool{},
		Water: map[[2]int]bool{}, Pond: map[[2]int]bool{}}
	var rest []string
	for _, l := range extra {
		ok, err := s.line(l)
		if err != nil {
			return nil, nil, fmt.Errorf("%q: %v", l, err)
		}
		if !ok {
			rest = append(rest, l)
		}
	}
	return s, rest, nil
}

// line takes one scene line; false when it is not one.
func (s *Scene) line(l string) (bool, error) {
	f := strings.Fields(l)
	need, ok := map[string]int{"floor": 2, "ceiling": 2, "face": 5, "edge": 4, "water": 3, "pond": 3, "nomap": 4,
		"prop": 5, "well": 7, "sea": 1}[f[0]]
	if !ok {
		return false, nil
	}
	if len(f) < need {
		return true, fmt.Errorf("want %d fields", need)
	}
	num := func(i int) int {
		n, _ := strconv.Atoi(f[i])
		return n
	}
	switch f[0] {
	case "prop":
		var at [2]float64
		var size, front, sink float64
		fmt.Sscan(strings.Join(f[2:], " "), &at[0], &at[1], &size, &front, &sink)
		img := WallTexture(f[1])
		if img == nil {
			return true, fmt.Errorf("no texture %q", f[1])
		}
		s.Props = append(s.Props, &Object{X: int(at[0]), Y: int(at[1]), Img: img, Size: size, At: &at, Front: front, Sink: sink})
	case "well":
		var at [2]float64
		var ring [3]float64
		var front float64
		fmt.Sscan(strings.Join(f[2:], " "), &at[0], &at[1], &ring[0], &ring[1], &ring[2], &front)
		img := WallTexture(f[1])
		if img == nil {
			return true, fmt.Errorf("no texture %q", f[1])
		}
		s.Props = append(s.Props, &Object{X: int(at[0]), Y: int(at[1]), Img: img, At: &at, Front: front, Ring: &ring})
	case "water":
		s.Water[[2]int{num(1), num(2)}] = true
	case "sea":
		s.Sea = true
	case "pond":
		s.Pond[[2]int{num(1), num(2)}] = true
	case "nomap":
		s.NoMap[[3]int{num(1), num(2), num(3)}] = true
	case "edge":
		s.Edges[[3]int{num(1), num(2), num(3)}] = true
	case "face":
		p := FacePart{Mat: f[4][0], T1: 1, U1: 1}
		if len(f[4]) > 1 { // a texture name instead of a map character
			p.Mat, p.Tex = 0, f[4]
		}
		if len(f) >= 9 {
			fmt.Sscan(strings.Join(f[5:9], " "), &p.T0, &p.T1, &p.U0, &p.U1)
		}
		k := [3]int{num(1), num(2), num(3)}
		s.Faces[k] = append(s.Faces[k], p)
	case "floor":
		s.Floor = f[1]
	case "ceiling":
		s.Ceiling = f[1]
	}
	return true, nil
}

// Show makes s the scene drawn, with the host's own objects (named props)
// among its props. It does not move the party: SetMap does.
func (s *Scene) Show(objs ...*Object) {
	Faces, Edges, Water, NoMap, Pond, Sea = s.Faces, s.Edges, s.Water, s.NoMap, s.Pond, s.Sea
	SetFlats(s.Floor, s.Ceiling)
	Objects = append(append(Objects[:0], s.Props...), objs...)
}

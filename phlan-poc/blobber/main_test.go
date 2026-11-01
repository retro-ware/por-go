package blobber

import (
	"math"
	"os"
	"testing"

	"gpu"
)

func TestMain(m *testing.M) {
	Glyph = func(byte) [8]byte { return [8]byte{0xFF} } // a stub font: a bar atop each cell
	os.Exit(m.Run())
}

func TestCore(t *testing.T) {
	level = testLevel
	if !wall(-1, 0) || !wall(0, 0) || wall(1, 1) {
		t.Fatal("wall(): outside map and '#' must be wall, '.' must not")
	}
	if _, err := gpu.NewShader([]byte(ditherSrc)); err != nil {
		t.Fatal("dither shader:", err)
	}
	if _, err := gpu.NewMeshShader(meshSrc); err != nil {
		t.Fatal("mesh shader:", err)
	}
	g := &Game{x: 1, y: 1, dir: 0, smooth: true, t: 1, tx: 1.5, tz: 1.5}
	g.startMove(1, 1, 3, 0) // turn N->W wraps angle the short way
	if !g.moving || math.Abs(g.ta-g.fa) > math.Pi+1e-9 {
		t.Fatalf("turn tween wrong: moving=%v fa=%v ta=%v", g.moving, g.fa, g.ta)
	}
	for i := 0; i < 30 && g.moving; i++ { // key not held: brakes onto the facing within half a second
		g.tween()
	}
	if g.moving || g.t != 1 || g.v != 0 {
		t.Fatalf("turn didn't stop on its facing: moving=%v t=%v v=%v", g.moving, g.t, g.v)
	}
}

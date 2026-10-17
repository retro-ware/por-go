package blobber

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestCore(t *testing.T) {
	level = testLevel
	if !wall(-1, 0) || !wall(0, 0) || wall(1, 1) {
		t.Fatal("wall(): outside map and '#' must be wall, '.' must not")
	}
	if _, err := ebiten.NewShader([]byte(ditherSrc)); err != nil {
		t.Fatal("dither shader:", err)
	}
	if _, err := ebiten.NewShader([]byte(texSrc)); err != nil {
		t.Fatal("texture shader:", err)
	}
	g := &Game{x: 1, y: 1, dir: 0, smooth: true, t: 1, tx: 1.5, tz: 1.5}
	g.act(ebiten.KeyW) // wall to the north: no move
	if g.moving {
		t.Fatal("walked into a wall")
	}
	g.act(ebiten.KeyQ) // turn N->W wraps angle the short way
	if !g.moving || math.Abs(g.ta-g.fa) > math.Pi+1e-9 {
		t.Fatalf("turn tween wrong: moving=%v fa=%v ta=%v", g.moving, g.fa, g.ta)
	}
	g.act(ebiten.KeyS)
	if g.queued != ebiten.KeyS {
		t.Fatal("input not queued while moving")
	}
	g.queued = 0
	for i := 0; i < 30 && g.moving; i++ { // key not held: brakes onto the facing within half a second
		g.tween()
	}
	if g.moving || g.t != 1 || g.v != 0 {
		t.Fatalf("turn didn't stop on its facing: moving=%v t=%v v=%v", g.moving, g.t, g.v)
	}
}

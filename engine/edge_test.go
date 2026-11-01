package engine

import "testing"

// Off the map every read is 0 (no wall, no way through), in every area.
func TestOffMapReads(t *testing.T) {
	e := &Engine{}
	e.gs.areaKind = 13
	for i := range e.gs.geo {
		e.gs.geo[i] = 0xFF
	}
	for _, p := range [][2]int{{-1, 4}, {16, 4}, {4, -1}, {4, 16}} {
		if e.WallAt(p[0], p[1], 2) != 0 || e.DoorAt(p[0], p[1], 2) != 0 || e.MapCell(p[0], p[1]) != 0 {
			t.Errorf("off-map read at %v not 0", p)
		}
	}
}

// An open side off the map that no script catches bumps (the original
// wrapped to the far edge); a caught one (New Phlan, west at 0,4) still
// leaves the area.
func TestMapEdgeNoWrap(t *testing.T) {
	e := quickStarted(t)
	e.set = 8
	e.LoadGeo(13) // Kobold Caves: (15,1) is open east off the map
	e.gs.areaKind, e.viewMode = 13, 1
	e.posX, e.posY, e.facing = 15, 1, 2
	if e.DoorAt(15, 1, 2) == 0 {
		t.Fatal("Kobold Caves (15,1) east is not open")
	}
	e.Move(func() {})
	if e.posX != 15 || e.posY != 1 {
		t.Fatalf("stepped off the map to %d,%d", e.posX, e.posY)
	}
}

func TestMapEdgeScriptedExit(t *testing.T) {
	e := quickStarted(t)
	if e.gs.areaKind != 0 {
		t.Skipf("Quick Start in area %d", e.gs.areaKind)
	}
	e.LoadArea(0)
	e.posX, e.posY, e.facing = 0, 4, 6
	e.explore(Action{Kind: ActForward})
	if e.game.LeftMap == 0 {
		t.Fatal("west from New Phlan 0,4 did not set LeftMap")
	}
	e.stepDone()
	if e.gs.areaKind != 20 {
		t.Fatalf("area %d, want the Slums (20)", e.gs.areaKind)
	}
}

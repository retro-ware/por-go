package engine

import "testing"

// TestDeskewGrid: with Deskew every walled cell is a rectangle: the walls
// lie on the block grid's rows and columns, a door is a gap in the middle.
func TestDeskewGrid(t *testing.T) {
	e := &Engine{}
	e.Deskew.Store(true)
	for i := range 0x200 { // every side of every cell a wall
		e.gs.geo[i] = 0x11
	}
	e.gs.geo[0x300+8*16+8] = 1 << 6 // cell (8,8): its west side a door
	e.posX, e.posY = 8, 8
	e.cmap = &combatMap{cursorSize: 1}
	e.dungeonMapFlat()
	for y := range 25 {
		for x := range 50 {
			wall := e.groundInfo(e.cmap.ground[y][x])[0] == 0xFF
			onGrid := (y-0xA)%5 == 0 || (x-0x15)%6 == 0
			door := x == 0x15 && (y == 0xC || y == 0xD)
			if wall != (onGrid && !door) {
				t.Errorf("(%d,%d): wall %v, want %v", x, y, wall, !wall)
			}
		}
	}
	if e.facingDir(0) != 0 || e.facingDir(2) != 4 {
		t.Errorf("deskewed facing: north %d, south %d", e.facingDir(0), e.facingDir(2))
	}
}

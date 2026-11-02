package engine

import "testing"

// TestDoorsStayOpen: a door opened by bash, pick or knock is open again
// after the map's GEO block is reloaded.
func TestDoorsStayOpen(t *testing.T) {
	e := &Engine{}
	e.posX, e.posY, e.facing = 3, 4, 2
	e.setDoor(3, 4, 2, 2)
	e.openDoor()
	e.gs.geo = [0x400]byte{} // the reload
	e.setDoor(3, 4, 2, 2)
	e.reopenDoors()
	if d := e.gs.geo[0x300+4<<4+3] >> 2 & 3; d != 1 {
		t.Fatalf("door %d, want 1 (open)", d)
	}
	if d := e.gs.geo[0x300+4<<4+4] >> 6 & 3; d != 1 {
		t.Fatalf("far side %d, want 1", d)
	}
}

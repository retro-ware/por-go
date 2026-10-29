package engine

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// A dropped portrait is saved as NAME.png beside the record and comes back
// with the character; the paper-doll head stays as it was.
func TestPortraitRoundTrip(t *testing.T) {
	e := &Engine{}
	e.Cfg.SaveDir = t.TempDir()
	c := &Character{}
	c.SetName("Yuna")
	c.PortHead, c.Portrait = 3, image.NewRGBA(image.Rect(0, 0, 4, 6))
	e.SaveCharacter(c, "CHRDATA1")
	var d Character
	if !e.LoadCharacter("CHRDATA1", &d, true) {
		t.Fatal("not loaded")
	}
	if d.Portrait == nil || d.Portrait.Bounds() != image.Rect(0, 0, 4, 6) || d.PortHead != 3 {
		t.Errorf("portrait %v head %d after the round trip", d.Portrait, d.PortHead)
	}
	c.Portrait = nil
	e.SaveCharacter(c, "CHRDATA1")
	e.LoadCharacter("CHRDATA1", &d, true)
	if d.Portrait != nil {
		t.Error("the png stayed after the portrait was dropped")
	}
}

// A named save carries the party's portraits: CHRDATA<n>.png in the slot.
func TestPortraitSavedWithSlot(t *testing.T) {
	e := &Engine{ecl: make([]byte, 0x1E00), set: 1}
	e.Cfg.SaveDir = t.TempDir()
	c := &Character{}
	c.SetName("Yuna")
	c.Portrait = image.NewRGBA(image.Rect(0, 0, 2, 2))
	e.party = append(e.party, c)
	e.saveTo("X")
	if _, err := os.Stat(filepath.Join(e.slotRoot(), "X", "CHRDATA1.png")); err != nil {
		t.Error("no portrait in the slot:", err)
	}
}

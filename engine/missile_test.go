package engine

import (
	"os"
	"slices"
	"testing"
)

// testEngine is an engine on the game files (the test skips without them),
// its saves under a temp dir: tests never read or write the game's own.
func testEngine(t *testing.T) *Engine {
	t.Helper()
	if _, err := os.Stat("../data/dos/TITLE.DAX"); err != nil {
		t.Skip("no game files")
	}
	e, err := New("../data")
	if err != nil {
		t.Fatal(err)
	}
	e.Cfg.SaveDir = t.TempDir()
	return e
}

// quickStarted: an engine with the Quick Start party, its roster written
// under a temp dir: tests never read or write the game's own saves.
func quickStarted(t *testing.T) *Engine {
	t.Helper()
	e := testEngine(t)
	seed := uint32(7) // the same party every run
	e.FixedSeed = &seed
	e.Init()
	e.QuickStart()
	if len(e.party) == 0 {
		t.Fatal("no Quick Start party")
	}
	return e
}

// A thrown weapon readied beside the weapon in hand goes into the Missiles
// slot (Readied 2), out of the hands, and comes back out of it; without a
// weapon in hand it goes there too: a thrown weapon is never readied in hand.
func TestMissileSlot(t *testing.T) {
	e := quickStarted(t)
	e.selectMember(0) // the fighter
	c := e.selected()
	w := e.AddItemCopy(c, e.RandomItem(itemLongSword))
	e.readyItem(w)
	e.ApplyItems(c)
	if c.Slots[slotWeapon] != w {
		t.Fatal("no weapon in hand")
	}
	j := e.AddItemCopy(c, e.RandomItem(itemJavelin))
	if dollSlot(e, c, j) != DollArrows {
		t.Errorf("doll slot %d, want Missiles", dollSlot(e, c, j))
	}
	e.readyItem(j)
	e.ApplyItems(c)
	if j.Readied != 2 || c.Slots[slotArrows] != j || e.missile(c) != j || c.Slots[slotWeapon] != w {
		t.Errorf("readied %d, missile %v, weapon kept %v", j.Readied, e.missile(c) == j, c.Slots[slotWeapon] == w)
	}
	e.ext.throwing = j // thrown straight from the slot: the sword stays readied
	e.ApplyItems(c)
	if c.Slots[slotWeapon] != j || w.Readied != 1 || j.Readied != 2 {
		t.Errorf("throwing: weapon is javelin %v, sword %d, javelin %d", c.Slots[slotWeapon] == j, w.Readied, j.Readied)
	}
	e.ext.throwing = nil
	e.ApplyItems(c)
	e.readyItem(j) // out again
	e.ApplyItems(c)
	if j.Readied != 0 || e.missile(c) != nil {
		t.Errorf("unready: readied %d", j.Readied)
	}
	e.readyItem(w) // no weapon in hand: still the Missiles slot
	e.ApplyItems(c)
	e.readyItem(j)
	e.ApplyItems(c)
	if j.Readied != 2 || c.Slots[slotWeapon] != nil {
		t.Errorf("no weapon: readied %d, weapon %v", j.Readied, c.Slots[slotWeapon])
	}
	// The computer's picks are undone after the battle (restoreLoadouts).
	e.saveLoadout(c)
	e.readyItem(j)
	e.ApplyItems(c)
	e.readyItem(w)
	e.ApplyItems(c)
	e.restoreLoadouts()
	if j.Readied != 2 || c.Slots[slotWeapon] != nil || w.Readied != 0 {
		t.Errorf("restored: javelin %d, sword %d", j.Readied, w.Readied)
	}
	// A launcher goes into the Launcher slot beside the weapon in hand and
	// shoots the Missiles slot's matching ammunition: a bow the arrows (not
	// quarrels), a sling nothing; one kind of ammunition at a time.
	i := slices.IndexFunc(e.party, func(c *Character) bool {
		return e.base(itemShortBow).Classes&e.base(itemSling).Classes&e.base(itemLongSword).Classes&c.ClassMask != 0
	})
	if i < 0 {
		t.Fatal("nobody for a sword, a bow and a sling")
	}
	e.selectMember(i)
	c = e.selected()
	e.ApplyItems(c)
	for _, s := range []int{slotWeapon, slotShield, slotArrows, slotBolts, slotLauncher} {
		if c.Slots[s] != nil {
			e.readyItem(c.Slots[s])
			e.ApplyItems(c)
		}
	}
	sword, bow, arrows, quarrels, sling := e.AddItemCopy(c, e.RandomItem(itemLongSword)), e.AddItemCopy(c, e.RandomItem(itemShortBow)), e.AddItemCopy(c, e.RandomItem(itemArrow)), e.AddItemCopy(c, e.RandomItem(itemQuarrel)), e.AddItemCopy(c, e.RandomItem(itemSling))
	e.readyItem(sword)
	e.ApplyItems(c)
	if dollSlot(e, c, bow) != DollLauncher {
		t.Errorf("bow: doll slot %d, want Launcher", dollSlot(e, c, bow))
	}
	e.readyItem(bow)
	e.ApplyItems(c)
	if bow.Readied != 3 || c.Slots[slotLauncher] != bow || c.Slots[slotWeapon] != sword || e.missile(c) != nil {
		t.Errorf("bow readied %d, sword kept %v, missile without arrows %v", bow.Readied, c.Slots[slotWeapon] == sword, e.missile(c))
	}
	e.readyItem(quarrels)
	e.ApplyItems(c)
	if dollSlot(e, c, quarrels) != DollArrows || e.missile(c) != nil {
		t.Errorf("quarrels: doll slot %d, missile for a bow %v", dollSlot(e, c, quarrels), e.missile(c))
	}
	e.readyItem(arrows) // the Missiles slot holds the quarrels
	if arrows.Readied != 0 {
		t.Error("arrows readied beside quarrels")
	}
	e.readyItem(quarrels)
	e.ApplyItems(c)
	e.readyItem(arrows)
	e.ApplyItems(c)
	if e.missile(c) != arrows {
		t.Errorf("bow and arrows: missile %v (readied %d, %s)", e.missile(c), arrows.Readied, e.ext.note)
	}
	e.readyItem(sling) // the Launcher slot holds the bow
	if sling.Readied != 0 {
		t.Error("sling readied beside the bow")
	}
	e.readyItem(bow)
	e.ApplyItems(c)
	e.readyItem(sling)
	e.ApplyItems(c)
	if e.missile(c) != sling || c.Slots[slotWeapon] != sword {
		t.Errorf("sling: missile %v (readied %d, %s)", e.missile(c), sling.Readied, e.ext.note)
	}
}

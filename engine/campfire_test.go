package engine

import (
	"image"
	"slices"
	"testing"
)

// Camp shows the camp picture (PIC 0x1D) in the view window and its fire
// burns: frames go by on the ticks while camp is up, and stop once it is gone.
func TestCampfireAnimates(t *testing.T) {
	e := quickStarted(t)
	e.CampMenu(nil)
	if !e.animating() || len(e.pic.frames) < 2 {
		t.Fatalf("camp: animating %v, %d frames", e.animating(), len(e.pic.frames))
	}
	first := e.pic.cur
	for i := 0; i < 200 && e.pic.cur == first; i++ {
		e.pic.last -= 1000 // the frame's delay gone by
		e.Apply(Action{Kind: ActTick})
	}
	if e.pic.cur == first {
		t.Fatal("the campfire does not change frames")
	}
	if v := e.View(); v.Window != WinPicture || !v.Ticks {
		t.Fatalf("camp view: window %v, ticks %v", v.Window, v.Ticks)
	}
	e.Apply(Action{Kind: ActClose})
	if e.animInput {
		t.Fatal("the picture still animates after camp")
	}
}

// In camp every member's context menu carries their camp actions (Icon,
// Drop, Memorize for a caster); outside camp it does not.
func TestCampMemberPowers(t *testing.T) {
	e := quickStarted(t)
	has := func(c *Character, k ActKind) bool {
		return slices.ContainsFunc(e.memberPowers(c), func(p Power) bool { return p.Kind == k })
	}
	if has(e.party[0], ActDropMember) {
		t.Fatal("Drop outside camp")
	}
	e.CampMenu(nil)
	for _, c := range e.party {
		if !has(c, ActIconEdit) || !has(c, ActDropMember) {
			t.Fatalf("%s: no camp actions", c.Name())
		}
		if book, _ := e.spellLists(c); len(book) > 0 != has(c, ActPlan) {
			t.Fatalf("%s: Memorize %v with %d spells", c.Name(), has(c, ActPlan), len(book))
		}
	}
}

// The overland publishes its whole map with the party on it, on the
// adventure screen (no classic page).
func TestAreaMapView(t *testing.T) {
	e := quickStarted(t)
	e.beginFlag = 4
	e.push(modeExplore, func(Action) bool { return false }, nil)
	e.area.In3D, e.viewMode, e.area.MapX, e.area.MapY = 0, 2, 7, 29
	e.LoadIconPic("ICON", 0, 9)
	e.areaMapIcons()
	e.publish()
	v := e.View()
	if v.Window != WinAreaMap || v.AreaMap == nil || !v.Adventure {
		t.Fatalf("overland: window %v, map %v, frame %v", v.Window, v.AreaMap != nil, v.Adventure)
	}
	if m := v.AreaMap; m.Party != image.Pt(7, 29) || m.Img.Bounds().Dx() != 44*m.Tile || m.Marker == nil {
		t.Fatalf("overland map: party %v, %v wide (tile %d), marker %v", m.Party, m.Img.Bounds().Dx(), m.Tile, m.Marker != nil)
	}
}

// Icon in a member's camp menu opens the icon editor as a page of its
// own, not under the 3D view.
func TestCampIconEditor(t *testing.T) {
	e := quickStarted(t)
	e.CampMenu(nil)
	e.Apply(Action{Kind: ActIconEdit, N: 0})
	if v := e.View(); v.IconEditor == nil || v.Adventure {
		t.Fatalf("editor %v, adventure %v", v.IconEditor != nil, v.Adventure)
	}
}

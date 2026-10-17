package ui

import (
	"strings"
	"testing"

	"blobber/font"

	"blobber/party"
)

// Every class and sex resolves to an embedded portrait.
func TestPortraits(t *testing.T) {
	for _, class := range []string{"knight", "sorcerer", "monk", "robber", "cleric", "archer", "paladin", "unknown"} {
		for _, sex := range []string{"m", "f"} {
			c := party.NewCharacter("X", class, 1, 0)
			c.Sex = sex
			if img := Portrait(c); img.Bounds().Dx() != PortraitSize {
				t.Errorf("%s/%s: %v", class, sex, img.Bounds())
			}
		}
	}
}

func TestItemIcons(t *testing.T) {
	for name, it := range party.Catalog {
		if itemIcon(it.Icon) == nil {
			t.Errorf("%s: no inventory icon %q", name, it.Icon)
		}
	}
	if itemIcon("key") == nil {
		t.Error("keys have no icon")
	}
}

func TestNPCsAndWrap(t *testing.T) {
	LoadNPCs(strings.NewReader("# comment\nmerchant merchant Mara the Merchant\n"))
	if n := NPCs["merchant"]; n.Name != "Mara the Merchant" || PortraitID(n.Portrait) == nil {
		t.Fatalf("%+v", n)
	}
	if PortraitID("nobody") != nil {
		t.Fatal("unknown portrait")
	}
	ls := wrap("one two three four five six seven|new line", 60)
	if len(ls) < 3 || ls[len(ls)-1] != "new line" {
		t.Fatalf("%q", ls)
	}
	for _, l := range ls {
		if font.Width(l) > 60 {
			t.Fatalf("too wide: %q", l)
		}
	}
}

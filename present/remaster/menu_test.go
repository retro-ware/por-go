package remaster

import (
	"slices"
	"testing"

	"gpu"

	"blobber"
	"por/engine"
)

// Esc closes the command menu (Enter opened it).
func TestEscClosesCommandMenu(t *testing.T) {
	p := &Presenter{active: true, view: &blobber.View{}}
	v := engine.View{Exploring: true}
	if !p.key(13, v) || !p.menu {
		t.Fatal("Enter did not open the menu")
	}
	if !p.key(27, v) || p.menu {
		t.Fatal("Esc did not close the menu")
	}
	p.key(13, v)
	p.Mouse(0, 0, false, false, true, 0, v)
	if p.menu {
		t.Fatal("a right click did not close the menu")
	}
}

// menuNav: Esc is back and never picks; a hotkey picks in either case; a
// hotkey letter not in the text is shown in front.
func TestMenuNav(t *testing.T) {
	sel := 0
	keys := []byte{'C', 'E'}
	if i, back, _ := menuNav(27, &sel, keys); i != -1 || !back {
		t.Fatal("Esc picked a row")
	}
	if i, _, _ := menuNav('e', &sel, keys); i != 1 || sel != 1 {
		t.Fatal("hotkey e did not pick row 1")
	}
	if menuNav(0x5000, &sel, keys); sel != 0 {
		t.Fatal("Down did not wrap")
	}
	if s, at := hotText("Camp", 'E'); s != "[E] Camp" || at != 1 {
		t.Fatal(s)
	}
}

// digitKeys: rows with no letter and that can be picked get 1-9, skipping
// digits already taken, then none.
func TestDigitKeys(t *testing.T) {
	keys := digitKeys([]byte{'B', 0, '2', 0, 0}, func(i int) bool { return i != 3 })
	if string(keys) != "B12\x003" {
		t.Fatalf("%q", keys)
	}
	many := digitKeys(make([]byte, 12), func(int) bool { return true })
	if many[8] != '9' || many[9] != 0 {
		t.Fatalf("%q", many)
	}
}

// editKey: the cursor moves, Backspace and Delete edit at it, typing
// inserts at it; caretLine shows it.
func TestEditCursor(t *testing.T) {
	s, c := "abd", 3
	for _, k := range []uint16{0x4B00, 'c', 0x4700, 0x5300, 'A', 0x4F00, 8} {
		editKey(k, &s, &c, 10, false)
	}
	if s != "Abc" || c != 3 {
		t.Fatalf("%q %d", s, c)
	}
	if editKey(0x4B00, &s, &c, 10, false); c != 2 {
		t.Fatal(c)
	}
	if l := caretLine(s, 1, 4, true); l != "A_c  " {
		t.Fatalf("%q", l)
	}
}

func TestPrice(t *testing.T) {
	for m, want := range map[engine.Money]string{{}: "0g", {Gold: 12}: "12g", {Gold: 12, Copper: 5}: "12g 5c", {Copper: 50}: "0g 50c"} {
		if got := price(m); got != want {
			t.Errorf("price(%v) = %q, want %q", m, got, want)
		}
	}
	if got := withGold("Heal", 7); got != "Heal (party 7g)" {
		t.Errorf("withGold = %q", got)
	}
}

func TestMemberKey(t *testing.T) {
	for _, c := range []struct {
		k      uint16
		cur, n int
		lr     bool
		want   int
		wantOK bool
	}{
		{KeyF1, 2, 4, false, 0, true},
		{KeyF1 + 3<<8, 0, 4, false, 3, true},
		{KeyF1 + 5<<8, 0, 4, false, 5, false}, // no sixth member
		{0x4B00, 0, 4, true, 3, true},         // Left wraps
		{0x4D00, 3, 4, true, 0, true},         // Right wraps
		{0x4D00, -1, 4, true, 0, true},        // none selected: the first
		{0x4B00, 1, 4, false, -1, false},      // Left elsewhere is no member key
		{KeyPadPrev, 1, 4, false, 0, true},    // the stick buttons always are
		{KeyPadNext, 1, 4, false, 2, true},
		{9, 1, 4, true, -1, false}, // Tab is never one (minimap, tabs, targets)
	} {
		if got, ok := memberKey(c.k, c.cur, c.n, c.lr); got != c.want || ok != c.wantOK {
			t.Errorf("memberKey(%#x, %d, %d, %v) = %d, %v; want %d, %v", c.k, c.cur, c.n, c.lr, got, ok, c.want, c.wantOK)
		}
	}
}

// The engine's yes/no question: Y, N, Esc, Enter (on No first) answer it
// with ActYes or ActNo; the arrows move to Yes.
func TestConfirmKeys(t *testing.T) {
	v := engine.View{Ask: &engine.Ask{Kind: engine.AskConfirm, Menu: engine.Menu{Prompt: "Flee?"}}}
	for _, c := range []struct {
		keys []uint16
		want engine.ActKind
	}{{[]uint16{'y'}, engine.ActYes}, {[]uint16{'n'}, engine.ActNo}, {[]uint16{27}, engine.ActNo},
		{[]uint16{13}, engine.ActNo}, {[]uint16{0x4800, KeyPadSelect}, engine.ActYes}} {
		p := &Presenter{confirmSel: 1}
		for _, k := range c.keys {
			p.key(k, v)
		}
		if a := p.Actions(); len(a) != 1 || a[0].Kind != c.want {
			t.Errorf("keys %x: %v", c.keys, a)
		}
	}
}

// The pad reads the binding table: XCOM 2's combat buttons in combat, the
// adventuring ones elsewhere.
func TestPadKeys(t *testing.T) {
	keys := func(combat bool) map[gpu.StandardGamepadButton]uint16 {
		m := map[gpu.StandardGamepadButton]uint16{}
		for _, k := range PadKeys(combat) {
			m[k.B] = k.Key
		}
		return m
	}
	for b, want := range map[gpu.StandardGamepadButton]uint16{dUp: 'w', dLeft: 'a', padA: KeyPadSelect, padB: 27, padLB: 0x4900, padRB: 0x5100,
		padLT: 'q', padRT: 'e', padR3: 0x4700, padY: 8, padX: 'g', padStart: KeyPadStart, padBack: 'l'} {
		if got := keys(true)[b]; got != want {
			t.Errorf("combat button %d sends %#x, want %#x", b, got, want)
		}
	}
	for b, want := range map[gpu.StandardGamepadButton]uint16{dUp: 0x4800, padA: KeyPadSelect, padY: KeyPadMinimap, padX: KeyPadOverview, padStart: 13, padBack: 27, padLT: 0x4700, padR3: KeyPadNext} {
		if got := keys(false)[b]; got != want {
			t.Errorf("button %d sends %#x, want %#x", b, got, want)
		}
	}
}

// Every binding has a keyboard, a mouse and a gamepad way, or says which
// is missing (gap); the help is made from the table, one line each.
func TestBindings(t *testing.T) {
	help := HelpLines()
	for _, b := range bindings {
		if (b.keys == "" || b.mouse == "" || b.pad == "") && b.gap == "" {
			t.Errorf("%s %q: keyboard %q, mouse %q, pad %q and no gap", b.ctx, b.desc, b.keys, b.mouse, b.pad)
		}
		if !slices.Contains(help, [2]string{b.keys, b.desc}) {
			t.Errorf("%s %q has no help line", b.ctx, b.desc)
		}
		for _, s := range []string{b.keys, b.mouse, b.pad} {
			if len(s) > helpKeyW || len(b.desc) > helpDescW {
				t.Errorf("%q / %q too wide for the help", s, b.desc)
			}
		}
	}
}

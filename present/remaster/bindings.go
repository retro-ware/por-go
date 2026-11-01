package remaster

import (
	"gpu"

	"por/present/input"
)

// The one table of the game's bindings: what each key, mouse gesture and
// gamepad button does, by context. The help screen (HelpLines) is made
// from it and the host's gamepad (PadKeys) reads it, so neither can drift
// from the other, and the screens' keyboard dispatch looks the keys up in
// it (actOf, combatAct) and switches on the Act, so none of the three can
// drift. Every action has a keyboard, a mouse and a gamepad way; where one
// is missing, gap says why. Letters and digits that pick a menu's or the
// bar's row are that row's data (its hotkey), and typed text is raw input.

// padKey is a gamepad button and the key it sends.
type padKey struct {
	B      gpu.StandardGamepadButton
	Key    uint16
	Repeat bool // held, it repeats
}

// code is a key code the row binds and the Act it stands for.
type code struct {
	K uint16
	A input.Act
}

// binding is one action in context ctx: its keys, mouse and pad as the
// help shows them, the key codes it binds and their Acts, the buttons the
// pad sends for it, and a missing way.
type binding struct {
	ctx, desc        string
	keys, mouse, pad string
	codes            []code
	padKeys          []padKey
	gap              string // why a way is missing ("" none)
}

// The window's keys as cmd/por sends them, BIOS style: low byte ASCII,
// high byte scan code. The table below is the only place that names them.
const (
	KeyF1          = 0x3B00 // F1; F2..F8 follow at +0x100
	KeyF9          = 0x4300 // save / load
	KeyFontPrev    = 0xFD00 // Ctrl-Shift-F (Ctrl-F is ASCII 6)
	KeyPadSelect   = 0xE000 // gamepad: choose the highlighted (as Enter)
	KeyPadMinimap  = 0xE100 // as Tab
	KeyPadOverview = 0xE200 // as O
	KeyPadPrev     = 0xE300 // gamepad: the previous party member (the left stick's button)
	KeyPadNext     = 0xE400 // ... the next (the right stick's)
	KeyPadStart    = 0xE500 // gamepad Start in combat: the options, even while aiming (where Esc cancels)

	keyUp, keyDown, keyLeft, keyRight = 0x4800, 0x5000, 0x4B00, 0x4D00
	keyPgUp, keyPgDn, keyHome, keyEnd = 0x4900, 0x5100, 0x4700, 0x4F00
	keyDel                            = 0x5300
	keyEnter, keyEsc, keyTab, keyBksp = 13, 27, 9, 8
	keyCtrlS, keyCtrlF                = 0x13, 6
)

// memberCodes are F1-F8 and the member each picks.
func memberCodes() []code {
	c := make([]code, input.Members)
	for i := range c {
		c[i] = code{KeyF1 + uint16(i)<<8, input.Member1 + input.Act(i)}
	}
	return c
}

// helpKeyW and helpDescW are the help's columns: the keys (or the mouse's
// or pad's way), then what they do.
const helpKeyW, helpDescW = 12, 23

// The contexts, in the help's order.
const (
	ctxMenus   = "Menus"
	ctxAdv     = "Adventuring"
	ctxCombat  = "Combat"
	ctxAnytime = "Any time"
)

const (
	dUp, dDown, dLeft, dRight  = gpu.StandardGamepadButtonLeftTop, gpu.StandardGamepadButtonLeftBottom, gpu.StandardGamepadButtonLeftLeft, gpu.StandardGamepadButtonLeftRight
	padA, padB, padX, padY     = gpu.StandardGamepadButtonRightBottom, gpu.StandardGamepadButtonRightRight, gpu.StandardGamepadButtonRightLeft, gpu.StandardGamepadButtonRightTop
	padLB, padRB, padLT, padRT = gpu.StandardGamepadButtonFrontTopLeft, gpu.StandardGamepadButtonFrontTopRight, gpu.StandardGamepadButtonFrontBottomLeft, gpu.StandardGamepadButtonFrontBottomRight
	padBack, padStart          = gpu.StandardGamepadButtonCenterLeft, gpu.StandardGamepadButtonCenterRight
	padL3, padR3               = gpu.StandardGamepadButtonLeftStick, gpu.StandardGamepadButtonRightStick
)

var bindings = []binding{
	{ctx: ctxMenus, keys: "Up Down", desc: "move the highlight", mouse: "point", pad: "D-pad, stick",
		codes:   []code{{keyUp, input.Up}, {keyDown, input.Down}},
		padKeys: []padKey{{dUp, keyUp, true}, {dDown, keyDown, true}}},
	{ctx: ctxMenus, keys: "PgUp PgDn", desc: "ten lines up, down", mouse: "wheel", pad: "LB RB",
		codes:   []code{{keyPgUp, input.PgUp}, {keyPgDn, input.PgDn}},
		padKeys: []padKey{{padLB, keyPgUp, true}, {padRB, keyPgDn, true}}},
	{ctx: ctxMenus, keys: "Home End", desc: "first, last line", mouse: "scroll bar", pad: "LT RT",
		codes:   []code{{keyHome, input.Home}, {keyEnd, input.End}},
		padKeys: []padKey{{padLT, keyHome, true}, {padRT, keyEnd, true}}},
	{ctx: ctxMenus, keys: "Enter Space", desc: "pick the highlighted", mouse: "click", pad: "A, Start",
		codes:   []code{{keyEnter, input.Enter}, {' ', input.Space}, {KeyPadSelect, input.PadA}},
		padKeys: []padKey{{padA, KeyPadSelect, false}, {padStart, keyEnter, false}}},
	{ctx: ctxMenus, keys: "Letter 1-9", desc: "pick that line", mouse: "click", pad: "A"},
	{ctx: ctxMenus, keys: "Y N", desc: "answer a question", mouse: "click", pad: "A"},
	{ctx: ctxMenus, keys: "Esc", desc: "back, No", mouse: "right click", pad: "B, Back",
		codes:   []code{{keyEsc, input.Esc}},
		padKeys: []padKey{{padB, keyEsc, false}, {padBack, keyEsc, false}}},
	{ctx: ctxMenus, keys: "F1-F8", desc: "member 1-8", mouse: "click a card", pad: "L3 R3",
		codes: memberCodes()},
	{ctx: ctxMenus, keys: "Tab", desc: "sheet, items: switch", mouse: "View, Items", pad: "Y"},
	{ctx: ctxMenus, keys: "Left Right", desc: "prev., next member", mouse: "click a card", pad: "L3 R3",
		codes:   []code{{keyLeft, input.Left}, {keyRight, input.Right}, {KeyPadPrev, input.PrevMember}, {KeyPadNext, input.NextMember}},
		padKeys: []padKey{{dLeft, keyLeft, true}, {dRight, keyRight, true}, {padL3, KeyPadPrev, false}, {padR3, KeyPadNext, false}}},
	{ctx: ctxMenus, keys: "+ - Bksp", desc: "spells: one more, less", mouse: "click (more)", pad: "A (more)", gap: "one less is keyboard only",
		codes: []code{{'+', input.More}, {'=', input.More}, {'-', input.Less}}}, // Backspace: the Letters row's

	{ctx: ctxAdv, keys: "Arrows", desc: "walk, turn", mouse: "map: Walk", pad: "D-pad, stick"}, // the Menus rows' arrows; the digits are the keypad (keypadActs)
	{ctx: ctxAdv, keys: "Enter", desc: "command menu", mouse: "click middle", pad: "A, Start"},   // the Menus row's Enter
	{ctx: ctxAdv, keys: "C I V A", desc: "cast items view camp", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAdv, keys: "W", desc: "sweep: search walking", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAdv, keys: "S", desc: "search this square", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAdv, keys: "M", desc: "map: walk, labels", mouse: "minimap", pad: "A, then pick",
		codes: []code{{'m', input.Map}}},
	{ctx: ctxAdv, keys: "Tab", desc: "minimap", mouse: "command menu", pad: "Y",
		codes:   []code{{keyTab, input.Minimap}, {KeyPadMinimap, input.Minimap}},
		padKeys: []padKey{{padY, KeyPadMinimap, false}}},
	{ctx: ctxAdv, keys: "O", desc: "party overview", mouse: "command menu", pad: "X",
		codes:   []code{{'o', input.Party}, {KeyPadOverview, input.Overview}},
		padKeys: []padKey{{padX, KeyPadOverview, false}}},
	{ctx: ctxAdv, keys: "Esc", desc: "options", mouse: "right click", pad: "B, Back"}, // the Menus row's Esc

	{ctx: ctxCombat, keys: "W A S D", desc: "move the cursor", mouse: "point", pad: "D-pad, stick",
		codes:   []code{{'w', input.CursorUp}, {'s', input.CursorDown}, {'a', input.CursorLeft}, {'d', input.CursorRight}},
		padKeys: []padKey{{dUp, 'w', true}, {dDown, 's', true}, {dLeft, 'a', true}, {dRight, 'd', true}}},
	{ctx: ctxCombat, keys: "Enter", desc: "go, attack, press", mouse: "click", pad: "A",
		codes:   []code{{keyEnter, input.Enter}, {' ', input.Space}, {KeyPadSelect, input.PadA}},
		padKeys: []padKey{{padA, KeyPadSelect, false}}},
	{ctx: ctxCombat, keys: "Tab", desc: "target; else button", mouse: "point, hover", pad: "LB RB",
		codes:   []code{{keyTab, input.Tab}, {keyPgUp, input.PgUp}, {keyPgDn, input.PgDn}},
		padKeys: []padKey{{padLB, keyPgUp, true}, {padRB, keyPgDn, true}}},
	{ctx: ctxCombat, keys: "Esc", desc: "cancel; else options", mouse: "right click", pad: "B, Start",
		codes:   []code{{keyEsc, input.Esc}, {KeyPadStart, input.Options}},
		padKeys: []padKey{{padB, keyEsc, false}, {padStart, KeyPadStart, false}}},
	{ctx: ctxCombat, keys: "Letter 0-9", desc: "press a button", mouse: "click it", pad: "LB RB, A"},
	{ctx: ctxCombat, keys: "Backspace", desc: "end the turn", mouse: "click End", pad: "Y",
		codes:   []code{{keyBksp, input.EndTurn}},
		padKeys: []padKey{{padY, keyBksp, false}}},
	{ctx: ctxCombat, keys: "G", desc: "guard", mouse: "click Guard", pad: "X",
		padKeys: []padKey{{padX, 'g', false}}},
	{ctx: ctxCombat, keys: "Q E", desc: "turn the camera", pad: "LT RT", gap: "no mouse way to turn the camera",
		codes:   []code{{'q', input.TurnLeft}, {'e', input.TurnRight}},
		padKeys: []padKey{{padLT, 'q', false}, {padRT, 'e', false}}},
	{ctx: ctxCombat, keys: "Arrows", desc: "pan the camera", pad: "right stick", gap: "no mouse way to pan the camera",
		codes: []code{{keyUp, input.Up}, {keyDown, input.Down}, {keyLeft, input.Left}, {keyRight, input.Right}}},
	{ctx: ctxCombat, keys: "Home", desc: "camera on the active", pad: "R3", gap: "no mouse way to recentre the camera",
		codes:   []code{{keyHome, input.Home}},
		padKeys: []padKey{{padR3, keyHome, false}}},
	{ctx: ctxCombat, keys: "Wheel", desc: "zoom", mouse: "wheel", gap: "no keyboard or pad zoom"},
	{ctx: ctxCombat, keys: "L", desc: "battle log", pad: "Back", gap: "no mouse way to open the log",
		codes:   []code{{'l', input.Log}},
		padKeys: []padKey{{padBack, 'l', false}}},

	{ctx: ctxAnytime, keys: "?", desc: "this help", mouse: "command menu", pad: "A, then pick",
		codes: []code{{'?', input.Help}}},
	{ctx: ctxAnytime, keys: "F9", desc: "save, load", mouse: "Esc menu", pad: "B, then pick",
		codes: []code{{KeyF9, input.SaveLoad}}},
	{ctx: ctxAnytime, keys: "L D", desc: "saves: load, delete", mouse: "click footer", pad: "X Y"},
	{ctx: ctxAnytime, keys: "Ctrl-S", desc: "sound on, off", mouse: "Esc > Audio", pad: "B, then pick",
		codes: []code{{keyCtrlS, input.Sound}}},
	{ctx: ctxAnytime, keys: "Ctrl-F", desc: "font (+Shift: back)", gap: "the font has no mouse or pad way",
		codes: []code{{keyCtrlF, input.Font}, {KeyFontPrev, input.FontPrev}}},
	{ctx: ctxAnytime, keys: "Letters", desc: "type a name, number", gap: "typing has no mouse or pad way (no on-screen keyboard)",
		codes: []code{{keyBksp, input.Backspace}, {keyDel, input.Delete}}},
}

// PadKeys are the gamepad's buttons and the keys they send: in combat
// (with no menu up) the combat rows' first, then the other rows'.
func PadKeys(combat bool) []padKey {
	var out []padKey
	seen := map[gpu.StandardGamepadButton]bool{}
	add := func(in bool) {
		for _, b := range bindings {
			if (b.ctx == ctxCombat) != in {
				continue
			}
			for _, k := range b.padKeys {
				if !seen[k.B] {
					seen[k.B] = true
					out = append(out, k)
				}
			}
		}
	}
	if combat {
		add(true)
	}
	add(false)
	return out
}

// HelpLines are the help screen's lines, made from bindings: a heading per
// context, then each action's keys and what it does, then its mouse and
// pad ways.
func HelpLines() [][2]string {
	var out [][2]string
	for _, c := range []string{ctxMenus, ctxAdv, ctxCombat, ctxAnytime} {
		out = append(out, [2]string{c, ""})
		for _, b := range bindings {
			if b.ctx == c {
				out = append(out, [2]string{b.keys, b.desc})
			}
		}
		for _, dev := range []struct {
			name string
			way  func(binding) string
		}{{"Mouse", func(b binding) string { return b.mouse }}, {"Gamepad", func(b binding) string { return b.pad }}} {
			out = append(out, [2]string{c + ", " + dev.name, ""})
			for _, b := range bindings {
				if w := dev.way(b); b.ctx == c && w != "" {
					out = append(out, [2]string{w, b.desc})
				}
			}
		}
	}
	return out
}

// actOf is what key k does outside combat (the rows of every context but
// Combat); letters match either case. None: the table does not bind k (a
// hotkey or a typed character, which the screen takes as raw input).
func actOf(k uint16) input.Act { return lookup(k, false) }

// combatAct is what key k does on the combat map (the Combat rows).
func combatAct(k uint16) input.Act { return lookup(k, true) }

func lookup(k uint16, combat bool) input.Act {
	if k >= 'A' && k <= 'Z' {
		k += 'a' - 'A'
	}
	for _, b := range bindings {
		if (b.ctx == ctxCombat) != combat {
			continue
		}
		for _, c := range b.codes {
			if c.K == k {
				return c.A
			}
		}
	}
	return input.None
}

// keyOf is the first key code bound to a outside combat: the mouse's and
// wheel's way into the key handlers.
func keyOf(a input.Act) uint16 {
	for _, b := range bindings {
		for _, c := range b.codes {
			if c.A == a && b.ctx != ctxCombat {
				return c.K
			}
		}
	}
	return 0
}

// keypadActs are the digits 1-9 as the keypad's keys (1 End .. 9 PgUp),
// where a screen does not offer the digit itself.
var keypadActs = [...]input.Act{input.End, input.Down, input.PgDn, input.Left, input.None, input.Right, input.Home, input.Up, input.PgUp}

// keypad is digit k as the keypad's key (None: not a digit 1-9).
func keypad(k uint16) input.Act {
	if k >= '1' && k <= '9' {
		return keypadActs[k-'1']
	}
	return input.None
}

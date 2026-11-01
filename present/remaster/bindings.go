package remaster

import "gpu"

// The one table of the game's bindings: what each key, mouse gesture and
// gamepad button does, by context. The help screen (HelpLines) is made
// from it and the host's gamepad (PadKeys) reads it, so neither can drift
// from the other. Every action has a keyboard, a mouse and a gamepad way;
// where one is missing, gap says why.

// padKey is a gamepad button and the key it sends.
type padKey struct {
	B      gpu.StandardGamepadButton
	Key    uint16
	Repeat bool // held, it repeats
}

// binding is one action in context ctx: its keys, mouse and pad as the
// help shows them, the buttons the pad sends for it, and a missing way.
type binding struct {
	ctx, desc        string
	keys, mouse, pad string
	padKeys          []padKey
	gap              string // why a way is missing ("" none)
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
		padKeys: []padKey{{dUp, 0x4800, true}, {dDown, 0x5000, true}}},
	{ctx: ctxMenus, keys: "PgUp PgDn", desc: "ten lines up, down", mouse: "wheel", pad: "LB RB",
		padKeys: []padKey{{padLB, 0x4900, true}, {padRB, 0x5100, true}}},
	{ctx: ctxMenus, keys: "Home End", desc: "first, last line", mouse: "scroll bar", pad: "LT RT",
		padKeys: []padKey{{padLT, 0x4700, true}, {padRT, 0x4F00, true}}},
	{ctx: ctxMenus, keys: "Enter Space", desc: "pick the highlighted", mouse: "click", pad: "A, Start",
		padKeys: []padKey{{padA, KeyPadSelect, false}, {padStart, 13, false}}},
	{ctx: ctxMenus, keys: "Letter 1-9", desc: "pick that line", mouse: "click", pad: "A"},
	{ctx: ctxMenus, keys: "Y N", desc: "answer a question", mouse: "click", pad: "A"},
	{ctx: ctxMenus, keys: "Esc", desc: "back, No", mouse: "right click", pad: "B, Back",
		padKeys: []padKey{{padB, 27, false}, {padBack, 27, false}}},
	{ctx: ctxMenus, keys: "F1-F8", desc: "member 1-8", mouse: "click a card", pad: "L3 R3"},
	{ctx: ctxMenus, keys: "Left Right", desc: "prev., next member", mouse: "click a card", pad: "L3 R3",
		padKeys: []padKey{{dLeft, 0x4B00, true}, {dRight, 0x4D00, true}, {padL3, KeyPadPrev, false}, {padR3, KeyPadNext, false}}},

	{ctx: ctxAdv, keys: "Arrows", desc: "walk, turn", mouse: "click edges", pad: "D-pad, stick"},
	{ctx: ctxAdv, keys: "Enter", desc: "command menu", mouse: "click middle", pad: "A, Start"},
	{ctx: ctxAdv, keys: "C I V A", desc: "cast items view camp", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAdv, keys: "W", desc: "sweep: search walking", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAdv, keys: "S", desc: "search this square", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAdv, keys: "M", desc: "map: walk, labels", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAdv, keys: "Tab", desc: "minimap", mouse: "command menu", pad: "Y",
		padKeys: []padKey{{padY, KeyPadMinimap, false}}},
	{ctx: ctxAdv, keys: "P", desc: "party overview", mouse: "command menu", pad: "X",
		padKeys: []padKey{{padX, KeyPadOverview, false}}},
	{ctx: ctxAdv, keys: "Esc", desc: "options", mouse: "right click", pad: "B, Back"},

	{ctx: ctxCombat, keys: "W A S D", desc: "move the cursor", mouse: "point", pad: "D-pad, stick",
		padKeys: []padKey{{dUp, 'w', true}, {dDown, 's', true}, {dLeft, 'a', true}, {dRight, 'd', true}}},
	{ctx: ctxCombat, keys: "Enter", desc: "go, attack, press", mouse: "click", pad: "A",
		padKeys: []padKey{{padA, KeyPadSelect, false}}},
	{ctx: ctxCombat, keys: "Tab", desc: "target; else button", mouse: "point, hover", pad: "LB RB",
		padKeys: []padKey{{padLB, 0x4900, true}, {padRB, 0x5100, true}}},
	{ctx: ctxCombat, keys: "Esc", desc: "cancel; else options", mouse: "right click", pad: "B, Start",
		padKeys: []padKey{{padB, 27, false}, {padStart, KeyPadStart, false}}},
	{ctx: ctxCombat, keys: "Letter 0-9", desc: "press a button", mouse: "click it", pad: "LB RB, A"},
	{ctx: ctxCombat, keys: "Backspace", desc: "end the turn", mouse: "click End", pad: "Y",
		padKeys: []padKey{{padY, 8, false}}},
	{ctx: ctxCombat, keys: "G", desc: "guard", mouse: "click Guard", pad: "X",
		padKeys: []padKey{{padX, 'g', false}}},
	{ctx: ctxCombat, keys: "Q E", desc: "turn the camera", pad: "LT RT", gap: "no mouse way to turn the camera",
		padKeys: []padKey{{padLT, 'q', false}, {padRT, 'e', false}}},
	{ctx: ctxCombat, keys: "Arrows", desc: "pan the camera", pad: "right stick", gap: "no mouse way to pan the camera"},
	{ctx: ctxCombat, keys: "Home", desc: "camera on the active", pad: "R3", gap: "no mouse way to recentre the camera",
		padKeys: []padKey{{padR3, 0x4700, false}}},
	{ctx: ctxCombat, keys: "Wheel", desc: "zoom", mouse: "wheel", gap: "no keyboard or pad zoom"},
	{ctx: ctxCombat, keys: "L", desc: "battle log", pad: "Back", gap: "no mouse way to open the log",
		padKeys: []padKey{{padBack, 'l', false}}},

	{ctx: ctxAnytime, keys: "?", desc: "this help", mouse: "command menu", pad: "A, then pick"},
	{ctx: ctxAnytime, keys: "F9", desc: "save, load", mouse: "Esc menu", pad: "B, then pick"},
	{ctx: ctxAnytime, keys: "Ctrl-S", desc: "sound on, off", mouse: "Esc > Audio", pad: "B, then pick"},
	{ctx: ctxAnytime, keys: "Ctrl-F", desc: "font (+Shift: back)", gap: "the font has no mouse or pad way"},
	{ctx: ctxAnytime, keys: "Letters", desc: "type a name, number", gap: "typing has no mouse or pad way (no on-screen keyboard)"},
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

package gpu

import "testing"

func TestPads(t *testing.T) {
	q := xinputPad(xinputState{buttons: 0x1000, lt: 100, rt: 200, ly: -32768})
	pads.polled = [maxPads]padState{1: q}
	for range 3 {
		tickPads()
	}
	if ids := AppendGamepadIDs(nil); len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("ids %v", ids)
	}
	if d := StandardGamepadButtonPressDuration(1, StandardGamepadButtonRightBottom); d != 3 {
		t.Errorf("A held %d ticks, want 3", d)
	}
	if !IsStandardGamepadButtonPressed(1, StandardGamepadButtonFrontBottomRight) || IsStandardGamepadButtonPressed(1, StandardGamepadButtonFrontBottomLeft) {
		t.Error("triggers: right past the threshold, left not")
	}
	if v := StandardGamepadAxisValue(1, StandardGamepadAxisLeftStickVertical); v != 1 {
		t.Errorf("left stick down %v, want 1", v)
	}
	pads.polled = [maxPads]padState{}
	tickPads()
	if d := StandardGamepadButtonPressDuration(1, StandardGamepadButtonRightBottom); d != 0 || len(AppendGamepadIDs(nil)) != 0 {
		t.Error("an unplugged pad still holds")
	}
	if StandardGamepadButtonPressDuration(9, 0) != 0 || StandardGamepadAxisValue(-1, 0) != 0 {
		t.Error("out of range")
	}
}

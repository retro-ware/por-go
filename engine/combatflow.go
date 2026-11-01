package engine

import (
	"image"
	"slices"
	"strconv"
)

// The fight as a screen (docs/engine-redesign.md, M6c): Combat's rounds
// are a timed screen (modeCombat) that takes one turn a frame (ActTick);
// the player's turn, aiming and the yes/no questions are screens pushed
// over it, their results handed on by continuations (later). What the
// rules show and how long (the original's Delay and MsgDelay) are
// CombatEvents the presenter plays at its own pace: the engine goes on at
// once, the next turn waits for the presenter's next ActTick.

// EventKind is what a CombatEvent reports.
type EventKind int

const (
	EvFrame   EventKind = iota // a moment of the fight to show for Ms (a missile's step, a blow drawn)
	EvMessage                  // Text went to the Log: shown for Ms to be read
	EvMoved                    // Unit stepped From -> To
	EvAttack                   // Unit attacks Target
	EvHit                      // Unit hits Target for Amount
	EvMiss                     // Unit misses Target
	EvDamage                   // Unit takes Amount damage
	EvSpell                    // Unit casts spell Amount (Text: its name)
	EvMissile                  // a missile flies From -> To
	EvDied                     // Unit fell (off the map)
	EvFled                     // Unit got away
	EvSound                    // sound Amount (Text: the sound pack's event) goes off here (extra: with what it belongs to, PlayEventSound)
)

// CombatEvent is one thing that happened in a fight, in order (View.Events,
// handed out by Apply once). An event held for Ms carries the map and the
// party as they stood (Battle, Party): the presenter shows that while it
// plays the event, the live View when it has played them all.
type CombatEvent struct {
	Kind         EventKind
	Unit, Target int // combatant IDs (Unit.ID; 0: none)
	Amount       int
	From, To     image.Point
	Text         string
	Ms           int
	Key          bool // held until a key ("press any key"), not for Ms
	Battle       *Battle
	Party        []Member
	Scene        *View // outside a fight: the whole page as it stood, shown while the event plays
}

// event records ev; one held for a time gets the scene to show.
func (e *Engine) event(ev CombatEvent) {
	if ev.Ms > 0 || ev.Key {
		ev.Battle, ev.Party = e.battle(), e.members()
	}
	e.events = append(e.events, ev)
}

// pause is a moment for the presenter to show the page as it stands: ms
// long, or until a key (key). The engine goes on at once (EvFrame).
func (e *Engine) pause(ms int, key bool) {
	if ms <= 0 && !key {
		return
	}
	if e.cs.on {
		e.event(CombatEvent{Kind: EvFrame, Ms: ms, Key: key})
		return
	}
	e.publish()
	v := e.View()
	v.Events, v.Ask, v.Ticks = nil, nil, false
	e.events = append(e.events, CombatEvent{Kind: EvFrame, Ms: ms, Key: key, Scene: &v})
}

// unitEvent records an instant event about c (and t).
func (e *Engine) unitEvent(k EventKind, c, t *Character, amount int) {
	if !e.cs.on {
		return
	}
	ev := CombatEvent{Kind: k, Unit: e.combatIndex(c), Amount: amount}
	if t != nil {
		ev.Target = e.combatIndex(t)
	}
	e.event(ev)
}

// combatState is the fight's own state: where in the round it is and whose
// turn has screens up.
type combatState struct {
	on     bool       // a fight is up: Delay and MsgDelay are events
	phase  fightPhase // what the next frame does
	actor  *Character // the member whose turn's screens are up (the turn ends when they are gone)
	resume func() bool
	aims   []aimAnswer // the aims taken before a spell is cast, in the order its targeting asks for them
}

type fightPhase int

const (
	fightRound fightPhase = iota // the round begins
	fightTurn                    // the next one to act
	fightEnd                     // the round ends
	fightOver                    // the battle is over
)

// aimAnswer is one aim the player took (playerAim's results).
type aimAnswer struct {
	found bool
	t     aimTarget
}

// pushCombat = Combat (ovr09:007A): set up the battle, then rounds until
// one side is down (or the party stops fighting), a timed screen.
func (e *Engine) pushCombat() {
	e.beginFlag = 5
	e.ext.battleLog, e.ext.panel = nil, nil
	e.combatTargets = true // DS:6B84 = 0068:0066 (ovr14:20C3)
	e.cs = combatState{on: true}
	e.combatSetup()
	if e.rv.side[0] == 0 || e.rv.side[1] == 0 {
		e.cs.phase = fightOver
	}
	e.pushTimed(modeCombat, e.applyCombat, e.combatEnd)
}

// applyCombat is modeCombat's handler: a frame (ActTick) plays the fight
// on to the next turn taken; a turn's screens gone (actRedraw) end it.
func (e *Engine) applyCombat(a Action) bool {
	if c := e.cs.actor; c != nil {
		if a.Kind == actRedraw {
			e.cs.actor = nil
			e.endTurn(c)
		}
		return false
	}
	if a.Kind != ActTick {
		return false
	}
	for {
		switch e.cs.phase {
		case fightOver:
			return true
		case fightRound:
			e.countSides()
			for _, c := range e.party {
				e.turnSetup(c)
			}
			e.game.Surprise = 0
			e.cs.phase = fightTurn
		case fightTurn:
			c := e.nextCombatant()
			if c == nil {
				e.cs.phase = fightEnd
				continue
			}
			base := len(e.screens)
			if !e.takeTurn(c) {
				e.sidesCheck()
				continue
			}
			if len(e.screens) > base { // the player's turn: it ends when its screens are gone
				e.cs.actor = c
				return false
			}
			e.endTurn(c)
			return false
		case fightEnd:
			e.cs.phase = fightRound
			if e.endRound() {
				e.cs.phase = fightOver
				return true
			}
		}
	}
}

// endTurn = takeTurn's end: the cursor off; the last one down ends the round.
func (e *Engine) endTurn(c *Character) {
	e.eraseCursor(e.combatX(c), e.combatY(c))
	if e.ext.throwing != nil { // extra: guard: a throw never outlasts its turn
		e.ext.throwing = nil
		e.ApplyItems(c)
	}
	e.sidesCheck()
}

// sidesCheck: extra: the last one down ends the battle at once (the round ends).
func (e *Engine) sidesCheck() {
	if e.countSides(); e.rv.side[0] == 0 || e.rv.side[1] == 0 {
		e.cs.phase = fightEnd
	}
}

// later hands the turn its result: f runs once the screens a choice pushed
// are gone (applyTurn), or at once when it pushed none.
func (e *Engine) later(f func() bool) { e.cs.resume = f }

// confirm (extra) asks combat's yes/no question q (modeConfirm, View.Ask
// AskConfirm): ActYes, or ActNo / ActClose answers it, then gets it.
func (e *Engine) confirm(q string, then func(yes bool)) {
	yes := false
	e.snap.ask = q
	e.push(modeConfirm, func(a Action) bool {
		yes = a.Kind == ActYes
		return yes || a.Kind == ActNo || a.Kind == ActClose
	}, func() {
		e.snap.ask = ""
		then(yes)
	})
}

// aimSpell takes the aims c's spell will ask for (chooseSpellTargets with
// the player aiming) before cast casts it, which then takes them in order:
// the rules run straight through, in the same order and with the same
// dice as when they asked in the middle.
func (e *Engine) aimSpell(c *Character, spell int, cast func()) {
	e.cs.aims = nil
	kind := e.spellData(spell, 0) & 0xF
	if !e.combatTargets || kind == 0 {
		cast()
		return
	}
	aoe, bolt := 1, bolts[spell]
	if kind > 8 && kind <= 0xE {
		aoe = int(kind&7) + 1
	}
	var aims []aimAnswer
	one := func(allowGround int, then func(aimAnswer)) {
		e.ext.aoe, e.ext.bolt, e.ext.aoeSpell = aoe, bolt, spell // the area preview, as chooseSpellTargets shows it
		e.ext.enemiesOnly = spellTable[spell&0xFF][7] == 0
		e.playerAim(c, e.spellDuration(spell), 0, allowGround, func(found bool, t aimTarget) {
			e.ext.aoe, e.ext.bolt, e.ext.aoeSpell, e.ext.enemiesOnly = 0, [2]int{}, 0, false
			a := aimAnswer{found, t}
			aims = append(aims, a)
			then(a)
		})
	}
	done := func() {
		e.cs.aims = aims
		cast()
		e.cs.aims = nil
	}
	switch {
	case kind == 0xF:
		one(0, func(aimAnswer) { done() })
	case kind >= 8 && kind <= 0xE:
		one(1, func(aimAnswer) { done() })
	default: // up to (kind&3)+1 different targets, as chooseSpellTargets' loop
		left := int(kind&3) + 1
		var picked []*Character
		var next func()
		next = func() {
			if left == 0 {
				done()
				return
			}
			one(0, func(a aimAnswer) {
				switch {
				case !a.found:
					left--
				case slices.Contains(picked, a.t.c):
					e.say("Already been targeted") // the cast logs it as it goes over the aims
				default:
					picked = append(picked, a.t.c)
					left--
				}
				next()
			})
		}
		next()
	}
}

// aimNow is chooseSpellTarget's aim: the next one aimSpell took (none
// left: not found, as a cancelled aim).
func (e *Engine) aimNow(c *Character, rng, allowGround int) (found bool, t aimTarget) {
	if len(e.cs.aims) > 0 {
		a := e.cs.aims[0]
		e.cs.aims = e.cs.aims[1:]
		return a.found, a.t
	}
	return
}

// members is the party's cards (View.Party).
func (e *Engine) members() []Member {
	var out []Member
	sel := e.rosterSel() // a member pick shows its candidate
	for i, c := range e.party {
		if !isParty(c) { // the monsters loaded for a fight sit on the same list: no cards for them
			continue
		}
		out = append(out, Member{
			Name: c.Name(), HP: int(c.HP), MaxHP: int(c.MaxHP),
			AC: 60 - int(c.AC[0]), Status: e.statusName(c), Selected: i == sel, Colour: rosterColour(c),
			Class: className(c.Class), Head: e.charHead(c), Portrait: c.Portrait, Powers: e.memberPowers(c),
		})
		m := &out[len(out)-1]
		from, to := levelSpan(c)
		m.Exp, m.LevelExp, m.NextExp = int(int32(c.Exp)), int(from), int(to)
		{
			ready, slots := 0, 0
			for _, s := range c.Memorized {
				if s > 0 && s < 0x80 {
					ready++
				}
			}
			for _, n := range c.SpellSlots[:6] {
				slots += int(n)
			}
			if slots > 0 {
				m.Spells = strconv.Itoa(ready) + "/" + strconv.Itoa(slots)
				if e.mode() == modeCamp {
					m.Memo = m.Spells
				}
			}
		}
	}
	return out
}

// stepMs is how long the presenter shows each combat step (EvMoved).
const stepMs = 160

// PlayEventSound plays an EvSound when the presenter reaches it, so a
// fight's sounds go with what they belong to (a blow with its lunge),
// not all at once when the rules ran.
func (e *Engine) PlayEventSound(ev CombatEvent) {
	e.soundNow = true
	e.playSound(ev.Amount, ev.Text)
	e.soundNow = false
}

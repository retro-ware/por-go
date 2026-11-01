package engine

import "time"

// Actions (extra): the remastered screens act on the game through typed
// actions instead of keys. A screen ported to them (ItemsMenu ...) publishes
// what it shows in the View; the front end hands decisions to Apply.

// ActKind is what an Action does.
type ActKind int

const (
	ActNone      ActKind = iota
	ActClose             // leave the screen
	ActMember            // show party member N
	ActLead              // make party member N the selected one (the command bar's member): F1-F8, the member keys; in a member pick the candidate
	ActReady             // inventory: ready or unready item N
	ActUse               // inventory: use item N
	ActRanged            // combat: throw the Missiles slot's weapon (aims as Attack)
	ActDrop              // inventory: drop item N (the player already confirmed)
	ActSell              // inventory: sell item N (confirmed)
	ActWalk              // combat: walk to square X, Y (one of Battle.Reach)
	ActAttack            // combat: attack unit N (one of Battle.Targets; 0: aim first)
	ActGuard             // combat: end the turn on guard
	ActBandage           // combat: bandage the first dying member
	ActWait              // combat: Done's Delay
	ActEnd               // combat: Done's Quit: end the turn
	ActQuick             // combat: the computer fights for the member
	ActTurn              // combat: turn undead
	ActCast              // cast spell N (one of Turn.Spells in combat, of View.Ask (AskSpell) elsewhere)
	ActItems             // combat: the inventory
	ActAim               // combat, aiming: take square X, Y
	ActPoint             // the cursor is on square X, Y: combat's aim (the preview follows), the full map's (View.MapCursor)
	ActYes               // yes to View.Ask (AskConfirm)
	ActNo                // no to View.Ask (AskConfirm); ActClose too
	ActFlee              // combat: step off the map's edge and try to run away (Turn offers it at the edge)
	ActChoose            // choose option N of View.Ask (X: the line of its List)
	ActOK                // Enter: the highlighted choice, or "press a key"
	ActText              // the line being typed (View.Ask) now reads Text
	ActInventory         // member N's items (a card's left click)
	ActSheet             // the inventory: back to the sheet under it
	ActMinimap           // the minimap over the roster (Tab)
	ActMap               // the full map (M)
	ActOverview          // the party overview, on the View page the items (P)
	ActHelp              // the help pages (?)
	ActSlots             // the save slots (F10)
	ActFont              // the next text font (N -1: the one before)
	ActSound             // sound on or off (Ctrl-S)
	ActResize            // the window shows X by Y text cells (40x25 at least): the page takes that size and the screen draws itself again
	ActSpells            // inventory: the member's memorised spells
	ActMagic             // inventory: the spell effects on the member
	ActHeal              // inventory: a paladin lays on hands
	ActCure              // inventory: a paladin cures disease
	ActMainMenu          // drop the game in play for the party menu (the pause menu's Back to Main Menu)
	ActBuy               // shop inventory: buy ware N (Inventory.Wares)
	ActTakeAll           // loot inventory: take every piece of treasure (Inventory.Loot)
	ActHead              // member N's next paper-doll head (N < 0: the selected member); Shift-click on the sheet's portrait
	ActPortrait          // the PNG at Text is member N's portrait (N < 0: the selected member); a file dropped on the sheet
	ActIcon              // the PNG at Text is the member's combat icon; a file dropped on the icon editor (View.IconDrop)
	ActOrder             // member N takes place X in the party (View.Ordering): a card dragged onto another's

	// Adventuring decisions (View.Exploring)
	ActForward   // a step ahead (3D view)
	ActBackward  // a step back, facing kept (3D view)
	ActTurnLeft  // face left (3D view)
	ActTurnRight // face right (3D view)
	ActGo        // a step towards Dir N (North..West) on the area map
	ActCamp      // make camp
	ActSearch    // search this square once
	ActSweep     // the slow, searching pace on or off
	ActCastPick  // member N (< 0: the selected one) picks a memorised spell to cast (View.Ask (AskSpell))

	// Camp decisions (View.Camp); N is the member (< 0: the selected one)
	ActRest       // rest: N is RestHeal .. RestUntil
	ActPlan       // member N memorizes spell X on every rest Y (+1 or -1) times more
	ActScribe     // member N scribes spell X from a scroll on the next rest, or no longer
	ActDropMember // member N leaves the party (the player confirmed)
	ActIconEdit   // member N's combat icon editor

	// Temple, shop and training decisions (View.Temple, View.Shop, View.Training)
	ActTempleCure // member N gets temple service X (View.Temple.Cures), paid (the player agreed)
	ActAppraise   // appraise a gem (N 0) or a jewel (N 1)
	ActKeep       // keep (N 1) or sell (N 0) the one appraised
	ActIdentify   // identify member N's item X (View.Shop.Unknown), paid (the player agreed)
	ActTrain      // train the selected member (the player agreed)
	ActLearn      // spell X goes into the book of the member trained (View.Training.Learn)

	// Party menu decisions (View.MainMenu); N is the member (< 0: the
	// selected one) where one is meant; ActMember views, ActDropMember drops
	ActCreate     // create a character
	ActModify     // member N's sheet to change
	ActAdd        // add saved characters to the party
	ActForget     // forget a saved character
	ActRemove     // member N goes back to the saved characters
	ActTraining   // the training hall for member N
	ActLoad       // load a saved game
	ActQuickStart // a ready-made party
	ActBegin      // begin adventuring
	ActQuit       // quit the game (the player confirmed)

	// Creation and Modify decisions (View.Creation), the pickers (View.Picker),
	// the icon editor (View.IconDrop) and the save slots (View.Slots)
	ActRoll       // roll the abilities (N 1: until the minimums Text, "STR INT WIS DEX CON CHA", met)
	ActSetField   // creation field N (FieldSex ..) takes its option X (Creation.Fields[N].Options)
	ActName       // the character's name is Text
	ActStat       // ability N is Text (STR as 18/xx)
	ActPicker     // open picker N (PickHead, PickBody: the sheet; PickIconHead, PickWeapon: the icon editor)
	ActPicked     // the picker takes picture N (the player agreed to Picker.Ask)
	ActPage       // the picker shows page N; a pager (help, the journal) goes N (+1 or -1) on: a picture page scrolls first
	ActIconSize   // icon editor: small or large
	ActIconColour // icon editor: part N's colour X (0, 1) becomes Y (Y < 0: only selects that swatch)
	ActSave       // save the game in slot Text (the player agreed to an overwrite)
	ActDelete     // delete the save Text (the player agreed)

	ActTick // a frame went by: the engine goes on in a timed loop (View.Ticks); never queued
)

// Action is one thing the player did on a remastered screen.
type Action struct {
	Kind ActKind
	N    int
	X, Y int    // a combat map square, a text cell (ActSpot)
	Text string // ActText
}

// Start runs body (Run, or Play after Init), which puts the first screen
// up, and returns the View.
func (e *Engine) Start(body func()) View {
	body()
	return e.Apply()
}

// Apply hands each decision to the screen on top (ActTick: a frame went
// by, for a timed screen, the auto-walk or the picture's animation) and
// returns the View it leaves, with the events since the last Apply. With
// no screen up, the party menu starts a game.
func (e *Engine) Apply(as ...Action) View {
	for _, a := range as {
		e.apply(a)
	}
	if e.over && e.mode() == modeExplore { // a script's quit, from a continuation
		e.feed(Action{Kind: actRedraw}, 0)
	}
	if len(e.screens) == 0 {
		e.PartyMenu(e.GameLoop) // ovr17:0161, then ovr04:32F8
	}
	e.publish()
	v := e.View()
	v.Events, e.events = e.events, nil
	return v
}

// apply gives one decision to the top screen.
func (e *Engine) apply(a Action) {
	switch a.Kind {
	case ActMainMenu:
		e.mainMenu()
		return
	case ActResize:
		e.resizePage(a.X, a.Y)
		return
	case ActTick:
		e.walkStep()
		switch {
		case e.ext.held.Kind != ActNone: // the walk's step
		case e.timed():
			e.feed(a, 0)
		default:
			e.animateWhileWaiting()
		}
	default:
		if e.ext.walk != nil { // a key or a click stops the walk and is used up
			e.ext.walk = nil
			if e.exploring() {
				return
			}
		}
		if a = e.take(a); a.Kind != ActNone {
			e.feed(a, 0)
		}
	}
	for e.ext.held.Kind != ActNone { // the walk's step, a pick's follow-up
		h := e.ext.held
		e.ext.held = Action{}
		if h = e.take(h); h.Kind != ActNone {
			e.feed(h, 0)
		}
	}
}

// say shows s as the window's note: what a direct action reports instead
// of the original's message line. It is cut to the bottom line (noteCols).
func (e *Engine) say(s string) { e.ext.note, e.ext.noteAt = cut(s, noteCols), time.Now() }

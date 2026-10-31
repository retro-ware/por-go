# Engine redesign: decisions in, state out

Decided 2026-10-31. Replaces the ported control flow of the original game.

## Why

The engine still runs the original's control flow on its own goroutine: it
decides when input happens, blocks inside deep call stacks (camp → rest →
heal; a script instruction → a menu; a combat turn → aim → confirm) and
talks to the front end through the original's screens (menu bars, list
cursors, "press Enter"). The front end translates its UI into those screens
and back. That limits what the front end can offer and every translation is
a chance for errors.

## Target

- **The front end owns flow and presentation.** It decides which screen is
  open, when it asks the player, what is shown, in what order, with what
  animation and timing. Cursors, highlights, paging, hotkeys, menu bars do
  not exist in the engine.
- **The engine owns game state and rules.** Party, characters, items,
  spells, map, position, time, money, combat state, script state. Rules are
  plain functions on that state.
- **One call, no goroutine:** `state, events := engine.Apply(decision)`.
  A decision is something the player decided (`Rest{Heal}`, `Walk{Forward}`,
  `Cast{caster, spell, target}`, `Buy{item}`, `Choose{option}`); it either
  applies or is refused with a reason. Events say what changed (`MapChanged`,
  `TimePassed`, `MemberHealed`, `CombatStarted`, `ScriptAsks{...}`) so the
  front end can animate them; the state is the truth.
- **What the engine needs from the player is state, not a blocking call.**
  When game content needs an answer (a script's question, a combat turn), the
  engine records it as pending (`State.Pending`) and returns; the next
  decision answers it. The front end chooses how and when to present it.

## What stays

- The data (DAX, ECL, GEO, MON*, items, spells) and the rules ported from
  the original (dice, THAC0, saves, spell effects, treasure, training).
- Area scripts (ECL) are game content: they keep running in the script VM,
  but the VM suspends at any instruction that needs the player (menus, text
  to acknowledge, Parlay) and resumes on the answering decision. Its
  "screens" become `ScriptAsks` data (prompt, options, portrait, text), not
  drawn pages.

## What goes

- The engine goroutine, the action channel, `Act`/`nextAct`/`pace`/`Busy`.
- `DisplayInput`, `Bar`, `ListMenu`, menu highlight and hotkey state,
  `ActMove`/`Dir*`/`ActContinue`/`ActSelect`/`ActPick`, nav.go regions.
- The engine's page drawing for UI (text cells, boxes, menus); sleeps for
  animation timing.

## Screens taking decisions

A subsystem converted to decisions keeps an explicit mode, `e.mode`
(engine/decide.go: modeTemple, modeShop, modeTraining, modeCamp, modeItems,
modeSheet, modeParty), with its state in
typed fields, and a handler `func (e *Engine) applyX(a Action) (done bool)`
that applies one decision and returns: it never blocks, sleeps or calls
DisplayInput, ListMenu, YesNo, act or decision. Refusals go to the note
(`e.say`). Until M6 removes the goroutine, the legacy callers run it with
`e.await(mode, handler)`, which feeds it `e.decision()` until done; M6
replaces await with `Apply` dispatching on `e.mode`. A handler opens
another screen with `e.push(mode, handler, leave)`: the pushed screen takes
the decisions until it is done, then `leave` runs and the screen under it
gets `actRedraw` (the temple's View pushes the sheet, the sheet its
inventory). `e.until(base)` is the blocking adapter for pushed screens.

## Milestones

Each leaves the game playable and `go test ./engine` green; the old flow
is deleted subsystem by subsystem, not kept alongside.

1. **Core API.** `State`, `Decision`, `Event`, `Apply`, refusals. The
   adventuring loop as decisions: step, turn, the map, time. Front end walks
   the 3D view through `Apply`.
2. **Camp.** Rest (heal, memorize, wait), memorize/scribe plans, cast out of
   combat, party order, drop, Bestiary as front-end screens over state.
3. **Party and characters.** Party menu, creation, view sheet, inventory
   (ready, use, drop, trade), training, temple, shops.
4. **Scripts.** VM suspends on asks; text, menus, Parlay, encounters,
   pictures as `ScriptAsks`.
5. **Combat.** Explicit combat state (turn order, whose turn, pending
   target); player and AI turns as decisions; animations from events.
6. **Removal.** Goroutine, channel, page drawing, old screens and Acts gone.

## Progress

- [x] M1a: adventuring is decisions (ActForward, ActBackward, ActTurnLeft,
  ActTurnRight, ActGo, ActCamp, ActSearch, ActSweep, ActCastPick, the
  member acts); the command bar is gone, the presenter owns the command
  menu (present/remaster/explore.go); View.Exploring, View.CanCast.
- [x] M2: camp takes decisions (ActRest, ActPlan, ActScribe, ActDropMember,
  ActIconEdit, the member acts, ActClose leaves); View.Camp publishes the
  spell book plan, the scrolls, healing and the bestiary; the presenter
  owns the camp screens (present/remaster/camp.go). The camp bars, Alter,
  Party Order (cards are dragged) and the bestiary page are gone.
- [x] M3a: questions and lists are data. DisplayInput publishes View.Menu
  (prompt, options, Arrows when the screen takes the arrows itself) and
  ListMenu View.List; the answer is ActChoose N (X: the line, N -1: the
  line itself). The engine keeps no highlight, list scroll, menu bar or
  hover; the presenter owns them all (present/remaster/list.go, the main
  menu's highlight, page questions as a popup). ActPick/ActSelect are gone.
- [x] M3b: the temple, the shop and training take decisions through
  handlers (applyTemple, applyShop, applyTraining) on `e.mode`, run by
  `e.await` until M6. ActTempleCure, ActAppraise, ActKeep, ActIdentify,
  ActTrain, ActLearn; View.Temple, View.Shop, View.Training. The
  questions (not needed, pay, deal, train, sell or keep) are the
  presenter's (present/remaster/temple.go, shop.go, train.go over camp's
  screens). The View and Browse screens they open are still the legacy
  inventory and character screens.
- [x] M3c1: the party menu, the character sheet and the inventory take
  decisions (applyParty, applySheet, applyItems) on a screen stack
  (`e.push`); the temple's View and the shop's Browse push them, so do
  camp's View and Items. New ActCreate, ActModify, ActAdd, ActForget,
  ActRemove, ActTraining, ActLoad, ActQuickStart, ActBegin, ActQuit;
  View.MainMenu is the functions open ([]ActKind), View.Sheet the sheet's
  Items and powers, InvItem.Scribe. The presenter orders the main menu,
  owns its Party submenu and asks Quit and Drop (twice); it draws the
  sheet's View menu (present/remaster/sheet.go) and asks before a scroll
  someone would scribe from goes. Camp-style menus fit the window (moved
  up, else smaller). Still blocking, called from the handlers: Create,
  Modify (the creation sheet, its drop-downs, pickers, typed fields, the
  icon editor), Add and Forget (pickSaved), Load (the slot screen), Quick
  Start, and the powers and Use (casting, the member pick: M4/M5).

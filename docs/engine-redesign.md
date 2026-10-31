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
inventory). Since M6b the engine's top level (`top`) only dispatches each
decision to the top screen; `e.inline(push)`/`e.until(base)` is the
blocking adapter left for callers inside combat and the script VM.

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
- [x] M3c2: creation and Modify (applyCreation), the grid pickers (heads,
  bodies, icon parts, the saved characters of Add and Forget), the icon
  editor and the save slots (F9, Load) take decisions on the screen stack;
  the legacy() path is gone and Quick Start no longer waits. New ActRoll,
  ActSetField, ActName, ActStat, ActPicker, ActPicked, ActPage,
  ActIconSize, ActIconColour, ActSave, ActDelete (ActKeep keeps, ActLoad
  with Text loads a slot); View.Creation (the fields and their options,
  the sheet's clickable cells), View.Picker (the page's cells, the pager),
  View.Slots. The presenter owns the drop-downs, the typed lines, the
  picker's cursor and frame, the slot list and every question (Abandon,
  Forget, Overwrite, Delete): present/remaster/screens.go. Still blocking:
  F9 from a blocking prompt runs the slot screen through `until`; the icon
  editor keeps the swatch the palette sets; Minimal gives up after 200000
  rolls instead of stopping on a key.
- [x] M4a: the script VM suspends. An instruction needing the player
  (HMENU, VMENU, INPUT string and number, WHO, the encounter menu, Parlay,
  DELAY, the DAMAGE "press Enter", the tavern's native gamble) records a
  ScriptAsk (eclask.go: AskMenu, AskList, AskText, AskMember, AskPause)
  with its completion and returns; RunECL puts modeScript up
  (applyScript) and goes on once the answer completed it; a completion
  may ask again (WAIT, the next gamble question). View.Script publishes
  the ask; View.Menu, List, Typing and Choosing still draw it. The
  engine no longer sleeps in DELAY: the presenter paces AskPause and
  sends ActContinue. Still blocking: Combat, the shop and temple a
  script opens (opCombat, until M5/M6 lets RunECL return), the journal
  pager (tavern tales), the story and title animations, the paladin's
  Heal and Cure, out-of-combat casting (spells1) and items' Use.
- [x] M4b: every question outside combat is an ask. DisplayInput (so
  YesNo, ListMenu, MenuInput, SelectCharacter), the typed line (typed),
  "press any key" (anyKey: the text pager, PromptRow24, HitMessage, the
  journal, history, help), the spell pick (AskSpell) and every pause
  outside combat (Delay, MsgDelay, DisplayMessageWait, WaitSeconds: the
  story, the title) record an eclWait and wait for it under modeScript
  through `e.block` (eclask.go): View.Script publishes it (new AskKey,
  AskSpell), the presenter answers or paces it; the engine sleeps only in
  combat (Delay with beginFlag 5). InputString and InputNumber are gone.
  The callers (paladin Heal/Cure, casting, Use, loot, Locked, the map,
  camp) are still straight-line code over `block`, the blocking adapter
  until M6, as await. Still blocking otherwise: combat's act() and ask
  (Flee?, Attack Ally?) and its sleeps (M5); F9 from a prompt runs the
  slot screen through `until`; the icon editor's swatch; opCombat and the
  shop, temple and training a script opens run inside RunECL (it cannot
  return mid-script until M5/M6 keeps the VM's state across them).
- [x] M5a: combat no longer sleeps or loops on act. Delay is always an
  AskPause the presenter paces (combat's moves, hits and messages too);
  the player's turn is modeTurn (applyTurn), aiming modeAim, and the
  combat yes/no (Flee?, Attack Ally?) modeConfirm (Battle.Ask, ActYes or
  ActClose), all run by `await` until M6. Scripted captures flush the
  presenter's own decisions every frame. Still to do (M5b): typed combat
  events (moved, hit, damage, missile, died) instead of pacing by pauses,
  the AI's and auto-combat's turns as presenter-requested steps (they run
  straight through combatLoop between player turns), and the calls under
  applyTurn that still block (walk, casting, aiming, the inventory and
  sheet through `await`/`block`).
- [x] M6a: no engine goroutine. The action channel, Act, Settled, Busy,
  Idle, pace, actsDone and the idle sleep are gone; the front end calls
  `e.Start(body) View` once (body: Run, or Play after Init; cmd/por starts
  it from the render loop, whose locked thread must resume it) and then
  `e.Apply(as ...Action) View`: the decisions are queued, the engine runs
  until it waits again and the View comes back. The hand-off is a strict
  coroutine (iter.Pull, act.go): every wait funnels through nextAct
  (decision, await, until, block, waitScript, input, act), which yields
  only with the queue empty. Timed loops (the view picture's animation,
  the auto-walk, the title's lightning, the story's frames, camp rest)
  park in `idle`, which yields with View.Ticks set; the front end sends
  ActTick (never queued) once a frame while Ticks is up, so the engine
  never runs on its own. decision() takes frames only while the picture
  animates or the party walks, else it waits for a decision.
  View.Working and View.Ticks are filled live by View() (a decision still
  queued; a timed loop). The tests drive Apply directly (settle ticks
  while Ticks, bounded) and run in about a second. The coroutine stays
  until the straight-line flows are flattened (M6b+). Callers still
  waiting in place (grep of await/until/block/waitScript/input/idle):
  await: CampMenu, playerTurn, combat's ask, playerAim, PartyMenu, Shop,
  Temple, waitScript; until: extraAct (F9's slot screen), TrainCharacter,
  ViewCharacter, ItemsMenu; block: Delay (so MsgDelay, WaitSeconds and
  every combat pause), typed, DisplayInput (YesNo, ListMenu, MenuInput,
  SelectCharacter), anyKey (pager, PromptRow24, HitMessage, help),
  pickSpell; waitScript: RunECL (the VM inside the script's opCombat,
  shop, temple, training) and the dev scenes; input: showJournal, keyHit
  (the title, rest interruptions); idle: camp rest, the story, the title
  and the adventuring prompt's decision loop; and combatLoop as a whole
  (the AI's and auto-combat's turns run straight through).
- [x] M6b: the top level and the screens. The engine's coroutine body is
  `top`: it dispatches each decision to the screen on top of the stack
  (with none up, the party menu starts a game); a slot load or Back to
  Main Menu unwinds to it (adventure and guardedRun are gone). The
  adventuring loop is modeExplore (applyExplore: GameLoop puts it up,
  afterCommand/lookDone/stepDone are its loop, commandBegin/End
  CommandInput's ends; a script's quit ends the game in its leave). The
  title (modeTitle), camp's rest (modeRest, Rest/restFor/healParty with
  continuations) and the finale (modeStory) are timed screens
  (pushTimed): a frame with no decision gives their handler ActTick.
  PartyMenu, CampMenu, Encamp, Shop and Temple push and return, the work
  after them in `leave`/a continuation; ViewCharacter, ItemsMenu and
  TrainCharacter are gone (pushSheet, pushItems, pushTraining); F9 only
  pushes the slot screen; keyHit is gone. Still blocking (M6c/M6d):
  `inline` at the callers in combat (the turn's items and sheet), loot,
  the script VM (opShop, opTemple, opProgram's party menu, encamp and
  finale) and the dev scenes; `await` in combat's turn, aim, confirm and
  waitScript; `block` (Delay, typed, DisplayInput, anyKey, pickSpell)
  inside rules and timed handlers (rest's Stop Resting?, MsgDelay);
  showJournal's input loop; combatLoop as a whole. Every handler still
  runs inside the coroutine (Apply resumes it): any of them may reach a
  `block` (RunECL, YesNo), so calling them directly from Apply waits for
  M6c/M6d.

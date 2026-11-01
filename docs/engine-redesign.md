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
inventory). Since M6d `Apply` dispatches each decision to the top screen and
returns; work after a screen goes in its leave or a continuation
(`after`, `whenDown`), and the engine's own prompts are asks put up by
`asking`/`displayThen`. There is no coroutine and no blocking adapter.

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
- [x] M6c: combat. The fight is a timed screen (modeCombat, combatflow.go):
  a frame (ActTick) plays the rounds on to the next turn taken (the AI's
  and auto-combat's one a frame), a player's turn is modeTurn pushed over
  it; its end (endTurn) waits until the turn's screens are gone. Aiming
  (modeAim), the yes/no (modeConfirm: Flee?, Attack Ally?), the turn's
  inventory and sheet are pushed screens whose results come back by
  continuation (later, then); a spell's aims are taken before it is cast
  (aimSpell) and its rules take them in order (aimNow), so they run
  straight through with the same dice. In a fight Delay and MsgDelay are
  CombatEvents (EvFrame, EvMessage, carrying the map and party as they
  stood) beside the typed ones (EvMoved, EvAttack, EvHit, EvMiss,
  EvDamage, EvSpell, EvMissile, EvDied, EvFled); Apply hands them out
  (View.Events) and the presenter plays them at its pace (replay in
  present/remaster/fight.go), holding ActTick and decisions until done; a
  key cuts a moment short. The loot screen is displayThen questions and
  pushed inventory and sheet (after); AfterCombat and Combat are `inline`
  at opCombat and the dev scenes until M6d. Still blocking (M6d): `inline`
  at opCombat (Combat, AfterCombat), the script VM's opShop, opTemple,
  opProgram and the dev scenes; inside them showResult's and hiredShare's
  press Enter and the loot's Detect (DisplayInput, CastSpell's asks); an
  item's Use in combat aims inline (aimNow without aimSpell); `await` in
  waitScript; `block` (Delay outside combat, typed, DisplayInput, anyKey,
  pickSpell) in rules and timed handlers; showJournal's input loop.
- [x] M6d: the coroutine is gone. `Apply` hands each decision to the top
  screen (act.go: ActTick to a timed screen, the auto-walk's step or the
  picture's animation; a slot load or Back to Main Menu unwinds by panic
  to `catch`) and returns; `Start` runs the body that puts the first
  screen up. The script VM is resumable (eclRun): its state is the
  engine's (eclPC, es); an instruction that asks or puts screens up
  (opCombat's fight and AfterCombat, shop, temple, opProgram's party
  menu, camp and finale, the tavern tale's journal) suspends it and
  `whenDown` resumes it once they are gone; GameLoop, stepDone, Move
  (the locked door), AreaChange and Encamp chain by continuations.
  Pauses and "press a key" outside a fight (Delay, MsgDelay,
  DisplayMessageWait, PromptRow24, HitMessage, the results' press Enter)
  are events (EvFrame with Scene, the whole page; Key: until a key) the
  presenter plays like the fight's. Every question is a continuation:
  YesNo, SelectCharacter, ListMenu, typed, pickSpell, keyThen (Help, the
  journal), rest's Stop Resting?, the full map, the paladin's powers;
  casting asks first (castAsks: the yes/no of a combat-only spell, the
  member, the aims through aimSpell, also for an item's Use in combat)
  and CastSpell runs straight through. The dev scenes push. Fixed: the
  cast-cancelled flag stayed set after a cancelled aim of a spell going
  off at the start of the turn.

The redesign is done: decisions in (Apply), state and events out.

Audit (2026-11-01), cuts with behaviour kept: `e.mode` is `mode()`, the
top screen's (the field and `screen.under` are gone); a slot load and Back
to Main Menu drop the stack (`unwindReset`, which `feed` notices) and put
the next screen up where they are raised: no panics, no `catch`, no
`ext.guarded`. Every question is one `Ask` (eclask.go: AskMenu, AskList,
AskText, AskNumber, AskMember, AskPause, AskKey, AskSpell) published as
`View.Ask`; View.Menu, List, Script, Typing, Choosing and SpellPick are
gone, the presenter reads them through askMenu/askList/typing/choosing/
spellPick. Arrows are the presenter's: ActMove, the Dir constants and
Menu.Arrows are gone, ActContinue is ActOK; the full map's cursor is
ActPoint (View.MapCursor, its clicks too), pagers take ActPage -1/+1,
Home/End and a member pick's candidate are ActLead. The input flags the
stack answers are gone (inCommand, menu, list, choosing, typing, raw,
menuFloor, viewing, toItems, campBar, camping, ordering, chosen, paging:
`exploring()`, `choosing()`, `typing()`, `up(modeCamp)`, Pick.Line,
eclWait.members, pushSheetOn); `modal`/`floor` stay (help, the journal, the
full map and the overview are not screens of their own). `ext.relist` is
gone; `ext.redraw` stays for the engine-drawn pages; actRedraw means only
"a screen over you is gone", which a question answers by showing itself
again. Changed: in the journal Left/PgUp scroll a tall picture up (they
paged on before); the wheel no longer reaches the engine; Up/Down on menus
that took the keypad move the presenter's highlight.

Page drawing (2026-11-01): the engine stops drawing pages; it publishes
typed state and the presenter draws it, in five steps (1 the text pane
and messages, 2 pictures, 3 help/journal/full map/overview as screens,
4 the data-heavy pages, 5 delete the page machinery: View.Text, Page,
Spots, layers). Step 1 done: `View.Pane` (nil unless the main frame is
up) holds the text pane under the 3D view as lines of `PaneRun` (column,
text, colours, Upcase), whatever DisplayTextBlock, CharMessage,
HitMessage and debugValues put there; the remaster draws its box from it
(paneImage, glyphs by `Engine.FontGlyph`) and the toast straight from its
Note. View.MainFrame and TextShown are gone (`Pane != nil`,
`Pane.Shown()`). StatusLine's row is not in the pane (the remaster never
showed it). The picture box still crops the page (step 2).
Step 2 done: `View.Page []Sprite` is `View.Pictures []Picture`, typed:
each a frame of a DAX file and block (File, Block, Frame: the picture, an
animation's current frame, a symbol, a tile; dax.Image keeps File, Block
and Base from its loader), decoded in the palette, at its page box (At);
or a fill (Fill, Colour). One list keeps the drawing order, so title,
credits, encounter pictures, the finale and the boxes' symbols draw as
before. The remaster draws the 3D view's picture window (WinPicture) from
the pictures (pictureBox) instead of cropping the page; WinText still
crops it (its text is step 3/4). Captures against HEAD (title, both title
pictures, credits, load and walk, the parlay picture) match frame for
frame except the main menu's wall-clock fade-in, which differs between
two runs of the old binary too. No finale dev scene exists to capture.
Step 3 done: help, the journal (and Rolf's history), the full map and
the party overview are screens on the stack (modeHelp, modeJournal,
modeMap, modeOverview) that draw nothing on the page; they publish
`View.Pages` (the box's title, the page's lines as `PaneRun`s from row
Top, the journal's pictures as `Scan`s: file, image, part, place and
paper colour, the hint, page and page count), `View.FullMap` (the map's
name, its squares as drawMap draws them at Cell pixels, the cursor, the
legend's lines with letter, colour and first square, the info line, the
label being typed) and `View.Overview` (the rows are View.Party's). The
remaster draws them over the page under (drawModal: box, glyphs, fills,
the scans over the window as before) and takes the full map's clicks
itself (ActPoint, a second click Label). `ext.modal`, `ext.floor`,
`modalOn`, `keyThen`/`askKey` and View.MapCursor are gone: `modal()`
asks the stack (the F9 slots count too); under a modal screen no spots
are published or taken, and the pane, the portraits and the drops are
not published. View.Overlays stays for the sheets' and pickers'
portraits (step 4). The overview was a pause scene and is now a screen
(any key closes it).
Captures against HEAD (help both pages and back, the journal dev scene,
POR_DEV=talljournal scrolled, the full map with cursor moves and a
label, the overview, walking) match frame for frame except the main
menu's wall-clock fade-in and, on the full map before its first key, the
3D view's status line ("4,0 S 14:23") the old engine wrote into the map
page: the map no longer shares the page, so it is gone.
Step 4 begun: the presenter no longer reads the page's text cells.
`View.Text []Cell` (glyph bitmaps) is `View.Lines [][]PaneRun`, the
page's rows as runs (column, text, colours, Upcase: the pane's way, one
helper `runs` for both) and `View.Cells` the page's size; the remaster
draws them in the current font (Engine.FontGlyph). This is the
publication only: the remaining pages (the sheets, after-combat, the
icon editor, the party list, rest, casting, money, pickers, slots, the
script ops, WinText, the area map) are still laid out by the engine in
its cells, View.Overlays still carries the PNG portraits and icons, and
View.Spots the clicks; each still has to publish its own typed data
before step 5 deletes the cells.
Captures against HEAD before captures were stopped (walk, the sheet with
effects and a PNG portrait, the sheet's spells, the inventory, loot
with POR_SEED, cast, the runes' WinText, parlay, the area map key, rest,
autocombat, the title menu, the creation sheet with its icons) matched
frame for frame but for the main menu's wall-clock fade-in and, without
POR_SEED, the clock-seeded dice. Further checks are headless only.
Step 4 part 2 (headless checks only): the presenter lays the data pages
out itself on a page of text cells (present/remaster/pages.go
`cellPage`: DisplayString's, ClearCells', DrawBorder's and
DrawBoxTitled's ways, the border from View.Border, the colours from
`View.Ink`, the page's 16 text colours in its palette now). Each screen
was recorded first (pages_test.go, testdata/*.txt: the text runs, the
pictures, the PNGs over the page) from the engine's own drawing, and the
presenter's layout of the new data must give the same.
- The character sheet: `View.SheetPage` (name and its colour, NPC, the
  particulars, abilities, money, level, experience and next, AC, HP,
  THAC0, damage, party items, movement, weapon, armour, status,
  effects, the portrait's box and PNG, the spell lists' title over the
  name row). CharacterSheet, showCombatStats, showStat, sheetEffects,
  showMoney and SheetPortrait no longer write text or borders (the DAX
  portrait stays among the pictures); the PNG portrait left
  View.Overlays. The sheet ignores page text under it: the 3D view's
  status line, written over "Exp" on the first sheet after a start, is
  gone.
- The creation sheet: the sheet above (SheetPage.Creating) and its
  combat icon from a PNG (SheetPage.Icons, at View.IconBox); the grey
  ground and the DAX icon stay pictures.
- The grid pickers: View.Picker gains Bottom (the box), Classes and
  Portraits (the saved characters' cards; Labels their names); the
  presenter draws the box, the pager, the names, classes and PNG heads,
  the engine only the DAX pictures. The save slots draw nothing on the
  engine's page any more (no saveScreen): the presenter's box, title and
  "No saving now" from View.Slots.
- The combat icon editor: `View.IconEditor` (the poses as pictures over
  their boxes, Size, the words', the parts' and the palette's cells, the
  palette's colour numbers, the swatch selected); the presenter draws the
  frame, the poses and the words and maps the clicks to ActPicker,
  ActIconSize and ActIconColour (iconClick) where the engine's regions
  (View.Spots) took them. The swatches stay engine fills. Its frame is
  View.Border's EGA cells, no longer the symbols in the combat palette.
  View.Overlays had no users left and is gone (and engine.Overlay, fit).

# UI audit: how interactions are done today, and one standard

Written 2026-11-01. It covers the screens drawn by present/remaster, present/battle and cmd/por, and blobber's Esc menu. File references are relative to the repo.

## 1. Kinds of interaction

Every screen the presenter draws does one of these nine jobs:

| Kind | Screens doing it today |
|---|---|
| **Menu** (pick an action) | main menu, Party submenu, command menu (Enter while exploring), camp/temple/shop/training main screens, sheet "View:", creation menu and Change, icon editor bar, slot submenu, script AskMenu, page questions, Esc options |
| **Context menu** (actions on one thing) | card context menu (powers), spell pick opened from it |
| **Item list** | inventory, shop Browse, loot, shop Identify, temple cures, appraise |
| **Other list** | memorize, scribe, training spells, bestiary, save slots, cast/spell pick, AskList popup, creation drop-downs, battle log |
| **Grid pick** | head, body, icon-part and saved-character pickers |
| **Yes/No** | Quit, Drop (twice), cure anyway / pay, shop deal, training, inventory drop/sell/buy, creation Exit, slot Overwrite/Delete, Flee/Attack Ally, Esc-menu confirms |
| **Text entry** | script AskText/AskNumber, creation name/stats/minimums, new slot name, full-map label |
| **Pager** | help, journal, history |
| **Member pick** | AskMember (cards/roster), F1–F8 everywhere |

## 2. What differs today (by kind)

### Menus

There are seven ways a menu is placed:
- `drawMenu` centred between the cards (3D popups, camp-style screens);
- `keyWindow` at the bottom (page questions, the sheet, creation, the icon editor);
- `keyWindow` centred (slots, the main menu's questions, Esc options);
- top centre at 16k (pick list, battle log);
- anchored to a card (context menu, tooltip);
- anchored under a field (drop-down);
- centred on the page at page scale (a page's AskList).

They use five scales between them: mk, scale(dst), card scale, page scale, and half scale in combat.

**Hover highlight.** These menus highlight under the pointer: main, command, engine popup, context, pick list, Esc options, inventory. These don't: camp/temple/shop/training, AskList, page questions, the sheet, creation, slots and drop-downs. The cause is the hover gate in `remaster.go:710`.

**Selection keys:**
- **Gamepad A (KeyPadSelect)** is ignored by camp-style screens, the engine popup, the command popup, the context menu, the pick list, the inventory and combat.
- **Space** doesn't select on the main menu, the sheet, the engine popup, the context menu or the pick list.

**Hotkeys** come from four places:
- an explicit `Key` field;
- the first letter of the text;
- `blobber.Hotkeys`;
- a fixed map for the battle bar.

`drawMenu` whitens the *first occurrence* of the key letter in the text. So "Camp" with key **E** shows no hotkey letter at all. Rows with key 0 (most list rows and many "Back" lines) have no hotkey.

When a hotkey is pressed:
- most menus act at once;
- the engine popup closes and re-dispatches the key to `keyAct`;
- the command popup closes and passes it to `exploreKey`.

**Esc** means something different on almost every screen:
- camp: pick the last line, which on training's spell list is a *spell*;
- engine popup: close the question;
- screens.go: go back one level;
- inventory: drop the carried stack, then close;
- sheet: falls through to ActClose;
- combat: cancel, else open the options.

**Right click** sends ActClose on camp screens, while Esc on the same screens picks the last line.

### Context menus

- The card menu starts with *no* highlight (`ctxSel = -1`). Enter does nothing until an arrow is pressed.
- It has no gamepad A and no Space.
- It swallows every other key.
- It is drawn after the cards and the Esc menu, so it sits on top. The tooltip uses the same box.

### Item lists

- **Big 20×20 atlas icons** appear only in the inventory, shop Browse, loot and the combat Attack/Ranged buttons.
- **Text only:** shop Identify ("Owner: Name"), temple cures, appraise, and every spell list.

### Other lists

**Scrolling** is handled four different ways:
- the inventory has a scroll bar;
- AskList (12 rows) and screens (14 rows) scroll with no indicator;
- camp lists never scroll; they shrink the whole menu's scale to fit (`camp.go:217`);
- the battle log shows only the last 20 lines and can't scroll.

**PgUp/PgDn** are missing on the camp lists and the inventory. Only the inventory and Esc options take the mouse wheel.

### Yes/No

There are six renderings:
1. a camp-style drawMenu box;
2. a dimmed screen with a centred keyWindow (main menu);
3. a keyWindow at the bottom (creation, pickers, icon editor);
4. a centred keyWindow with no dim (slots);
5. the inventory panel with a "Yes No" row that highlights only on hover;
6. battle-bar buttons with the question on the status line.

The default answer is also inconsistent:
- **No** for the main menu questions, `askThen` and the Esc menu;
- **Yes** for camp Drop and training;
- whatever row was highlighted before, for temple Not/Pay and shop Deal (`sel` isn't reset);
- in the inventory, **Enter = Yes** and *any* other key (Esc, n) = No;
- combat has no default.

### Text entry

There are two editors:
- the engine Ask, which sends every keystroke and is capped at 24 visible characters (a blue bar on the input row);
- screens.go, which sends on Enter, is capped at its own `max`, and has no bar.

The full-map label is a third placement, on the page's prompt row. AskNumber accepts any character, not just digits. Neither editor has a movable cursor.

### Member pick and F1–F8

F1–F8 does something different on each screen:
- command bar: select the card and open its context menu;
- camp and main menu: ActLead;
- sheet: ActMember;
- inventory: ActMember;
- lists, context menu and engine popup: fall through to ActInventory;
- creation and pickers: ignored.

### Gamepad

- The shoulders send `,` and `.`, which nothing handles.
- There is no pad access to Tab (inventory tabs, combat targets), L (battle log), PgUp/PgDn or member switching.
- Blobber's Esc menu polls the pad itself with its own repeat rate (20/4 frames), separate from cmd/por's.

### Combat

The arrows pan the camera, while everywhere else they move a selection. The bar has hover highlight but no keyboard focus. End is on Backspace.

### Code duplication behind this

- **Up/Down wrap logic** is written separately in about 12 places: `remaster.go` 184, 496, 530, 567; `camp.go:229`; `mainmenu.go:270`; `sheet.go:26`; `screens.go:283`; `list.go` 94, 213; `inventory.go:915`; blobber.
- **Hotkey scanning** is written separately in about 11 places, with four different case-folding idioms.
- **Menu models:** `campItem`, `command`, `word`, `barItem`, `menuLine`.
- **Selection state:** `p.sel` is shared by six menus, while seven others keep their own.
- **Box/border drawers:** there are five.
- **Text renderers:** there are five.
- **Rect bookkeeping:** `menuRects` is saved and restored by clone-and-swap in three places.

## 3. The standard

### One widget: `Menu`

Every menu, context menu, list, yes/no and drop-down becomes the same model:

```
Menu{Title, Rows []Row{Text, Key, Icon, Off, Detail}, Sel, Anchor, Kind}
```

It is drawn by one drawer and driven by one key/mouse/pad handler. Each screen supplies only rows and actions.

### Look

- Red ribbon border, black ground, green text, white hotkey letter, and a blue bar on the selection (today's drawMenu look).
- Always drawn **on top of everything**: cards, toasts, the battle bar and the 3D view.
- Questions and context menus dim what's under them.
- The hotkey letter is the one the `Key` says. If that letter isn't in the text, it's shown as `[E] Camp`.
- Every selectable row gets a hotkey: letters from `blobber.Hotkeys`, and digits for list rows.

### Placement

Placement is decided by `Kind`, never by the screen:

| Kind | Where |
|---|---|
| menu or question | centred in the free area, under the picture if one is up |
| context | anchored to the card or thing it's about |
| drop-down | under the field |
| list | centred, fixed rows with a scroll bar |

One scale for all: `Settings.Menu`.

### Input (every kind)

| Action | Key | Mouse | Gamepad |
|---|---|---|---|
| Move | Up/Down (Left/Right for adjustable rows), PgUp/PgDn, Home/End | hover highlights, wheel scrolls | d-pad / stick, shoulders = page |
| Pick | Enter, Space, hotkey | left click | A |
| Back | Esc (closes the menu, or goes back one level; never picks a row) | right click | B |
| Member | F1–F8 = switch member wherever a member applies | click a card | — |

Selection starts on the first enabled row. A question starts on its safe answer: **No** for destructive questions, otherwise the first option. Selection memory is per menu, not shared.

### Item lists

Every list of items uses the inventory's row: a **20×20 atlas icon**, then the name, then the price or count. That applies to shop Browse, loot, Identify, appraise, the temple's items (Remove Curse) and anything that lists items. Spell lists get a spell-school icon if we want one. The bestiary uses the creature icon in the row.

### Lists

- Scroll bar whenever rows overflow, never a shrunk scale.
- PgUp/PgDn and the wheel everywhere.
- The battle log becomes a scrollable list.

### Yes/No

A `Menu` of kind question: Yes/No rows, hotkeys Y/N, the default answer as above, Esc/B = No. That includes the inventory's panel and the battle confirmation; in battle the bar's buttons stay as a shortcut.

### Text entry

One editor:
- the prompt is the title;
- a blinking cursor, with Left/Right/Home/End movement;
- a digit filter for numbers;
- sent on Enter (the engine gets the final text);
- Esc cancels.

### Pagers

Help, the journal and history take Left/Right/PgUp/PgDn, the wheel, a click on the ◀ ▶ marks and the gamepad shoulders.

### Combat

- Arrows move a square cursor (as in aiming).
- Camera pan moves to WASD and the right stick.
- The bar gets keyboard and pad focus (Left/Right across buttons, A presses).
- End moves off Backspace onto E.

### Gamepad

- Shoulders = page / previous–next member.
- X = context menu of the selected thing.
- Y = minimap.
- Start = Esc menu.
- One repeat rate, including blobber's Esc menu.

## 4. Decisions since the audit

- **Placement and size:** each screen may place and size its menus to fit its own layout. Screens that share a layout share one rule, and so do Yes/No questions.
- **Own widgets:** the inventory's item panes and the battle bar stay their own widgets, because they aren't menus. The inventory's bottom action row is a toolbar and keeps its own model too.
- **Combat:** the arrows keep panning the camera and Backspace keeps End, since nothing else uses them in combat. Several mappings for the same thing are fine, as long as each one is obvious.
- **Windows:** one central function draws every bordered window.
- **Scroll bars:** everything that scrolls uses the same scroll bar.

## 5. Status (2026-11-01)

### Done

| Commit | What changed |
|---|---|
| 45c0d34 | **One menu row and key handler** (`menuRow`, `menuNav` in present/remaster/menu.go) for these screens: camp, temple, shop, training, the main menu, the sheet, page screens, the command and engine popups, the spell pick and the context menu.<br>• Up/Down wrap; PgUp/PgDn and Home/End move the highlight.<br>• Enter, Space, pad A and the hotkey (either case) pick.<br>• Esc and right click go back and never pick a row.<br>• A hotkey letter missing from the text shows as `[E] Camp`.<br>• Camp screens highlight under the pointer. |
| fb60548 | **Context menu:** drawn on top of everything with the view dimmed. It opens on its first row. |
| 227e4d3 | **Questions start on the safe answer:** No for Drop. Temple and shop questions reset to Yes instead of keeping the last highlighted row. |
| e6459c8 | **Item icons in menus:** menu rows can carry the 20×20 atlas icon (`drawMenuIcons`); shop Identify uses it. |
| 2c62973 | **Camp lists scroll** instead of shrinking their scale. |
| 45d854a | **One line editor** (`editKey`) for engine and page text entry. Number prompts accept digits only. |
| 8aa1ace | **Gamepad shoulders** send PgUp/PgDn. |
| 4fe37a4 | **One window function:** `window()` draws every frame (fill and ribbon border, returning the inside rectangle): menus, the inventory halves and panel, cards and `drawBox`. `drawBorder` is gone. |
| 16026db | **One scroll bar** (present/remaster/scrollbar.go): the bevelled one, which can be clicked and dragged. Used by the inventory, camp lists, the list popup, page lists, save slots and drop-downs.<br>• The battle log scrolls with the wheel, arrows, PgUp/PgDn, Home/End and dragging. |
| a64418e | **Standard menus:**<br>• The inventory question is a standard menu.<br>• The list popup uses `menuNav`, and Esc closes it.<br>• The list popup highlights the row under the pointer. |
| 7ecdf0a | **One place for Yes/No:** every question is centred over the dimmed view at the Menus scale. |
| 6309d63 | **Battle bar focus:**<br>• Tab/Shift-Tab and the shoulders move a focus; Enter and pad A press the focused button.<br>• Every button shows its key, including Esc on Cancel.<br>• Yes/No show Y and N; Esc means No. |
| 841a0a0 | **Digit hotkeys:** list rows with no letter hotkey get 1–9, then none (`digitKeys`): camp lists, page screens, the sheet and the list popup. |
| ab848fe | **Text cursor:** `editKey` keeps a cursor: Left/Right/Home/End move it, Backspace and Delete edit at it, typing inserts. It blinks (`caretLine`) in the engine and page-screen editors. |
| a737edc | **Pagers:** help, the journal and history page with the wheel and with clicks on the ◀ ▶ marks drawn on the hint row. |
| 0ee0cfc | **Bestiary icons:** bestiary rows show the creature's icon, shrunk to the row with its aspect kept. Spell lists stay text: the repo has no spell-school art. |
| 70c6de3 | **Page boxes:** cellPage and the modal pages share `cellBorder`. They stay text-grid cells in the page's CPU image (uploaded once per View, cropped by the adventure frame), so they don't go through `window()`. |
| 5c3c5b6 | **Esc-menu questions:** Yes, No with Y/N, starting on No; `keyWindow` centres and dims them once. |
| 297dc0c | **One repeat rate:** blobber's `KeyRepeated` uses cmd/por's `repeats` (`blobber.Repeats`). Pad X (Overview) and Start (Enter) keep their bindings. |

Tests: `TestMenuNav`, `TestScrollBar`, `TestBarFocus`, `TestDigitKeys` and `TestEditCursor`. Checks are headless only.

### Open

- Temple cures and appraise list no items, so they stay text only.
- Spell lists have no icons: there is no spell-school art.

## 6. Second pass (2026-11-01)

### The user's decisions

- **Members:** F1–F8 select that member on every screen where a member applies. Left/Right are the previous/next member where they are free; Tab/Shift-Tab only if always free (they are not, see below). The inventory's Home/End no longer change the member. The pad gets a member switch only on free buttons.
- **Portrait clicks:** a right click is the member's sheet everywhere. A left click is the inventory where there is no context menu (combat, camp-style screens). While exploring, a plain left click opens the card's context menu, which lists View and Items. A later correction removed a Shift-click path for it.
- **Selection markers:** exactly two. The *text selection* is a blue ground, for text lines, menus and list rows. The *outline selection* is a green (EGA 10) frame, for anything with pictures. Hover uses the same marker as selection.
- **Toasts:** one style, a red flash on the bottom row, in every view. The inventory's toast must not hide the toolbar.
- **Leaving:** the row that leaves a screen is "Exit", hotkey E, and Esc does it.
- **Prices:** "Ng", plus " Nc" only when copper is needed, from one function. Price lists show the party's gold the same way.

### Done

| Commit | What changed |
|---|---|
| fbfd75c | **Selection markers:** `pal.SelectText` and `pal.SelectOutline` are used for menus, inventory rows, doll boxes, the toolbar, picker cells and battle-bar buttons. The yellow, cyan, dark and gold variants are gone. Rows with nothing to pick (`do` nil, a list's Off lines) are drawn in EGA 8 by drawMenu, drawMenuIcons and keyWindow. |
| dc6eae4 | **One toast:** `noteLine` draws the toast red. The inventory keeps its bottom row for the toast, under the toolbar. |
| 5dda978 | **Prices:** `price(engine.Money)` and `withGold`. `engine.InvItem.Price` is typed `Money`, and `Training.Gold` was added. |
| 32b2ade | **Members:** `memberKey` handles F1–F8, Left/Right on the menu screens and the pad's stick buttons. Camp memorize/scribe counts move to -/+ (Backspace still one less). Home/End are no longer a member key. Cards: left click is the inventory, right click the sheet. |
| 6dbfe5e | **Inventory on the gamepad:** A is Enter, Y is the next tab (as Tab), and the shoulders (PgUp/PgDn) move a toolbar focus. |
| 6cbcb0d | **Menus:**<br>• Hover only follows a moved pointer, and highlights every keyWindow menu.<br>• The wheel is up-for-up, one line a step, on every list.<br>• Page questions use menuNav.<br>• The spell pick has its own selection, reset on open, and digit hotkeys (`spellKeys`) like every spell list.<br>• A click outside a modal menu means back.<br>• `globalKey` passes help, minimap, F9, Ctrl-S and the font keys on.<br>• Exit/E rows replace Leave and Back; the command popup's Camp moves to **A**. |
| 8d7744f | **Correction:** a plain left click on a card opens the context menu again while exploring; there is no Shift-click. |
| 7f4302f | **Blobber's Esc menu:** mirrors menuNav, with PgUp/PgDn, Home/End, digit hotkeys, and Left/Right repeating. It can't import present/remaster without a cycle. |

### Conflicts and limits

**Tab** is never a member key, because Tab is already used:
- the minimap while exploring (pad Y too);
- the inventory's tabs;
- combat targets and the battle bar's focus.

**Left/Right** are not member keys in these places, because they already do something there:
- exploring, where they turn (no clean alternative);
- combat, where they pan the camera;
- text entry, where they move the cursor;
- the full map, where they move its cursor;
- pagers, where they page.

The camp's memorize/scribe count used Left/Right; it moved to -/+.

Member keys are also off under a Yes/No question about the member (camp Drop, temple Pay, the party menu's questions), so the question's subject can't change while it's open.

**Gamepad:** the stick buttons (L3/R3) were the only free buttons, so they switch the member.

**The command popup's Camp** couldn't take C, which Cast uses, so it uses A.

**Clicks outside a menu** don't mean back on these screens, because their own page takes clicks:
- the sheet (Shift-click on the portrait);
- the full map;
- pagers;
- the creation sheet, pickers and the icon editor (their main screens).

**Keys passed on:** the engine popup, the context menu and camp pass on only the global keys. Any other key would act on the screen hidden underneath them.

**Not changed:**
- The full map's yellow cursor is the original's page drawing, so it stays.
- The party menu has no inventory, so a left click on a card there only selects the member.

## 7. Third pass (2026-11-01)

### Decisions

- **Yes/No in the front end.** The engine publishes a typed question, `View.Ask` of kind `AskConfirm` (the prompt only: no rows, no preselection), and takes `ActYes`, or `ActNo`/`ActClose`. `YesNo` (Stop resting?, Lose it?/Use it?, cure anyway?) and combat's `confirm` (Flee?, Attack Ally?) both publish it; `Battle.Ask` is gone. The presenter draws every one with `keyWindow`: centred, dimmed, Y/N, starting on No; Esc, B, a right click and a click outside are No. The battle bar's Yes/No stay as a shortcut and send the same decisions. Abandon and Discard (creation, Modify) were already front-end questions (`askThen`, the engine only publishes the prompt), so they stay. The engine never asks "Abort Spell?" (an extra keeps the spell), so there is nothing to move. Outcomes are unchanged.
- **One binding table** (present/remaster/bindings.go): context, keys, mouse way, pad way, the pad's buttons, and a `gap` naming a missing device. The help is made from it (`HelpLines`, given to the engine with `SetHelp`: the engine only pages it, and its own text is gone), and cmd/por's gamepad reads its buttons (`PadKeys`), so neither can drift. The keyboard dispatch is still the screens' own code; the test checks the table, not each key.
- **Combat, XCOM 2 style** (only in combat with no menu over the map; exploring keeps its pad):

| Action | Keyboard | Mouse | Gamepad |
|---|---|---|---|
| Move the square cursor (free: from the active unit; aiming: the aim) | W A S D | point | D-pad, left stick |
| Confirm: walk or attack at the cursor, fire, press the focused button | Enter, Space | click | A |
| Cancel aiming, else the options | Esc | right click | B |
| Next/previous target (aiming), else the bar's focus | Tab / Shift-Tab, PgUp/PgDn | point | RB / LB |
| Press a bar button | its letter, 1-0 | click | LB/RB then A |
| Turn the camera | Q E | — | LT RT |
| Pan the camera | arrows | — | right stick |
| Recentre on the active unit | Home | — | R3 |
| End the turn | Backspace | click End | Y |
| Guard (the overwatch) | G | click Guard | X |
| Options | Esc | right click | Start |
| Battle log | L | — | Back |
| Zoom | — | wheel | — |

  W A S D no longer pan (the arrows still do); Q and E still turn the camera, and D is no longer "done". The cursor moves as the camera looks. A moved mouse takes over from the keys' cursor.
- **Mouse while adventuring:** a click on the view's left or right third turns, its top third walks on, its bottom third steps back, the middle opens the command menu. The command menu gains Map, Minimap, Overview and Help, so the mouse and the pad reach them. A right click is the options.
- **Back targets:** going back restores the row that opened the submenu (camp/temple/shop `campUI.to`/`back`, the Party submenu, the page screens' `open`, blobber's pages and questions). The temple, shop and training questions and the inventory's start on No.
- **Appraise:** Keep/Sell gets Exit (E). The original's leaving sells, so Exit and Esc sell and go back to the gem list; they no longer leave the shop or temple.
- **Quit:** one row "Quit to DOS" (hotkey D on the main menu: Q is Quick Start's) and one question, "Quit to DOS? Unsaved progress is lost." (`blobber.QuitRow`, `QuitQuestion`).

### Done

| Commit | What changed |
|---|---|
| 32d3a4f, 26f7725 | Yes/No moves to the front end (`AskConfirm`, `ActNo`). Tests: `TestYesNoConfirm` (engine), `TestConfirmKeys`; `TestFightRuns` answers combat's question through `View.Ask`, `TestBarFocus` sends `ActNo` for No. |
| c3a9f32 | The binding table, the help from it, the combat pad and keyboard cursor, adventuring by mouse. The aiming hint names the cursor keys (in the section's own commit). Tests: `TestPadKeys`, `TestBindings`, `TestCursor`. |
| a0fa82b | Blobber's Esc menu pad as cmd/por's menus: B and Back are Esc, Start is Enter, LB/RB PgUp/PgDn, LT/RT Home/End. The host does not feed it keys: blobber still polls, with identical buttons. |
| c223edc | The inventory question's hover follows only a moved pointer; Space picks in the inventory and on the battle bar; the combat toast is `noteLine`'s, and a question no longer replaces it. |
| 3156bd3 | Rows restored on back; Appraise's Exit. |
| 4276087 | Quit; the Esc menu's settings pages end in Exit (E), as Esc. |
| 8707714 | View, Cast, Name, the stats, Save as, Choose spell, Rest time, Stop resting?, Lose it?, Use it?, "... is not diseased. Cure anyway?". |

### Gaps and limits

The ground rule asks for keyboard, mouse and gamepad ways to every action. These are missing, and the table names them (`gap`):
- **Typing** (names, numbers, save names, map labels) has no mouse or pad way: there is no on-screen keyboard.
- **Combat camera:** the mouse cannot turn, pan or recentre it, and only the mouse zooms.
- **The battle log** has no mouse way to open it (L, pad Back).
- **The font** (Ctrl-F) has no mouse or pad way.
- **Help** is reached by mouse and pad only from the command menu while adventuring.

Other limits:
- **Training's new spell:** the list has no Exit. The engine requires a pick there (as the original), so Esc does nothing rather than leave.
- **Start** opens the options in combat but, as Enter, picks inside them; it does not close them as XCOM's does.
- **XCOM deviations:** Back is the battle log, not an overview. L3 has no combat use (it stays the previous member). The cursor steps a square a press, as the keys do, not analog. Guard stands in for overwatch.
- **The full map's label prompt** ("Label: ") stays inline on the page's prompt row, with its colon.

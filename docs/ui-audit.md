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

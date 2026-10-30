package engine

import (
	"os"
	"slices"
)

// DevScene returns a routine that sets up and runs one screen for a
// headless check (snap ... game SLOT KEYS SCENE), or nil.
func (e *Engine) DevScene(name string) func() {
	switch name {
	case "shop":
		return func() {
			e.run.treasure = nil
			for _, t := range []itemType{itemGauntlets, itemJavelin, itemArrow, itemMageScroll, itemLeatherArmor, 0x05} {
				e.run.treasure = append(e.run.treasure, e.RandomItem(t))
			}
			if len(e.party) > 2 { // unknown items for Id
				e.party[0].Items = append(e.party[0].Items, e.RandomItem(0x15))
				e.party[2].Items = append(e.party[2].Items, e.RandomItem(0x32), e.RandomItem(0x05))
			}
			e.Shop()
			e.beginFlag = 4 // as opCombat after it: the 3D view comes back
			e.DrawMainScreen()
		}
	case "temple":
		return func() {
			c := e.selected()
			e.purse.gold = 10000
			c.HP = 1
			e.Temple()
			e.beginFlag = 4 // as opCombat after it: the 3D view comes back
			e.DrawMainScreen()
		}
	case "appraise":
		return func() {
			e.purse.gems, e.purse.jewelry = 3, 2
			e.Shop()
			e.beginFlag = 4 // as opCombat after it: the 3D view comes back
			e.DrawMainScreen()
		}
	case "combat", "wildcombat", "castfight", "missile", "autocombat": // castfight: every member has Cure Light Wounds memorized (the inventory's Spells in a fight); missile: the Ranged button for everyone (a sling, a bow and arrows, javelins in the Missiles slot); autocombat: the computer fights for the party against eight (measuring the battle view)
		return func() {
			if name == "missile" {
				launchers := []itemType{itemSling, itemShortBow} // the first two who may: a sling, a bow with arrows (hands freed)
				for i, c := range e.party {
					e.selectMember(i)
					e.ApplyItems(c)
					if c.Slots[slotWeapon] == nil || c.Status != statusOkay {
						continue
					}
					if len(launchers) > 0 && e.base(launchers[0]).Classes&c.ClassMask != 0 {
						for _, s := range []int{slotWeapon, slotShield} {
							if c.Slots[s] != nil {
								e.readyItem(c.Slots[s])
								e.ApplyItems(c)
							}
						}
						e.readyItem(e.AddItemCopy(c, e.RandomItem(launchers[0])))
						e.ApplyItems(c)
						if launchers[0] == itemShortBow {
							e.readyItem(e.AddItemCopy(c, e.RandomItem(itemArrow)))
						}
						launchers = launchers[1:]
					}
					if c.Slots[slotWeapon] == nil || e.base(c.Slots[slotWeapon].Type).Usage&8 == 0 { // else a javelin beside the weapon
						e.readyItem(e.AddItemCopy(c, e.RandomItem(itemJavelin)))
					}
					e.ApplyItems(c)
				}
				e.selectMember(0)
			}
			if name == "wildcombat" { // the overland's terrain, from the dungeon save
				e.viewMode = 2
			}
			if name == "castfight" {
				for _, c := range e.party {
					c.Memorized[0] = spCureLightWounds
				}
				e.selected().Portrait = readPNG(os.Getenv("POR_PNG")) // the sheet's portrait, when POR_PNG is set
			}
			// Block 1 exists only in MON2CHA; take the first monster of
			// the current set (DS:5376) so any save can start a fight.
			idx := 1
			if f := e.L.File(e.set.file("MON") + "CHA"); f != nil && len(f.Entries) > 0 {
				idx = f.Entries[0].ID
			}
			count := 3
			if name == "autocombat" {
				count = 8
				for _, c := range e.party {
					c.Auto = 1 // as Quick for everyone
				}
			}
			e.addMonsters(idx, count, 1)
			// as opSetupMonster with max distance 1: the monsters stand off
			// a block (a fresh game's distance is 0, right among the party)
			e.game.MonsterDist = min(uint16(e.FreeSteps(e.posX, e.posY, e.facing)), 1)
			e.Combat()
			e.AfterCombat()
			e.DrawMainScreen()
		}
	case "parlay": // an encounter's menu, then the talk-style menu a script's Parlay shows after PARLAY
		return func() { // as opEncounterMenu at distance 0 with a close-up, then the script's Parlay
			sprite, pic := 1, 1
			for _, f := range []struct {
				name string
				id   *int
			}{{"SPRIT", &sprite}, {"PIC", &pic}} {
				if df := e.L.File(e.set.file(f.name)); df != nil && len(df.Entries) > 0 {
					*f.id = df.Entries[0].ID
				}
			}
			e.es.inEncounterMenu, e.game.Portrait = 1, 0xFF
			e.ShowEncounter(sprite, pic, 0, &e.es.encounterShown)
			e.msgCol, e.msgRow = 1, 0x11
			e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, true, "A band of kobolds blocks the way.")
			for e.MenuInput(false, colWhite, colLightGreen, colLightMagenta, "~COMBAT ~WAIT ~FLEE ~PARLAY", "") == 1 { // WAIT: a message, then the menu again
				e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, true, "Both sides wait. The kobolds eye the party with suspicion.")
			}
			e.ClearBox(0, promptRow, lastCol, promptRow)
			e.es.inEncounterMenu = 0
			e.MenuInput(false, colWhite, colLightGreen, colLightMagenta, "~HAUGHTY ~SLY ~NICE ~MEEK ~ABUSIVE", " ")
			e.DrawMainScreen()
		}
	case "cure": // the paladin's Cure: the member pick, then the 'not diseased, cure anyway' question
		return func() {
			e.paladinCure(e.selected())
			e.DrawMainScreen()
		}
	case "portrait": // the PNG at POR_PNG as the selected member's portrait: the sheet, then the view with the cards
		return func() {
			e.selected().Portrait = readPNG(os.Getenv("POR_PNG"))
			e.ViewCharacter()
			e.run.treasure = []*Item{e.RandomItem(itemGauntlets)} // then the loot screen
			e.lootTake()
			e.DrawMainScreen()
		}
	case "sheetcast": // the sheet's Spells with the PNG at POR_PNG as the portrait: the list is a page of its own (no portrait overlay, no tall row)
		return func() {
			e.selectMember(slices.IndexFunc(e.party, func(c *Character) bool { return c.Status == statusOkay }))
			c := e.selected()
			c.Portrait = readPNG(os.Getenv("POR_PNG"))
			c.Memorized[0] = spCureLightWounds
			e.ViewCharacter()
			e.DrawMainScreen()
		}
	case "cast", "castko": // Cast on the command bar with Cure Light Wounds memorized; castko with the selected member unconscious
		return func() {
			c := e.selected()
			if name == "cast" { // an able caster
				for i, m := range e.party {
					if m.Status == statusOkay {
						e.selectMember(i)
						c = m
						break
					}
				}
			} else {
				c.Status, c.Okay = statusUnconscious, 0
			}
			for _, m := range e.party { // every able member: the right column's context menus too
				if m.Status == statusOkay {
					m.Memorized[0] = spCureLightWounds
				}
			}
			c.Memorized[0] = spCureLightWounds
			e.command(cmdCast, true)
			e.DrawMainScreen()
		}
	case "who": // a member pick on the view, as a script's "Who will search?"
		return func() { e.SelectCharacter("Who will search?", false, nil) }
	case "effects":
		return func() { // spell effects for the View page: many on member 1, one on member 2
			for _, t := range []affectType{affBless, affCurse, affMonsterInvisible, affPoisoned, affRegenerating, affFireResistance} {
				e.addAffectNow(e.party[0], t, 0, 100)
			}
			e.addAffectNow(e.party[1], affMonsterInvisible, 0, 100)
			for i, t := range []itemType{itemGauntlets, itemJavelin, itemArrow} { // and items for the inventory
				e.AddItemCopy(e.party[i%2], e.RandomItem(t))
			}
			e.ApplyItems(e.party[0])
			e.ApplyItems(e.party[1])
		}
	case "journal": // a text window naming journal entries, as ECL3 block 0 prints them
		return func() {
			e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, true, "YOU OVERHEAR TAVERN TALE ")
			e.journalPrinted("YOU OVERHEAR TAVERN TALE ", true)
			e.DisplayTextBlock(1, msgTop, winRight, winBottom, colLightGreen, false, "3")
			e.journalPrinted("3", false)
			e.journalPrinted("READ JOURNAL ENTRIES 26 AND 4.", true)
		}
	case "runes": // the Sokal Keep elf's scroll (ECL1 block 21, 9BD7): opClearBox, three opProtection lines
		return func() {
			e.snap.win = WinText // as opClearBox
			e.ClearBox(1, 1, 15, 15)
			e.run.textInBox = 1
			e.msgCol, e.msgRow = 2, 5
			for _, l := range [][]byte{{0x56, 0x57, 0x6b, 0x6c, 0x71, 0x72}, {0x67, 0x68, 0x40, 0x41, 0x58, 0x59, 0x5c, 0x5d, 0x67, 0x68, 0x6b, 0x6c, 0x46, 0x47}, {0x67, 0x68, 0x4e, 0x4f, 0x48, 0x49, 0x67, 0x68, 0x69, 0x6a, 0x5a, 0x5b, 0x50, 0x51}} {
				for i, c := range l {
					e.DisplayChar(e.msgCol+i, e.msgRow, colWhite, c+1, false)
				}
				e.msgRow++
			}
			e.anyKey()
		}
	case "inventory": // member 1 fully equipped, a party inventory to scroll
		return func() {
			c := e.party[0]
			for _, it := range c.Items { // the save's own gear joins the party inventory
				it.Readied = 0
			}
			for _, t := range []itemType{itemLongSword, itemShield, itemPlateMail, itemGauntlets, 0x40, 0x41, 0x42, itemCloak, 0x44, itemRing, itemProtectionRing, itemArrow, itemBracers} {
				it := e.RandomItem(t)
				it.Readied = 1
				c.Items = append(c.Items, it)
			}
			for _, t := range []itemType{itemDagger, itemShortBow, itemPotion, itemMageScroll, itemWand, itemValuable, itemHolyWater, itemOil, itemJavelin, itemQuarrel, itemChainMail, itemDart, itemHammer, 0x48, itemClericScroll, itemStrengthPotion, 0x4B} {
				e.AddItemCopy(c, e.RandomItem(t))
			}
			e.ApplyItems(c)
		}
	case "lockeddoor": // a locked door ahead that no one can pick (the Pick menu's failure)
		return func() {
			for d := 0; d < 8; d += 2 { // face a wall
				if e.WallAt(e.posX, e.posY, d) != 0 && !e.offMapSolid(e.posX+dirDX[d], e.posY+dirDY[d]) {
					e.facing = d
					break
				}
			}
			e.setDoor(e.posX, e.posY, e.facing, 2)
			e.setDoor(e.posX+dirDX[e.facing], e.posY+dirDY[e.facing], (e.facing+4)%8, 2)
			e.run.doorTry = [3]byte{1, 1, 1}
			for _, c := range e.party {
				c.Thief[1] = 0
			}
			e.Redraw()
		}
	case "loot":
		return func() {
			e.run.treasure = nil
			for _, t := range []itemType{itemGauntlets, itemJavelin, itemArrow} {
				e.run.treasure = append(e.run.treasure, e.RandomItem(t))
			}
			e.loot = purse{gold: 156, gems: 7}
			e.AfterCombat()
			e.DrawMainScreen()
		}
	}
	return nil
}

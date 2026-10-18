package engine

// DevView is a named screen the snap tool can render (development aid).
type DevView struct {
	Name string
	Draw func()
}

// DevViews returns the views of the loaded game worth checking: the 3D
// view in all four directions.
func (e *Engine) DevViews() []DevView {
	var v []DevView
	for _, d := range []int{0, 2, 4, 6} {
		d := d
		v = append(v, DevView{"nesw"[d/2 : d/2+1], func() {
			e.ClearCells(0, 0, 39, 24)
			e.Draw3DView(e.posX, e.posY, d)
		}})
	}
	return v
}

// DevScene returns a routine that sets up and runs one screen for a
// headless check (snap ... game SLOT KEYS SCENE), or nil.
func (e *Engine) DevScene(name string) func() {
	switch name {
	case "shop":
		return func() {
			e.run.treasure = nil
			for _, t := range []byte{0x3F, 0x15, 0x49, 0x3D, 0x32, 0x05} {
				e.run.treasure = append(e.run.treasure, e.RandomItem(t))
			}
			if len(e.party) > 2 { // unknown items for Id
				e.party[0].Items = append(e.party[0].Items, e.RandomItem(0x15))
				e.party[2].Items = append(e.party[2].Items, e.RandomItem(0x32), e.RandomItem(0x05))
			}
			e.Shop()
			e.DrawMainScreen()
		}
	case "temple":
		return func() {
			c := e.selected()
			e.purse.gold = 10000
			c.HP = 1
			e.Temple()
			e.DrawMainScreen()
		}
	case "appraise":
		return func() {
			e.purse.gems, e.purse.jewelry = 3, 2
			e.Shop()
			e.DrawMainScreen()
		}
	case "combat", "wildcombat":
		return func() {
			if name == "wildcombat" { // the overland's terrain, from the dungeon save
				e.viewMode = 2
			}
			// Block 1 exists only in MON2CHA; take the first monster of
			// the current set (DS:5376) so any save can start a fight.
			idx := 1
			if f := e.L.File(e.set.file("MON") + "CHA"); f != nil && len(f.Entries) > 0 {
				idx = f.Entries[0].ID
			}
			e.addMonsters(idx, 3, 1)
			e.Combat()
			e.AfterCombat()
			e.DrawMainScreen()
		}
	case "effects":
		return func() { // spell effects for the View page: many on member 1, one on member 2
			for _, t := range []byte{1, 2, 0x47, 0x37, 0x3B, 0x3D} {
				e.addAffectNow(e.party[0], t, 0, 100)
			}
			e.addAffectNow(e.party[1], 0x47, 0, 100)
			for i, t := range []byte{0x3F, 0x15, 0x49} { // and items for the inventory
				e.AddItemCopy(e.party[i%2], e.RandomItem(t))
			}
			e.ApplyItems(e.party[0])
			e.ApplyItems(e.party[1])
		}
	case "journal": // a text window naming journal entries, as ECL3 block 0 prints them
		return func() {
			e.DisplayTextBlock(1, 0x11, 0x26, 0x16, 0xA, true, "YOU OVERHEAR TAVERN TALE ")
			e.journalPrinted("YOU OVERHEAR TAVERN TALE ", true)
			e.DisplayTextBlock(1, 0x11, 0x26, 0x16, 0xA, false, "3")
			e.journalPrinted("3", false)
			e.journalPrinted("READ JOURNAL ENTRIES 26 AND 4.", true)
		}
	case "loot":
		return func() {
			e.run.treasure = nil
			for _, t := range []byte{0x3F, 0x15, 0x49} {
				e.run.treasure = append(e.run.treasure, e.RandomItem(t))
			}
			e.loot = purse{gold: 156, gems: 7}
			e.AfterCombat()
			e.DrawMainScreen()
		}
	}
	return nil
}

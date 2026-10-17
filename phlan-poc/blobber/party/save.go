package party

// Prepare writes who wears each item into the items, so the party can be JSON-encoded without pointer cycles.
func (p *Party) Prepare() {
	for _, st := range p.Inventory {
		for _, it := range st {
			it.Worn, it.WornAt = 0, ""
			for i, c := range p.Chars {
				for s, x := range c.Equipped {
					if x == it {
						it.Worn, it.WornAt = i+1, s
					}
				}
			}
		}
	}
}

// Relink rebuilds Wearer and Equipped from the saved Worn fields after a JSON decode.
func (p *Party) Relink() {
	for _, c := range p.Chars {
		c.Equipped = map[Slot]*Item{}
	}
	for _, st := range p.Inventory {
		for _, it := range st {
			if it.Worn > 0 && it.Worn <= len(p.Chars) {
				it.Wearer = p.Chars[it.Worn-1]
				it.Wearer.Equipped[it.WornAt] = it
			}
		}
	}
}

package remaster

import (
	"fmt"

	"por/engine"
)

// The shop (View.Shop): Browse is the inventory with the goods; the
// presenter asks before an Id.
func (p *Presenter) shopItems(v engine.View) (string, []campItem) {
	c, s := &p.camp, v.Shop
	to := func(n int) func() { return func() { c.screen, c.sel = n, 0 } }
	switch c.screen {
	case shopID:
		if len(s.Unknown) == 0 {
			break
		}
		var it []campItem
		for i, u := range s.Unknown {
			it = append(it, campItem{u.Owner + ": " + u.Name, 0, func() { c.pick, c.screen = i, shopDeal }})
		}
		return fmt.Sprintf("Identify for %dg (%d gold)", s.IDPrice, s.Gold), append(it, campItem{"Back", 'B', to(campMain)})
	case shopDeal:
		if c.pick >= len(s.Unknown) {
			c.screen = shopID
			return p.shopItems(v)
		}
		u := s.Unknown[c.pick]
		return fmt.Sprintf("Identify %s for %d gold. Is it a deal?", u.Name, s.IDPrice), []campItem{
			{"Yes", 'Y', func() { p.sendX(engine.ActIdentify, u.Member, u.Item); c.screen = shopID }},
			{"No", 'N', to(shopID)}}
	case appraise:
		return p.appraiseItems(s.Appraise, to(campMain))
	}
	it := []campItem{{"Browse", 'B', func() { p.send(engine.ActInventory, -1) }}, {"Appraise", 'A', to(appraise)}}
	if len(s.Unknown) > 0 {
		it = append(it, campItem{"Identify", 'I', to(shopID)})
	}
	return "Shop", append(it, campItem{"Exit", 'E', func() { p.send(engine.ActClose, 0) }})
}

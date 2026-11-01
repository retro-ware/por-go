package remaster

import (
	"fmt"

	"por/engine"
)

// The shop (View.Shop): Browse is the inventory with the goods; the
// presenter asks before an Id.
func (p *Presenter) shopItems(v engine.View) (string, []menuRow) {
	c, s := &p.camp, v.Shop
	switch c.screen {
	case shopID:
		if len(s.Unknown) == 0 {
			break
		}
		var it []menuRow
		for i, u := range s.Unknown {
			it = append(it, menuRow{u.Owner + ": " + u.Name, 0, func() { c.pick = i; c.to(shopDeal, 1)() }})
		}
		return withGold("Identify for "+gold(s.IDPrice), s.Gold), append(it, menuRow{"Exit", 'E', c.back(campMain)})
	case shopDeal:
		if c.pick >= len(s.Unknown) {
			c.screen = shopID
			return p.shopItems(v)
		}
		u := s.Unknown[c.pick]
		return withGold(fmt.Sprintf("Identify %s for %s. Is it a deal?", u.Name, gold(s.IDPrice)), s.Gold), []menuRow{
			{"Yes", 'Y', func() { p.sendX(engine.ActIdentify, u.Member, u.Item); c.back(shopID)() }},
			{"No", 'N', c.back(shopID)}}
	case appraise:
		return p.appraiseItems(s.Appraise, c.back(campMain))
	}
	it := []menuRow{{"Browse", 'B', func() { p.send(engine.ActInventory, -1) }}}
	if appraisable(s.Appraise) {
		it = append(it, menuRow{"Appraise", 'A', c.to(appraise, 0)})
	}
	if len(s.Unknown) > 0 {
		it = append(it, menuRow{"Identify", 'I', c.to(shopID, 0)})
	}
	return "Shop", append(it, menuRow{"Exit", 'E', func() { p.send(engine.ActClose, 0) }})
}

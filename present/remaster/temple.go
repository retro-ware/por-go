package remaster

import (
	"fmt"

	"por/engine"
)

// The temple (View.Temple): the presenter asks; ActTempleCure comes agreed.
func (p *Presenter) templeItems(v engine.View) (string, []campItem) {
	c, t := &p.camp, v.Temple
	to := func(s int) func() { return func() { c.screen, c.sel = s, 0 } }
	name := selectedName(v)
	cure := engine.Cure{}
	if c.pick >= 0 && c.pick < len(t.Cures) {
		cure = t.Cures[c.pick]
	}
	switch c.screen {
	case templeHeal:
		var it []campItem
		for i, cu := range t.Cures {
			it = append(it, campItem{fmt.Sprintf("%-20s %5d", cu.Name, cu.Price), 0, func() {
				c.pick = i
				if cu.Needed {
					c.screen = templePay
				} else {
					c.screen = templeNot
				}
			}})
		}
		return name + ", how can we help you?", append(it, campItem{"Back", 'B', to(campMain)})
	case templeNot:
		return name + " " + cure.Not + " Cast cure anyway?", []campItem{{"Yes", 'Y', func() { c.screen = templePay }}, {"No", 'N', to(templeHeal)}}
	case templePay:
		return fmt.Sprintf("%s will only cost %d gold pieces. Pay for cure? (%d gold)", cure.Name, cure.Price, t.Gold), []campItem{
			{"Yes", 'Y', func() { p.sendX(engine.ActTempleCure, -1, c.pick); c.screen = templeHeal }},
			{"No", 'N', to(templeHeal)}}
	case appraise:
		return p.appraiseItems(t.Appraise, to(campMain))
	}
	return "Temple", []campItem{{"Heal", 'H', to(templeHeal)}, {"View", 'V', func() { p.send(engine.ActMember, -1) }},
		{"Appraise", 'A', to(appraise)}, {"Exit", 'E', func() { p.send(engine.ActClose, 0) }}}
}

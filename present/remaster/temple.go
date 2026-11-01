package remaster

import (
	"fmt"

	"por/engine"
)

// The temple (View.Temple): the presenter asks; ActTempleCure comes agreed.
func (p *Presenter) templeItems(v engine.View) (string, []menuRow) {
	c, t := &p.camp, v.Temple
	to := func(s int) func() { return func() { c.screen, c.sel = s, 0 } }
	name := selectedName(v)
	cure := engine.Cure{}
	if c.pick >= 0 && c.pick < len(t.Cures) {
		cure = t.Cures[c.pick]
	}
	switch c.screen {
	case templeHeal:
		var it []menuRow
		for i, cu := range t.Cures {
			it = append(it, menuRow{fmt.Sprintf("%-20s %6s", cu.Name, price(engine.Money{Gold: cu.Price})), 0, func() {
				c.pick = i
				if cu.Needed {
					c.screen = templePay
				} else {
					c.screen = templeNot
				}
				c.sel = 0
			}})
		}
		return withGold(name+", how can we help you?", t.Gold), append(it, menuRow{"Exit", 'E', to(campMain)})
	case templeNot:
		return name + " " + cure.Not + " Cast cure anyway?", []menuRow{{"Yes", 'Y', func() { c.screen, c.sel = templePay, 0 }}, {"No", 'N', to(templeHeal)}}
	case templePay:
		return withGold(fmt.Sprintf("%s will only cost %s. Pay for cure?", cure.Name, price(engine.Money{Gold: cure.Price})), t.Gold), []menuRow{
			{"Yes", 'Y', func() { p.sendX(engine.ActTempleCure, -1, c.pick); c.screen = templeHeal }},
			{"No", 'N', to(templeHeal)}}
	case appraise:
		return p.appraiseItems(t.Appraise, to(campMain))
	}
	return "Temple", []menuRow{{"Heal", 'H', to(templeHeal)}, {"View", 'V', func() { p.send(engine.ActMember, -1) }},
		{"Appraise", 'A', to(appraise)}, {"Exit", 'E', func() { p.send(engine.ActClose, 0) }}}
}

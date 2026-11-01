package remaster

import (
	"fmt"

	"por/engine"
)

// The temple (View.Temple): the presenter asks; ActTempleCure comes agreed.
func (p *Presenter) templeItems(v engine.View) (string, []menuRow) {
	c, t := &p.camp, v.Temple
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
					c.to(templePay, 1)()
				} else {
					c.to(templeNot, 1)()
				}
			}})
		}
		return withGold(name+", how can we help you?", t.Gold), append(it, menuRow{"Exit", 'E', c.back(campMain)})
	case templeNot:
		return name + " " + cure.Not + " Cast cure anyway?", []menuRow{{"Yes", 'Y', c.to(templePay, 1)}, {"No", 'N', c.back(templeHeal)}}
	case templePay:
		return withGold(fmt.Sprintf("%s will only cost %s. Pay for cure?", cure.Name, price(engine.Money{Gold: cure.Price})), t.Gold), []menuRow{
			{"Yes", 'Y', func() { p.sendX(engine.ActTempleCure, -1, c.pick); c.back(templeHeal)() }},
			{"No", 'N', c.back(templeHeal)}}
	case appraise:
		return p.appraiseItems(t.Appraise, c.back(campMain))
	}
	it := []menuRow{{"Heal", 'H', c.to(templeHeal, 0)}, {"View", 'V', func() { p.send(engine.ActMember, -1) }}}
	if appraisable(t.Appraise) {
		it = append(it, menuRow{"Appraise", 'A', c.to(appraise, 0)})
	}
	return "Temple", append(it, menuRow{"Exit", 'E', func() { p.send(engine.ActClose, 0) }})
}

// appraisable: the party has gems or jewelry to appraise (or one is being
// valued): else Appraise is not offered.
func appraisable(a engine.Appraise) bool { return a.Gems+a.Jewelry > 0 || a.Valued > 0 }

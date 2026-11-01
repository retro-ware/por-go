package remaster

import (
	"fmt"
	"strings"

	"por/engine"
)

// Training (View.Training): the hall's offer, the question, the new spell.
func (p *Presenter) trainItems(v engine.View) (string, []menuRow) {
	t := v.Training
	leave := menuRow{"Exit", 'E', func() { p.send(engine.ActClose, 0) }}
	switch {
	case len(t.Learn) > 0:
		var it []menuRow
		for _, s := range t.Learn {
			it = append(it, menuRow{s.Name, 0, func() { p.sendX(engine.ActLearn, -1, s.ID) }})
		}
		return t.Name + " may add a spell to the book", it
	case t.Why != "":
		return t.Why, []menuRow{leave}
	}
	cost := inkGold
	if t.Gold < t.Price {
		cost = inkRed
	}
	g := func(c rune, s string) string { return string(c) + s + string(c) }
	return fmt.Sprintf("%s will become\n%s.\n\nTraining costs %s.\nThe party has %s.\n\nDo you wish to train?",
			t.Name, g(inkGold, strings.Join(t.Becomes, " and ")), g(cost, price(engine.Money{Gold: t.Price})), g(inkGold, price(engine.Money{Gold: t.Gold}))),
		[]menuRow{{"Yes", 'Y', func() { p.send(engine.ActTrain, -1) }}, {"No", 'N', leave.do}}
}

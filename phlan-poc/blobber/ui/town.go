package ui

import (
	"bufio"
	"fmt"
	"image"
	"io"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"blobber"
	"blobber/font"
	"blobber/party"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Offer is one line in a shop. Do runs it after the cost has been paid and returns a message.
type Offer struct {
	Icon, Label string
	Cost        int
	Do          func() string
}

// Shop is the window behind the `town` opcode: a list of offers, recomputed every frame
// so sell lists and prices follow the party.
type Shop struct {
	P        *party.Party
	Title    string
	Offers   func() []Offer
	Sound    func(string) // optional, called with "coins" on a purchase
	Char     int          // character being served (F1-F6 or a portrait click); guild and trainer offer for them
	sel      int
	msg      string
	hoverBtn string
	canvas
}

// NewTown opens the named building: smith, guild, temple, tavern, trainer or bank.
// Unknown kinds open a shop with nothing on offer.
func NewTown(kind string, p *party.Party) *Shop {
	s := &Shop{P: p, Title: kind, Offers: func() []Offer { return nil }}
	s.msg = "Welcome."
	switch kind {
	case "smith":
		s.Title = "Blacksmith"
		s.Offers = func() []Offer { return append(buy(p, party.Weapon, party.Armor, party.Accessory), sell(p)...) }
	case "guild":
		s.Title = "Mage Guild"
		s.Offers = func() []Offer {
			return append(append(spells(p.Chars[s.Char]), buy(p, party.Consumable, party.Scroll)...), sell(p)...)
		}
	case "temple":
		s.Title = "Temple"
		s.Offers = func() []Offer { return temple(p) }
	case "tavern":
		s.Title = "Tavern"
		s.Offers = func() []Offer { return tavern(p) }
	case "trainer":
		s.Title = "Training Grounds"
		s.Offers = func() []Offer { return trainer(p.Chars[s.Char]) }
	case "bank":
		s.Title = "Bank"
		s.Offers = func() []Offer { return bank(p) }
	case "inn":
		s.Title = "Inn"
		s.Offers = func() []Offer { return inn(p) }
	default:
		s.msg = "The " + kind + " is closed."
	}
	return s
}

// spells offers c the spells they can learn and do not know yet.
func spells(c *party.Character) []Offer {
	var out []Offer
	for _, sp := range party.Spells {
		if c.CanLearn() && !c.Knows(sp.ID) {
			out = append(out, Offer{"scroll", fmt.Sprintf("%s learns %s (%d SP)", c.Name, sp.Name, sp.SP), sp.Price, func() string {
				if c.Learn(sp.ID) != nil {
					return ""
				}
				return fmt.Sprintf("%s learns %s: %s.", c.Name, sp.Name, sp.Desc)
			}})
		}
	}
	return out
}

func buy(p *party.Party, kinds ...party.Kind) []Offer {
	var names []string
	for n, it := range party.Catalog {
		for _, k := range kinds {
			if it.Kind == k && party.Tier(it) <= 2 { // up to Steel; finer metals are found, not bought
				names = append(names, n)
			}
		}
	}
	sort.Slice(names, func(i, j int) bool { // by kind, then price
		a, b := party.Catalog[names[i]], party.Catalog[names[j]]
		return a.Kind < b.Kind || a.Kind == b.Kind && (a.Value < b.Value || a.Value == b.Value && names[i] < names[j])
	})
	var out []Offer
	for _, n := range names {
		it := party.NewItem(n, 0)
		out = append(out, Offer{it.Icon, "Buy " + it.Name(), it.Value, func() string {
			p.AddItem(party.NewItem(n, 0))
			return "You buy the " + it.Name() + "."
		}})
	}
	return out
}

// sell offers every unequipped item at half its value; the cost is negative (the party is paid).
func sell(p *party.Party) []Offer {
	var out []Offer
	for _, st := range p.Inventory {
		it := st[0]
		if it.Wearer != nil || it.Kind == party.Key || it.Value == 0 {
			continue
		}
		out = append(out, Offer{it.Icon, fmt.Sprintf("Sell %s (x%d)", it.Name(), len(st)), -it.Value / 2, func() string {
			p.RemoveItem(it)
			return "Sold the " + it.Name() + "."
		}})
	}
	return out
}

func temple(p *party.Party) []Offer {
	missing := 0
	for _, c := range p.Chars {
		if c.HP > 0 {
			missing += c.MaxHP - c.HP + c.MaxSP - c.SP
		}
	}
	out := []Offer{{"potion", "Heal the living", missing, func() string {
		for _, c := range p.Chars {
			if c.HP > 0 {
				c.Cure()
				c.HP, c.SP = c.MaxHP, c.MaxSP
			}
		}
		return "You feel restored."
	}}}
	for _, c := range p.Chars {
		if c.HP <= 0 || c.Has["condition:dead"] {
			out = append(out, Offer{"amulet", "Raise " + c.Name, 100, func() string {
				c.Has["condition:dead"] = false
				c.HP = max(1, c.MaxHP/2)
				return c.Name + " lives again!"
			}})
		}
	}
	return out
}

// Rumors are what the barkeep tells, from maps/rumors.txt (one per line).
var Rumors = []string{"The barkeep has nothing to say."}

func LoadRumors(r io.Reader) {
	var out []string
	for sc := bufio.NewScanner(r); sc.Scan(); {
		if l := strings.TrimSpace(sc.Text()); l != "" && l[0] != '#' {
			out = append(out, l)
		}
	}
	if len(out) > 0 {
		Rumors = out
	}
}

func tavern(p *party.Party) []Offer {
	return []Offer{
		{"bag", "Buy 10 food", 5, func() string { p.Food += 10; return "Your packs are full of bread." }},
		{"potion", "Round of ale for the party", 2, func() string {
			return "The barkeep leans in: \"" + Rumors[rand.Intn(len(Rumors))] + "\""
		}},
		{"bag", "Rest the night", 10, func() string {
			for _, c := range p.Chars {
				if c.HP > 0 {
					c.HP, c.SP = c.MaxHP, c.MaxSP
				}
			}
			p.Day++
			p.Minutes = 8 * 60
			return "You sleep soundly. It is morning."
		}},
	}
}

func trainer(c *party.Character) []Offer {
	need := c.ExpToLevel()
	label := fmt.Sprintf("Train %s to level %d", c.Name, c.Level+1)
	if c.Exp < need {
		label += fmt.Sprintf(" (needs %d exp)", need)
	}
	return []Offer{{"w003", label, c.Level * 50, func() string {
		if c.Exp < need {
			return "" // refused; cost is refunded by the caller
		}
		c.LevelUp()
		return fmt.Sprintf("%s is now level %d!", c.Name, c.Level)
	}}}
}

// inn: create a character, or swap party members with the roster.
func inn(p *party.Party) []Offer {
	out := []Offer{{"scroll", "Sign up a new adventurer", 0, func() string {
		blobber.Modal = nil
		createCharacter(p, func() { blobber.Modal = NewTown("inn", p) })
		return "The innkeeper fetches the register."
	}}}
	for i, c := range p.Roster {
		out = append(out, Offer{"bag", fmt.Sprintf("Take %s (level %d %s) along", c.Name, c.Level, c.Class), 0, func() string {
			if !p.Hire(i) {
				return ""
			}
			return c.Name + " joins the party."
		}})
	}
	for i, c := range p.Chars {
		out = append(out, Offer{"bag", "Leave " + c.Name + " at the inn", 0, func() string {
			if !p.Dismiss(i) {
				return ""
			}
			return c.Name + " waits at the inn."
		}})
	}
	return out
}

// createCharacter asks name, sex, race and class in a chain of prompts, then calls done.
// ponytail: prompt chain, no stat re-rolling; a creation window when the look matters.
func createCharacter(p *party.Party, done func()) {
	pick := func(title string, names []string, on func(string)) {
		lines := []string{title}
		for i, n := range names {
			lines = append(lines, fmt.Sprintf("  %d  %s", i+1, n))
		}
		blobber.ShowPrompt(lines, false, func(a string) {
			n, _ := strconv.Atoi(a)
			if n < 1 || n > len(names) {
				done()
				return
			}
			on(names[n-1])
		})
	}
	blobber.ShowPrompt([]string{"Name of the new adventurer?"}, true, func(name string) {
		if name = strings.TrimSpace(name); name == "" {
			done()
			return
		}
		pick("Sex?", []string{"male", "female"}, func(sex string) {
			pick("Race?", party.Names(party.Races), func(race string) {
				pick("Class?", party.Names(party.Classes), func(class string) {
					c := party.Create(name, sex[:1], race, class, rand.Intn)
					if len(p.Chars) < party.MaxParty {
						p.Chars = append(p.Chars, c)
					} else {
						p.Roster = append(p.Roster, c)
					}
					done()
				})
			})
		})
	})
}

func bank(p *party.Party) []Offer {
	return []Offer{
		{"coin", fmt.Sprintf("Deposit all gold (%d)", p.Gold), 0, func() string {
			p.Bank += p.Gold
			p.Gold = 0
			return fmt.Sprintf("Your account holds %d gold.", p.Bank)
		}},
		{"coin", fmt.Sprintf("Withdraw all gold (%d)", p.Bank), 0, func() string {
			p.Gold += p.Bank
			p.Bank = 0
			return fmt.Sprintf("You carry %d gold.", p.Gold)
		}},
	}
}

// Choose pays for offer i and runs it. An offer whose Do returns "" refuses and is refunded.
func (s *Shop) Choose(i int) {
	o := s.Offers()
	if i < 0 || i >= len(o) {
		return
	}
	if o[i].Cost > s.P.Gold {
		s.msg = "You cannot afford that."
		return
	}
	s.P.Gold -= o[i].Cost
	m := o[i].Do()
	if m == "" {
		s.P.Gold += o[i].Cost
		s.msg = "Not yet."
		return
	}
	s.msg = m
	if s.Sound != nil && o[i].Cost != 0 {
		s.Sound("coins")
	}
}

const shopRows = 11

var (
	shopList = image.Rect(16, 44, cw-16, 284)
	buyBox   = rc(330, 322, 64, 19)
	leaveBox = rc(398, 322, 64, 19)
)

func shopRow(r int) image.Rectangle { return rc(22, 54+r*rowH, cw-44, rowH) }

func (s *Shop) Update() bool {
	p := blobber.KeyJustPressed
	if p(ebiten.KeyEscape) {
		return false
	}
	for i := range min(6, len(s.P.Chars)) {
		if p(ebiten.KeyF1 + ebiten.Key(i)) {
			s.Char = i
		}
	}
	if nc := len(s.P.Chars); p(ebiten.KeyPageDown) || p(ebiten.KeyTab) {
		s.Char = (s.Char + 1) % nc
	} else if p(ebiten.KeyPageUp) {
		s.Char = (s.Char + nc - 1) % nc
	}
	if cx, cy := ebiten.CursorPosition(); inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if i := PartyAt(s.P, cx, cy); i >= 0 {
			s.Char = i
		}
	}
	n := len(s.Offers())
	switch {
	case blobber.KeyRepeated(ebiten.KeyUp):
		s.sel--
	case blobber.KeyRepeated(ebiten.KeyDown):
		s.sel++
	case p(ebiten.KeyEnter) || p(ebiten.KeySpace):
		s.Choose(s.sel)
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		s.sel -= int(wy)
	}
	s.sel = max(0, min(s.sel, n-1))
	x, y := s.mouse()
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	s.hoverBtn = ""
	switch {
	case in(x, y, closeBox) || in(x, y, leaveBox):
		s.hoverBtn = "leave"
		if left {
			return false
		}
	case in(x, y, buyBox):
		s.hoverBtn = "buy"
		if left {
			s.Choose(s.sel)
		}
	}
	for r := range shopRows {
		if i := s.top() + r; i < n && in(x, y, shopRow(r)) {
			s.sel = i
			if left {
				s.Choose(i)
			}
		}
	}
	return true
}

// top is the first visible row, keeping the selection on screen.
func (s *Shop) top() int { return max(0, s.sel-shopRows+1) }

func (s *Shop) Draw(screen *ebiten.Image) {
	d := s.begin()
	Frame(d, rc(0, 0, cw, ch))
	Icon(d, CloseIcon, closeBox.Min.X, closeBox.Min.Y)
	Title(d, strings.ToUpper(s.Title)+" - "+s.P.Chars[s.Char].Name, cw/2, 14)
	DarkFrame(d, shopList)
	o := s.Offers()
	if len(o) == 0 {
		Print(d, "(nothing on offer)", 28, 58, tan)
	}
	for r := 0; r < shopRows && s.top()+r < len(o); r++ {
		i, row := s.top()+r, shopRow(r)
		if i == s.sel {
			fillRect(d, row, selFill)
		}
		icon(d, o[i].Icon, row.Min.X+2, row.Min.Y+2, 1)
		Print(d, o[i].Label, row.Min.X+22, row.Min.Y+3, light)
		cost := ""
		switch {
		case o[i].Cost > 0:
			cost = fmt.Sprintf("%d gold", o[i].Cost)
		case o[i].Cost < 0:
			cost = fmt.Sprintf("+%d gold", -o[i].Cost)
		}
		col := gold
		if o[i].Cost > s.P.Gold {
			col = tan
		}
		Print(d, cost, row.Max.X-font.Width(cost)-4, row.Min.Y+3, col)
	}
	Print(d, s.msg, 20, 296, light)
	icon(d, "coin", 18, 324, 1)
	Print(d, fmt.Sprintf("%d gold   %d in the bank", s.P.Gold, s.P.Bank), 36, 326, light)
	buy := Normal
	if len(o) == 0 {
		buy = Off
	} else if s.hoverBtn == "buy" {
		buy = Hover
	}
	leave := Normal
	if s.hoverBtn == "leave" {
		leave = Hover
	}
	Button(d, buyBox.Min.X, buyBox.Min.Y, buyBox.Dx(), "Buy", buy)
	Button(d, leaveBox.Min.X, leaveBox.Min.Y, leaveBox.Dx(), "Leave", leave)
	s.present(screen)
}

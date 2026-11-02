package engine

import (
	"slices"
	"strings"
)

// Quest journal (extra): the commissions the party has been given, each
// with who gave it where and the very texts the scripts showed when it
// was handed over (e.ext.quests, kept with the save), published as
// View.Quests. A quest is recorded when the text window shows one of its
// texts (scriptText); it is done by the quest table the clerk reads
// (data/assets/scripts/names/global.txt, QuestTable 4AA6-4ABF: 254 reward
// due, 255 rewarded).

// questDef is a quest: its journal name (at most 19 letters: the list's width), giver and place, the texts that
// hand it over (a part of each, in capitals) and its done test.
type questDef struct {
	name, giver, place string
	texts              []string
	done               func(e *Engine) bool
}

// questOver: the quest table's word at script address a says the deed is
// done (254 reward due, 255 rewarded).
func questOver(a int) func(e *Engine) bool {
	return func(e *Engine) bool { return e.ReadVar(a) >= 254 }
}

const clerk, cityHall = "The council clerk", "City Hall, New Phlan"

// quests are the commissions of the scripts (area8 OfferLoop, Cadorna's,
// Urslingen's and the council's chambers; area0 the bishop's study).
var quests = []questDef{
	{"Clear the slums", clerk, cityHall, []string{"THE SLUMS IMMEDIATELY TO OUR WEST"}, questOver(0x4ABB)},
	{"Sokal Keep", clerk, cityHall, []string{"SOKAL KEEP ON THORN ISLAND"}, questOver(0x4AA7)},
	{"Lore of old Phlan", clerk, cityHall, []string{"REWARD FOR BOOKS, MAPS, TOMES"}, func(e *Engine) bool { return e.ReadVar(0x4AC2) == 255 }},
	{"Podal Plaza auction", clerk, cityHall, []string{"A WEAPON OF GREAT POWER IS TO BE AUCTIONED"}, questOver(0x4AB0)},
	{"Cadorna's treasure", "Junior Councilman Cadorna", "His chambers, east of the City Hall clerk",
		[]string{"CADORNA HAS A SPECIAL COMMISSION", "WHEN OLD PHLAN WAS OVERRUN, A FAMILY TREASURE", "THE FAITHFUL SERVANT SENT TO FETCH IT"},
		func(e *Engine) bool {
			return e.ReadVar(0x4AB8) >= 254 || e.ReadVar(0x4AC8) == 255 || e.ReadVar(0x4ABE) >= 254
		}},
	{"Temple of Bane", "Bishop Braccio of Tyr", "The Temple of Tyr, New Phlan",
		[]string{"YOU ARE SUMMONED BY THE BISHOP OF BRACCIO", "USHERED INTO THE BISHOP'S STUDY", "HE IS BOUND TO RECOVER THE TEMPLE"}, questOver(0x4ABD)},
	{"Kovel Mansion", clerk, cityHall, []string{"THIEVES OPERATES OUT OF THE OLD KOVEL MANSION"}, questOver(0x4AB2)},
	{"The nomads", clerk, cityHall, []string{"FIND THE NOMADS AND STOP THEM"}, questOver(0x4AB7)},
	{"The kobold tribe", clerk, cityHall, []string{"A LARGE TRIBE OF KOBOLDS"}, questOver(0x4AB6)},
	{"The poisoned river", clerk, cityHall, []string{"SOURCE OF THE RIVER'S POLLUTION"}, questOver(0x4AB3)},
	{"The lizardmen", clerk, cityHall, []string{"A TRIBE OF LIZARDMEN IS PREPARING"}, questOver(0x4AB5)},
	{"The heir of Bivant", clerk, cityHall, []string{"THE HEIR TO THE HOUSE OF BIVANT"}, func(e *Engine) bool { return e.ReadVar(0x4AA9) >= 128 }},
	{"Zhentil envoys", "Lord Cadorna", "His chambers, south of the City Hall clerk",
		[]string{"CADORNA HAS A DELICATE DIPLOMATIC MISSION", "IT IS IMPERATIVE THAT A MESSAGE BE DELIVERED", "WHEN YOU GET TO ZHENTIAL KEEP", "DELIVER A MESSAGE TO THE ZHENTIL KEEP"},
		func(e *Engine) bool { return e.ReadVar(0x4AB4) != 0 }},
	{"Stojanow Gate", "Lord Urslingen", "Behind Cadorna's old quarters, City Hall",
		[]string{"LORD URSLINGEN WISHES URGENTLY", "WE MUST FIRST SECURE STOJANOW GATE", "THE GATE IS TOO STRONG TO BE TAKEN"}, questOver(0x4AB9)},
	{"Valjevo Castle", "Lord High Mayor Ulrich Eberhard", "The council chambers, City Hall",
		[]string{"SUMMONED TO A SPECIAL COUNCIL MEETING", "THE ENTIRE CITY COUNCIL IS WAITING", "UNANIMOUS REQUEST OF THE COUNCIL"}, questOver(0x4ABA)},
	{"Valhingen Graveyard", clerk, cityHall,
		[]string{"CONCERNING VALHINGEN GRAVEYARD", "THE UNDEAD ARE CROSSING THE RIVER", "UNDEAD FROM THE GRAVEYARD HAVE GROWN", "COMMISSION TO END THE GRAVEYARD MENACE"}, questOver(0x4AB1)},
}

// questNote is a quest the party was given (quests[ID]) and the texts shown.
type questNote struct {
	ID    int
	Texts []string
}

// Quest is a journal entry: Text is what was said, paragraph by paragraph.
type Quest struct {
	Name, Giver, Place string
	Text               []string
	Done               bool
}

// questPrinted records text s of the text window when it hands over a quest.
func (e *Engine) questPrinted(s string) {
	s = strings.TrimSpace(s)
	up := strings.ToUpper(s)
	for id, q := range quests {
		if !slices.ContainsFunc(q.texts, func(t string) bool { return strings.Contains(up, t) }) {
			continue
		}
		i := slices.IndexFunc(e.ext.quests, func(n questNote) bool { return n.ID == id })
		if i < 0 {
			e.ext.quests = append(e.ext.quests, questNote{ID: id})
			i = len(e.ext.quests) - 1
		}
		if n := &e.ext.quests[i]; !slices.Contains(n.Texts, s) {
			n.Texts = append(n.Texts, s)
		}
		return
	}
}

// questsView is the journal: the quests given, newest first.
func (e *Engine) questsView() []Quest {
	var out []Quest
	for _, n := range slices.Backward(e.ext.quests) {
		if n.ID < 0 || n.ID >= len(quests) {
			continue
		}
		q := quests[n.ID]
		out = append(out, Quest{q.name, q.giver, q.place, n.Texts, q.done(e)})
	}
	return out
}

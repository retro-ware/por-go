package engine

import (
	"slices"
	"strings"
)

// Quest journal (extra): the commissions the party has been given
// (e.ext.quests, kept with the save), each with who gave it where and its
// giver's words from the original dialogue (brief), published as
// View.Quests. A quest is recorded when the text window shows one of its
// texts (scriptText): most of the clerk's commissions set no flag when
// offered (area8 OfferLoop), so the text is the only trace. It is also
// listed once a script flag says it was offered (given) or it is done.
// Done is the quest table the clerk reads
// (data/assets/scripts/names/global.txt, QuestTable 4AA6-4ABF: 254 reward
// due, 255 rewarded) or the word the quest's own scripts test.

// questDef is a quest: its journal name (at most 19 letters: the list's width), giver and place, the texts that
// hand it over (a part of each, in capitals), the giver's words, its offered test (nil: no
// flag) and its done test.
type questDef struct {
	name, giver, place string
	brief              []string // the giver's own words: the scripts' texts that hand the quest over
	texts              []string
	done               func(e *Engine) bool
	given              func(e *Engine) bool
}

// varAtLeast: the word at script address a is at least v.
func varAtLeast(a int, v uint16) func(e *Engine) bool {
	return func(e *Engine) bool { return e.ReadVar(a) >= v }
}

// questOver: the quest table's word at script address a says the deed is
// done (254 reward due, 255 rewarded).
func questOver(a int) func(e *Engine) bool {
	return varAtLeast(a, 254)
}

const clerk, cityHall = "The council clerk", "City Hall, New Phlan"

// quests are the commissions of the scripts (area8 OfferLoop, Cadorna's,
// Urslingen's and the council's chambers; area0 the bishop's study).
var quests = []questDef{
	{"Clear the slums", clerk, cityHall, []string{"THE SLUMS IMMEDIATELY TO OUR WEST NEED TO BE CLEARED OF MONSTERS."}, []string{"THE SLUMS IMMEDIATELY TO OUR WEST"}, questOver(0x4ABB), nil},
	{"Sokal Keep", clerk, cityHall, []string{"SOKAL KEEP ON THORN ISLAND MUST BE CLEARED."}, []string{"SOKAL KEEP ON THORN ISLAND"}, questOver(0x4AA7), nil},
	{"Lore of old Phlan", clerk, cityHall, []string{"THE COUNCIL IS OFFERING A REWARD FOR BOOKS, MAPS, TOMES, ETC. WHICH PROVIDE USEFUL INFORMATION ABOUT PHLAN BEFORE THE FALL.  THE REWARD IS TIED TO THE VALUE OF THE INFORMATION."}, []string{"REWARD FOR BOOKS, MAPS, TOMES"}, varAtLeast(0x4AC2, 255), nil},
	{"Podal Plaza auction", clerk, cityHall, []string{"A WEAPON OF GREAT POWER IS TO BE AUCTIONED TO OUR ENEMIES. THIS AUCTION IS TO BE HELD IN PODAL PLAZA.  FIND OUT WHAT THE WEAPON IS AND RETURN."}, []string{"A WEAPON OF GREAT POWER IS TO BE AUCTIONED"}, questOver(0x4AB0), varAtLeast(0x4AB0, 1)}, // 1 AuctionOn: offered
	{"Cadorna's treasure", "Junior Councilman Cadorna", "His chambers, east of the City Hall clerk",
		[]string{"WHEN OLD PHLAN WAS OVERRUN, A FAMILY TREASURE WAS HIDDEN IN THE WESTERN BUILDING OF THE TEXTILE COMPLEX.", "THE FAITHFUL SERVANT SENT TO FETCH IT NEVER RETURNED.  THE COMPLEX IS JUST SOUTH OF PODOL PLAZA.  BRING THE TREASURE TO ME AND YOU WILL BE WELL REWARDED."}, []string{"CADORNA HAS A SPECIAL COMMISSION", "WHEN OLD PHLAN WAS OVERRUN, A FAMILY TREASURE", "THE FAITHFUL SERVANT SENT TO FETCH IT"},
		func(e *Engine) bool { // paid (area0, area8: IronBoxState HandedIn goes with it) or Cadorna exposed (area4)
			return e.ReadVar(0x4AB8) >= 254 || e.ReadVar(0x4ABE) >= 254
		}, varAtLeast(0x4A97, 254)}, // CadornaCommission: offered by the clerk
	{"Temple of Bane", "Bishop Braccio of Tyr", "The Temple of Tyr, New Phlan",
		[]string{"HE IS BOUND TO RECOVER THE TEMPLE WHICH HAS BEEN DESECRATED TO BANE.  GO WITH HIM ACROSS THE RIVER AND HELP HIM TO CLEANSE THE TEMPLE.  IN PAYMENT, YOU MAY KEEP THE HIDDEN TREASURES."}, []string{"YOU ARE SUMMONED BY THE BISHOP OF BRACCIO", "USHERED INTO THE BISHOP'S STUDY", "HE IS BOUND TO RECOVER THE TEMPLE"}, questOver(0x4ABD), varAtLeast(0x4A9B, 254)}, // BishopSummons
	{"Kovel Mansion", clerk, cityHall, []string{"A LARGE GROUP OF THIEVES OPERATES OUT OF THE OLD KOVEL MANSION. THE COUNCIL WILL PAY TO HAVE THEM CLEANED OUT."}, []string{"THIEVES OPERATES OUT OF THE OLD KOVEL MANSION"}, questOver(0x4AB2), nil},
	{"The nomads", clerk, cityHall, []string{"FIND THE NOMADS AND STOP THEM FROM JOINING FORCES WITH OUR ENEMIES."}, []string{"FIND THE NOMADS AND STOP THEM"}, questOver(0x4AB7), nil},
	{"The kobold tribe", clerk, cityHall, []string{"A LARGE TRIBE OF KOBOLDS IS BEING RECRUITED BY THE ENEMY.  MAKE SURE THEY DON'T JOIN THE ENEMY FORCES."}, []string{"A LARGE TRIBE OF KOBOLDS"}, questOver(0x4AB6), nil},
	{"The poisoned river", clerk, cityHall, []string{"FIND THE SOURCE OF THE RIVER'S POLLUTION AND END IT."}, []string{"SOURCE OF THE RIVER'S POLLUTION"}, questOver(0x4AB3), nil}, // 4AB3: Boss (vars.go), 254 Yarash beaten (area23)
	{"The lizardmen", clerk, cityHall, []string{"A TRIBE OF LIZARDMEN IS PREPARING TO JOIN THE ENEMY.  PREVENT THIS."}, []string{"A TRIBE OF LIZARDMEN IS PREPARING"}, questOver(0x4AB5), nil},
	{"The heir of Bivant", clerk, cityHall, []string{"THE HEIR TO THE HOUSE OF BIVANT MUST BE RESCUED.  WE WILL PAY GENEROUSLY FOR HIS SAFE RETURN."}, []string{"THE HEIR TO THE HOUSE OF BIVANT"}, varAtLeast(0x4AA9, 128), nil}, // 128 bungled, 254 rescued (area1): over either way
	{"Zhentil envoys", "Lord Cadorna", "His chambers, south of the City Hall clerk",
		[]string{"IT IS IMPERATIVE THAT A MESSAGE BE DELIVERED TO THE ZHENTIL OUTPOST TO THE WEST.  I AM SENDING YOU AS DIPLOMATIC ENVOYS."}, []string{"CADORNA HAS A DELICATE DIPLOMATIC MISSION", "IT IS IMPERATIVE THAT A MESSAGE BE DELIVERED", "WHEN YOU GET TO ZHENTIAL KEEP", "DELIVER A MESSAGE TO THE ZHENTIL KEEP"},
		varAtLeast(0x4AB4, 1), varAtLeast(0x4A98, 254)}, // area28's own test: 1 commandant killed, 253 left, 254 escorted (area25); ZhentilMission
	{"Stojanow Gate", "Lord Urslingen", "Behind Cadorna's old quarters, City Hall",
		[]string{"THANK YOU FOR COMING.  I MUST BE BRIEF FOR OUR NEED IS URGENT.  OUR ONE CHANCE TO DEFEAT THE ENEMY IS THROUGH AN ASSAULT ON THE CASTLE. TO DO THIS, WE MUST FIRST SECURE STOJANOW GATE."}, []string{"LORD URSLINGEN WISHES URGENTLY", "WE MUST FIRST SECURE STOJANOW GATE", "THE GATE IS TOO STRONG TO BE TAKEN"}, questOver(0x4AB9), nil},
	{"Valjevo Castle", "Lord High Mayor Ulrich Eberhard", "The council chambers, City Hall",
		[]string{"IT IS THE UNANIMOUS REQUEST OF THE COUNCIL THAT YOU LEAD THE ATTACK. YOUR SKILL AND BRAVERY HAVE WON THE ADMIRATION OF ALL PHLAN.  WE LOOK TO YOU TO HELP US WIN OUR FREEDOM."}, []string{"SUMMONED TO A SPECIAL COUNCIL MEETING", "THE ENTIRE CITY COUNCIL IS WAITING", "UNANIMOUS REQUEST OF THE COUNCIL"}, questOver(0x4ABA), nil},
	{"Valhingen Graveyard", clerk, cityHall,
		[]string{"THE UNDEAD ARE CROSSING THE RIVER IN HORDES AND KILLING OR CARRYING OFF OUR SETTLERS.  THE COUNCIL DEMANDS THESE ATTACKS BE STOPPED. A GENEROUS REWARD IS OFFERED."}, []string{"CONCERNING VALHINGEN GRAVEYARD", "THE UNDEAD ARE CROSSING THE RIVER", "UNDEAD FROM THE GRAVEYARD HAVE GROWN", "COMMISSION TO END THE GRAVEYARD MENACE"}, questOver(0x4AB1), nil},
}

// questNote is a quest the party was given (quests[ID]).
type questNote struct {
	ID int
}

// Quest is a journal entry: Text is what its giver said, paragraph by paragraph.
type Quest struct {
	Name, Giver, Place string
	Text               []string
	Done               bool
}

// questPrinted records the quest text s of the text window hands over.
func (e *Engine) questPrinted(s string) {
	s = strings.TrimSpace(s)
	up := strings.ToUpper(s)
	for id, q := range quests {
		if !slices.ContainsFunc(q.texts, func(t string) bool { return strings.Contains(up, t) }) {
			continue
		}
		if !slices.ContainsFunc(e.ext.quests, func(n questNote) bool { return n.ID == id }) {
			e.ext.quests = append(e.ext.quests, questNote{ID: id})
			e.say("Quest added to journal")
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
		out = append(out, Quest{q.name, q.giver, q.place, q.brief, q.done(e)})
	}
	for id, q := range quests { // the flags say so, no text seen (a save from before the journal)
		noted := slices.ContainsFunc(e.ext.quests, func(n questNote) bool { return n.ID == id })
		if done := q.done(e); !noted && (done || q.given != nil && q.given(e)) {
			out = append(out, Quest{q.name, q.giver, q.place, q.brief, done})
		}
	}
	return out
}

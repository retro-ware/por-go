package engine

import "testing"

// TestQuestPrinted: the clerk's offer and Cadorna's briefing make one
// entry each, their texts in order, once; other texts none.
func TestQuestPrinted(t *testing.T) {
	e := &Engine{}
	e.questPrinted("THE CLERK SHUFFLES THROUGH HER PAPERS.")
	e.questPrinted("JUNIOR COUNCILMAN CADORNA HAS A SPECIAL COMMISSION FOR YOU.  HIS CHAMBERS ARE THROUGH THE EAST DOOR.'")
	e.questPrinted("SOKAL KEEP ON THORN ISLAND MUST BE CLEARED.'")
	e.questPrinted("'WHEN OLD PHLAN WAS OVERRUN, A FAMILY TREASURE WAS HIDDEN IN THE WESTERN BUILDING OF THE TEXTILE COMPLEX.")
	e.questPrinted("SOKAL KEEP ON THORN ISLAND MUST BE CLEARED.'")
	n := e.ext.quests
	if len(n) != 2 || quests[n[0].ID].name != "Cadorna's treasure" || len(n[0].Texts) != 2 || quests[n[1].ID].name != "Sokal Keep" || len(n[1].Texts) != 1 {
		t.Fatalf("got %+v", n)
	}
}

// A quest its script flags say was offered, or is done, is in the journal
// without its text seen (a save from before the journal); done follows the
// quest table.
func TestQuestFromFlags(t *testing.T) {
	e := quickStarted(t)
	if q := e.questsView(); len(q) != 0 {
		t.Fatalf("a new party's journal: %+v", q)
	}
	e.WriteVar(0x4A9B, 254) // BishopSummons
	if q := e.questsView(); len(q) != 1 || q[0].Name != "Temple of Bane" || q[0].Done {
		t.Fatalf("summoned by the bishop: %+v", q)
	}
	e.WriteVar(0x4ABD, 255) // QuestTempleOfBane rewarded
	e.WriteVar(0x4AB3, 254) // Boss: Yarash beaten, the river clean
	if q := e.questsView(); len(q) != 2 || !q[0].Done || q[1].Name != "The poisoned river" || !q[1].Done {
		t.Fatalf("both done: %+v", q)
	}
}

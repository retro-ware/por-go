package main

import (
	"cmp"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"blobber"
	"blobber/events"
	"blobber/maps"
	"blobber/party"
	"blobber/sfx"
	"blobber/ui"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	host := newConsole(party.New())
	host.reset(host.newParty())
	pt := host.Party
	if f, err := maps.Open("npcs.txt"); err == nil {
		ui.LoadNPCs(f)
		f.Close()
	}
	for name, load := range map[string]func(io.Reader){"awards.txt": ui.LoadAwards, "rumors.txt": ui.LoadRumors} {
		if f, err := maps.Open(name); err == nil {
			load(f)
			f.Close()
		}
	}
	w, err := host.enter(host.start.m)
	if err != nil {
		log.Fatal(err)
	}

	blobber.DebugLines = pt.Summary
	blobber.HUD = func(s *ebiten.Image) { ui.DrawParty(s, pt) }
	blobber.OnPromptAnswer = func() []string { // character list for "who will" prompts
		var l []string
		for i, c := range pt.Chars {
			l = append(l, fmt.Sprintf("  F%d  %s", i+1, c.Name))
		}
		return l
	}
	blobber.OnTeleportMenu = host.teleportMenu
	blobber.OnMove = func(x, y, dir int) {
		blobber.ClearSign()
		pump(host.vm, host.vm.Trigger(host.m, x, y, events.Facing(dir)))
	}
	// F: take what lies here; with nothing to take, search the tile.
	blobber.OnUse = func(x, y, dir int) {
		if got := host.TakeFloor(x, y); len(got) > 0 {
			blobber.ShowSign(x, y, dir, []string{"Taken: " + strings.Join(got, ", ")})
			return
		}
		pump(host.vm, host.vm.Use(host.m, x, y, events.Facing(dir)))
	}
	blobber.Keys[ebiten.KeyJ] = func() { blobber.Modal = &ui.Journal{P: pt} }
	blobber.Keys[ebiten.KeyR] = host.rest
	blobber.Keys[ebiten.KeyG] = toggleGod
	blobber.Keys[ebiten.KeyK] = killAhead
	blobber.Keys[ebiten.KeyN] = host.spawnMenu
	blobber.Keys[ebiten.KeyT] = func() { blobber.Night = !blobber.Night } // phlan-poc: day / night
	blobber.Keys[ebiten.KeyF5] = func() { report(host.saveGame(), "Game saved.") }
	blobber.Keys[ebiten.KeyF9] = func() { report(host.loadGame(), "Game loaded.") }
	blobber.Keys[ebiten.KeyI] = func() { blobber.Modal = ui.NewInventory(pt) }
	for i := range pt.Chars { // F1-F6: that character's inventory
		blobber.Keys[ebiten.KeyF1+ebiten.Key(i)] = func() {
			inv := ui.NewInventory(pt)
			inv.Select(i)
			blobber.Modal = inv
		}
	}
	sfx.Init()
	blobber.Keys[ebiten.KeyC] = func() { blobber.Modal = ui.NewSpellbook(pt) }
	pt.OnCast = func(s party.Spell) {
		sfx.Play("magic")
		if blobber.Fighting() { // a melee round: the round passes the turn and aims the bolts
			if s.Kind == "light" {
				blobber.Light(60)
			}
			return
		}
		blobber.EndTurn()
		switch s.Kind {
		case "light":
			blobber.Light(60)
		case "bolt":
			blobber.Bolt(s.Power, 14, 0.6, 0.8, 1)
		case "fire":
			blobber.Bolt(s.Power, 5, 1, 0.3, 0.1)
		}
	}
	host.installCombat()
	pt.OnHit = func(who, n int, kind string) {
		ui.Hit(who, n, kind)
		sfx.Play("hit")
		if host.Wiped() {
			blobber.Modal = &ui.Talk{Text: "Your party has fallen.", Done: func(bool) { host.title() }}
		}
	}
	// clicking a portrait opens that character's inventory
	blobber.OnClick = func(x, y int) bool {
		i := ui.PartyAt(pt, x, y)
		if i >= 0 {
			inv := ui.NewInventory(pt)
			inv.Select(i)
			blobber.Modal = inv
		}
		return i >= 0
	}

	ui.Install()
	loadSettings()
	devOpen(host)

	if os.Getenv("BLOBBER_SHOT") == "" { // phlan-poc: screenshots skip the title menu
		host.title()
	}
	if err := blobber.Run(w.lvl, host.start.x, host.start.y, host.start.dir); err != nil {
		log.Fatal(err)
	}
}

// report shows the outcome of a save or load in the message strip.
func report(err error, ok string) {
	if err != nil {
		blobber.Message(err.Error())
		return
	}
	blobber.Message(ok)
}

// devOpen opens windows named in BLOBBER_OPEN (comma separated) at startup, for checking the look
// together with BLOBBER_SHOT: inventory[:ITEM to wear], town:KIND, cutscene:NAME, hit, prompt, sign,
// map:NAME:X:Y:DIR (DIR 0-3, north first).
func devOpen(host *console) {
	for _, s := range strings.Split(os.Getenv("BLOBBER_OPEN"), ",") {
		kind, arg, _ := strings.Cut(s, ":")
		switch kind {
		case "inventory":
			inv := ui.NewInventory(host.Party)
			if arg != "" { // a character's slots with the first equippable item worn
				inv.Select(0)
				host.Party.Equip(host.Party.Chars[0], host.Party.FindItem(arg))
			}
			blobber.Modal = inv
		case "town":
			host.Town(arg)
		case "map": // after the first frame, like sign
			var m string
			var x, y, d int
			fmt.Sscanf(strings.ReplaceAll(arg, ":", " "), "%s %d %d %d", &m, &x, &y, &d)
			hud := blobber.HUD
			blobber.HUD = func(s *ebiten.Image) {
				blobber.HUD = hud
				host.Teleport(m, x, y)
				blobber.Face(d)
				hud(s)
			}
		case "cutscene":
			host.Cutscene(arg)
		case "hit":
			host.Party.Damage(0, 3, cmp.Or(arg, "fire"))
		case "help": // as a window, for the look
			blobber.Modal = &ui.HelpScreen{}
		case "spells": // Bob knows every spell, book open
			for _, s := range party.Spells {
				host.Party.Chars[1].Learn(s.ID)
			}
			blobber.Modal = ui.NewSpellbook(host.Party)
		case "npc": // npc:ID shows that NPC talking
			blobber.Modal = &ui.Talk{Who: arg, Text: "Peace, travellers. The temple heals the wounded and lifts curses, for an offering. Will you give?", Ask: true}
		case "prompt":
			blobber.ShowPrompt([]string{"Who will search the rubble?", "  F1  Ann", "  F2  Bob", "  F3  Cyd"}, false, func(string) {})
		case "sign": // after the first frame: Run clears signs
			hud := blobber.HUD
			blobber.HUD = func(s *ebiten.Image) {
				blobber.ShowSign(1, 1, 1, []string{"Welcome to the demo dungeon.", "Press F here for the opcode lab."})
				blobber.HUD = hud
				hud(s)
			}
		}
	}
}

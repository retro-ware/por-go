package ui

import (
	"bufio"
	"bytes"
	"embed"
	"image/png"
	"io"
	"strings"

	"blobber/party"

	"github.com/hajimehoshi/ebiten/v2"
)

// One pool of portraits for party members and NPCs alike: assets/portraits/<id>.png.
//
//go:embed assets/portraits/*.png
var portraitFS embed.FS

// PortraitSize is the side of a portrait image.
const PortraitSize = 45

// portraitArt maps a class to its default portrait set: pal, mage or ranger, each with man and lady.
var portraitArt = map[string]string{
	"knight": "pal", "paladin": "pal",
	"sorcerer": "mage", "cleric": "mage", "mage": "mage", "monk": "mage",
	"robber": "ranger", "archer": "ranger", "ranger": "ranger",
}

var portraits = map[string]*ebiten.Image{}

// PortraitID returns the portrait from the pool by id, or nil.
func PortraitID(id string) *ebiten.Image {
	if img, ok := portraits[id]; ok {
		return img
	}
	var img *ebiten.Image
	if b, err := portraitFS.ReadFile("assets/portraits/" + id + ".png"); err == nil {
		if m, err := png.Decode(bytes.NewReader(b)); err == nil {
			img = ebiten.NewImageFromImage(m)
		}
	}
	portraits[id] = img
	return img
}

// Portrait returns c's picture: c.Portrait if set and known, else by class (unknown classes look
// like paladins) and sex ("f" = lady).
func Portrait(c *party.Character) *ebiten.Image {
	if img := PortraitID(c.Portrait); img != nil {
		return img
	}
	id := portraitArt[strings.ToLower(c.Class)]
	if id == "" {
		id = "pal"
	}
	if strings.HasPrefix(strings.ToLower(c.Sex), "f") {
		id += "lady"
	} else {
		id += "man"
	}
	return PortraitID(id)
}

// NPC is a speaker scripts refer to by id.
type NPC struct{ Name, Portrait string }

// NPCs is the registry, filled by LoadNPCs.
var NPCs = map[string]NPC{}

// LoadNPCs reads registry lines "id portrait Display Name"; # starts a comment.
func LoadNPCs(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 3 || strings.HasPrefix(f[0], "#") {
			continue
		}
		NPCs[f[0]] = NPC{strings.Join(f[2:], " "), f[1]}
	}
}

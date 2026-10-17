package sprites

import "strings"

// Portraits share one face; H (headgear/hair) and C (clothes) are recoloured per class.
// Get("portrait_<class>"); unknown classes fall back to "portrait".
const face = `
....kkkkkkkk....
...kHHHHHHHHk...
..kHHHHHHHHHHk..
..kHHHHHHHHHHk..
..kHffffffffHk..
..kffkffffkffk..
..kffffffffffk..
..kffffbbffffk..
..kffffffffffk..
...kffrrrrffk...
....kffffffk....
.....kkkkkk.....
...kkCCffCCkk...
..kCCCCCCCCCCk..
.kCCCCCCCCCCCCk.
.kCCCCCCCCCCCCk.`

func init() {
	for class, hc := range map[string]string{
		"":         "bu",
		"knight":   "ss",
		"paladin":  "yY",
		"sorcerer": "pp",
		"cleric":   "wy",
		"robber":   "BG",
		"archer":   "ee",
	} {
		name := "portrait"
		if class != "" {
			name += "_" + class
		}
		art[name] = strings.NewReplacer("H", hc[:1], "C", hc[1:]).Replace(face)
	}
}

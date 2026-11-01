package remaster

import (
	"fmt"
	"testing"

	"por/dax"
	"por/engine"
	"por/phlan-poc/blobber"
	"por/phlan-poc/blobber/maps"
)

// Every 3D area of the game (a GEO block) has its remastered map: the
// classic page never stands in for one.
func TestEveryAreaHasMap(t *testing.T) {
	l, err := dax.NewLoader("../../data/dos")
	if err != nil {
		t.Skip("no game files")
	}
	for disk := 1; disk <= 8; disk++ {
		f := l.File(fmt.Sprintf("GEO%d", disk))
		if f == nil {
			t.Skip("no GEO files")
		}
		for b := 0; b < 64; b++ {
			if f.Block(b) == nil {
				continue
			}
			n, ok := areaMaps[engine.AreaID(b)]
			if !ok {
				t.Errorf("GEO%d block %d: no remastered map (phlan-poc/areas.py, make_map.py)", disk, b)
				continue
			}
			m, err := maps.Levels.Open(n + ".map")
			if err != nil {
				t.Errorf("GEO%d block %d: %v", disk, b, err)
				continue
			}
			if _, _, err := blobber.ParseScene(m); err != nil {
				t.Errorf("%s.map: %v", n, err)
			}
			m.Close()
		}
	}
}


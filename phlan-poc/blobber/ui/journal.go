package ui

import (
	"bufio"
	"io"
	"sort"
	"strings"

	"blobber"
	"blobber/party"

	"gpu"
)

// Awards maps an award id to its journal text (maps/awards.txt).
var Awards = map[string]string{}

// LoadAwards reads "id text..." lines.
func LoadAwards(r io.Reader) {
	for sc := bufio.NewScanner(r); sc.Scan(); {
		f := strings.Fields(strings.SplitN(sc.Text(), "#", 2)[0])
		if len(f) >= 2 {
			Awards[f[0]] = strings.Join(f[1:], " ")
		}
	}
}

// Journal lists the party's awards: the quest log.
type Journal struct{ P *party.Party }

func (j *Journal) Update() bool {
	return !blobber.KeyJustPressed(gpu.KeyEscape) && !blobber.KeyJustPressed(gpu.KeyJ) &&
		!gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft)
}

func (j *Journal) Draw(screen *gpu.Image) {
	seen := map[string]bool{}
	var lines []string
	for _, c := range j.P.Chars {
		for k, v := range c.Has {
			if id, ok := strings.CutPrefix(k, "award:"); ok && v && !seen[id] {
				seen[id] = true
				t := Awards[id]
				if t == "" {
					t = id
				}
				lines = append(lines, t)
			}
		}
	}
	sort.Strings(lines)
	if len(lines) == 0 {
		lines = []string{"Nothing to report yet."}
	}
	blobber.Window(screen, "JOURNAL", lines, -1, false)
}

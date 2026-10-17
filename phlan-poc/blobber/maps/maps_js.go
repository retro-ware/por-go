//go:build js

package maps

import (
	"embed"
	"io"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.map *.evt *.txt cutscenes
var embedded embed.FS

// Open opens the copy of maps/<name> built into the browser version.
func Open(name string) (io.ReadCloser, error) { return embedded.Open(name) }

// List names every built-in level (maps/<name>.map), sorted.
func List() []string {
	names, _ := fs.Glob(embedded, "*.map")
	for i, n := range names {
		names[i] = strings.TrimSuffix(n, ".map")
	}
	sort.Strings(names)
	return names
}

//go:build !js

// Package maps opens the game's maps, event scripts, NPC registry and cutscenes:
// from the maps/ directory on disk, except in the browser build, which embeds them.
package maps

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Open opens maps/<name> on disk.
func Open(name string) (io.ReadCloser, error) { return os.Open("maps/" + name) }

// List names every level on disk (maps/<name>.map), sorted.
func List() []string {
	names, _ := filepath.Glob("maps/*.map")
	for i, n := range names {
		names[i] = strings.TrimSuffix(filepath.Base(n), ".map")
	}
	sort.Strings(names)
	return names
}

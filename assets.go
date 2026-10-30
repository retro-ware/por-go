// Package por holds what the port adds to the game and `por -setup` writes
// into data/: the area scripts as text (data/assets/scripts, see
// MODERNIZED.md §6). They are built into the binary so an installation is
// complete without the repository.
package por

import "embed"

//go:embed data/assets/scripts
var Assets embed.FS

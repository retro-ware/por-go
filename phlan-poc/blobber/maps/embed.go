package maps

import "embed"

// Levels holds the .map files built into the program, for hosts that draw
// the levels without the maps/ folder at hand (the remastered Pool of Radiance).
//
//go:embed *.map
var Levels embed.FS

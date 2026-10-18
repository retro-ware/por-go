package engine

import "strconv"

// Set is one of the game's numbered data sets: most files come in eight
// numbered copies (ECL1..8, GEO, WALLDEF, 8X8D, ITEM, MONnCHA, HEAD, BODY,
// FINAL ...), one per part of the world, and the current area reads its
// set's (DS:5376, saved with the game, changed by the area scripts).
// Loaders name the set they read: the area's (Engine.set), or a fixed one.
type Set byte

// charArt is the set the party's portraits come from: the character
// creation hall's, whichever set the current area reads.
const charArt Set = 3

// file is the name of family's file in set s ("ECL" -> "ECL3").
func (s Set) file(family string) string { return family + strconv.Itoa(int(s)) }

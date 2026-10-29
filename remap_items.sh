#!/bin/sh
# Edit the inventory's item icons with remapper (~/Projects/remapper):
# items.rec maps each item key to a 16x16 cell of atlas.png. Press s in
# remapper to save, then rebuild the game (the files are embedded).
set -e
cd "$(dirname "$0")/present/remaster/items"
exec remapper 16 16 atlas.png items.rec

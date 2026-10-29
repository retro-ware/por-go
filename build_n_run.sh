#!/bin/sh
# Build the Go port and play it with the original game files in this folder
# (the *.DAX files, START.EXE, ITEMS, ...; they are not in git).
#
#   ./build_n_run.sh            play in a window (scale 3)
#   ./build_n_run.sh -scale 4   any extra flags go to the game
#   ./build_n_run.sh remaster   start in the remastered view (F10 switches)
#   ./build_n_run.sh snap OUT game A '^^<|'
#                               headless: load save A, press keys, one PNG
#                               per key in OUT (^ v < > arrows, { } Home / End, | Enter,
#                               @ # $ gamepad A / Y / X,
#                               [ ] ( ) d-pad up / down / left / right, !N F1-F6, ? help,
#                               %cX,Y; click text cell X,Y, %hX,Y; point there,
#                               %u %d wheel, %r right button, %e Esc, %l Ctrl-L,
#                               _ wait; slot - stays in the party menu)
set -e
root=$(cd "$(dirname "$0")" && pwd)
cd "$root"
if [ ! -f START.EXE ]; then
	echo "no game data: copy your Pool of Radiance files (START.EXE, *.DAX, ITEMS, ...) into $root" >&2
	exit 1
fi
mkdir -p bin
if [ "$1" = snap ]; then
	shift
	mkdir -p "$1"
	go build -o bin/snap ./cmd/snap
	exec bin/snap "$root" "$@"
fi
if [ "$1" = remaster ]; then
	shift
	export POR_REMASTER=1
fi
go build -o bin/por ./cmd/por
ln -sfn "$root" bin/data # por reads ./data
cd bin && exec ./por "$@"

#!/bin/sh
# Build the Go port and play it with the Amiga graphics and sound: the DOS
# game files in this folder plus DISK2, converted from the Amiga .adf disks
# (loose or zipped) under $ADF_DIR (default ~/Downloads). The game is staged
# in bin/amiga, so ./build_n_run.sh stays DOS.
#
#   ./build_n_run_amiga.sh            play in a window (scale 3)
#   ./build_n_run_amiga.sh -scale 4   any extra flags go to the game
#   ./build_n_run_amiga.sh remaster   start in the remastered view
#   ADF_DIR=~/amiga ./build_n_run_amiga.sh
set -e
root=$(cd "$(dirname "$0")" && pwd)
cd "$root"
if [ ! -f START.EXE ]; then
	echo "no game data: copy your Pool of Radiance files (START.EXE, *.DAX, ITEMS, ...) into $root" >&2
	exit 1
fi
mkdir -p bin
go build -o bin/por ./cmd/por
data=bin/amiga
if [ ! -d "$data/DISK2" ]; then
	mkdir -p "$data"
	for f in "$root"/*; do
		[ -f "$f" ] && ln -sf "$f" "$data/"
	done
	bin/por -data "$data" -import-disk2 "${ADF_DIR:-$HOME/Downloads}"
fi
if [ "$1" = remaster ]; then
	shift
	export POR_REMASTER=1
fi
exec bin/por -data "$data" "$@"

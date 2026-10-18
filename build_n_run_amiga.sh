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
mkdir -p "$data"
for f in "$root"/*; do # every run, so new files show up too
	[ -f "$f" ] && ln -sf "$f" "$data/"
done
[ -e "$data/SAVE" ] || ln -s "$root/SAVE" "$data/SAVE" # one set of saves for DOS and Amiga
[ -d "$data/DISK2" ] || bin/por -data "$data" -import-disk2 "${ADF_DIR:-$HOME/Downloads}"
if [ "$1" = remaster ]; then
	shift
	export POR_REMASTER=1
fi
exec bin/por -data "$data" "$@"

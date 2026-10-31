#!/bin/sh
# Build the remastered game (./por) and play it with the files in ./data:
#   data/dos     the DOS Pool of Radiance files it reads (*.DAX, ITEMS, POOL.CFG; not in git)
#   data/amiga   the Amiga graphics and sound, converted on the first run from
#                the Amiga .adf disks under $ADF_DIR (default ~/Downloads)
#   data/assets  fonts, speech and the event scripts (scripts/ is in git); dasp/,
#                the Dubtrain Angband Sound Pack, copied on the first run from
#                $DASP_DIR (default ~/Downloads/Dubtrain Angband Sound Pack v3.1.0)
#   data/save    the saves and what the game writes
#
#   ./build_n_run.sh            play in a window (scale 3)
#   ./build_n_run.sh -scale 4   any extra flags go to the game
#   ADF_DIR=~/amiga ./build_n_run.sh
set -e
cd "$(dirname "$0")"
if [ ! -f data/dos/TITLE.DAX ]; then
	echo "no game data: copy your Pool of Radiance files (*.DAX, ITEMS, POOL.CFG) into $(pwd)/data/dos" >&2
	exit 1
fi
go build -o por ./cmd/por
[ -d data/amiga ] || ./por -data data -import-disk2 "${ADF_DIR:-$HOME/Downloads}"
dasp="${DASP_DIR:-$HOME/Downloads/Dubtrain Angband Sound Pack v3.1.0}"
[ -d data/assets/dasp ] || [ ! -f "$dasp/sound.cfg" ] || { mkdir -p data/assets/dasp && cp "$dasp"/* data/assets/dasp/; }
exec ./por "$@" # por reads ./data

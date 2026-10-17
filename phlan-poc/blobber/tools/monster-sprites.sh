#!/usr/bin/env bash
# Turn every still in a folder into an animated monster sheet with sprite-gen.
#   tools/monster-sprites.sh [art/monsters]
# in : <dir>/<name>.{png,jpg,jpeg}  — one monster on a flat background
# out: cmd/blobber/assets/monsters/<name>/sheet.png + manifest.json (frame_layout rects, fps, loop per state)
#      <dir>/.runs/<name>/ keeps the sprite-gen run (raw rows, frames, qa gifs); delete it to regenerate.
# Needs: git, python3, and an OpenAI API key (OPENAI_API_KEY, else the one codex stores) — billed per image, 4 per monster.
set -euo pipefail
IN=${1:-art/monsters}
OUT=cmd/blobber/assets/monsters
ROOT=${SPRITE_GEN_ROOT:-$HOME/.sprite-gen}
SG=$ROOT/.venv/bin/sprite-gen
STATES=(idle attack hurt death)
REQUEST='{"states":{
 "idle":  {"frames":4,"fps":4,"loop":true, "action":"menacing idle: subtle breathing and weight shifting, weapon held ready"},
 "attack":{"frames":4,"fps":8,"loop":false,"action":"windup, heavy strike toward the viewer, follow-through, recovery back toward idle; no detached effects"},
 "hurt":  {"frames":3,"fps":8,"loop":false,"action":"recoils from a hit: flinches back with head and torso turned away, then steadies"},
 "death": {"frames":5,"fps":6,"loop":false,"action":"collapses: staggers, drops to knees, falls forward and lies still on the ground in the last frame"}
}}'

if [ ! -x "$SG" ]; then
  echo "installing sprite-gen into $ROOT"
  [ -d "$ROOT" ] || git clone -q https://github.com/aldegad/sprite-gen "$ROOT"
  python3 -m venv "$ROOT/.venv" && "$ROOT/.venv/bin/pip" install -q -e "$ROOT"
fi
: "${OPENAI_API_KEY:=$(python3 -c 'import json;print(json.load(open("'"$HOME"'/.codex/auth.json")).get("OPENAI_API_KEY",""))' 2>/dev/null || true)}"
[ -n "$OPENAI_API_KEY" ] || { echo "set OPENAI_API_KEY"; exit 1; }
export OPENAI_API_KEY

shopt -s nullglob nocaseglob
for img in "$IN"/*.{png,jpg,jpeg}; do
  name=$(basename "${img%.*}")
  run=$IN/.runs/$name
  [ -f "$OUT/$name/manifest.json" ] && { echo "$name: done, skip"; continue; }
  echo "== $name"
  [ -f "$run/sprite-request.json" ] || "$SG" prepare --out-dir "$run" --character-id "$name" --base-image "$img" --request <(echo "$REQUEST") >/dev/null
  mkdir -p "$run/raw" "$run/reports/gen-set"
  for s in "${STATES[@]}"; do   # gen-set only drives codex/grok, so drive gen per row ourselves
    [ -f "$run/raw/$s.png" ] && continue
    echo "  gen $s"
    "$SG" gen --provider openai --prompt-file "$run/prompts/$s.txt" --out "$run/raw/$s.png" \
      --report "$run/reports/gen-set/$s.json" --ref "$run"/base-source.* --ref "$run/references/layout-guides/$s.png" >/dev/null &
  done
  wait
  "$SG" extract --run-dir "$run" >/dev/null
  "$SG" compose-atlas --run-dir "$run" >/dev/null
  mkdir -p "$OUT/$name"
  cp "$run/sprite-sheet-alpha.png" "$OUT/$name/sheet.png"
  cp "$run/manifest.json" "$OUT/$name/manifest.json"
  echo "  -> $OUT/$name  (qa gifs: $run/qa/)"
done

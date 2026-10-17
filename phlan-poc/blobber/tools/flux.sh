#!/usr/bin/env bash
# Local FLUX.2 klein (4B, MLX via mflux) image generation.
#   tools/flux.sh "a red dragon, pixel art" [out.png] [WxH] [ref.png ...]
# With ref images it edits/conditions on them; without, plain text-to-image.
# env: FLUX_MODEL (flux2-klein-4b | flux2-klein-9b), FLUX_STEPS (4), FLUX_SEED (random)
# First run installs mflux (uv) and downloads the model to ~/.cache/huggingface.
set -euo pipefail
[ $# -ge 1 ] || { sed -n 2,6p "$0"; exit 1; }
PROMPT=$1; OUT=${2:-flux-$(date +%s).png}; SIZE=${3:-1024x1024}; shift $(( $# < 3 ? $# : 3 ))
BIN=$HOME/.local/bin
if [ ! -x "$BIN/mflux-generate-flux2" ]; then
  command -v uv >/dev/null || brew install uv
  uv tool install mflux
fi
args=(--model "${FLUX_MODEL:-flux2-klein-4b}" --quantize 8 --steps "${FLUX_STEPS:-4}"
      --width "${SIZE%x*}" --height "${SIZE#*x}" --seed "${FLUX_SEED:-$RANDOM}"
      --prompt "$PROMPT" --output "$OUT")
if [ $# -gt 0 ]; then "$BIN/mflux-generate-flux2-edit" "${args[@]}" --image-paths "$@"
else "$BIN/mflux-generate-flux2" "${args[@]}"; fi
echo "$OUT"

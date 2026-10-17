#!/bin/sh
# Generate floor, wall and ceiling textures with OpenAI (see tools/new-texture.py).
cd "$(dirname "$0")" && exec python3 tools/new-texture.py "$@"

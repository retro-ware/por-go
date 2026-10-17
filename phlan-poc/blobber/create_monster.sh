#!/bin/sh
# Generate monsters with Gemini (see tools/new-monster.py).
cd "$(dirname "$0")" && exec python3 tools/new-monster.py "$@"

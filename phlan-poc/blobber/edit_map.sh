#!/bin/sh
# Map editor: ./edit_map.sh [mapname]  (default: demo)
cd "$(dirname "$0")" && exec go run ./cmd/editor "$@"

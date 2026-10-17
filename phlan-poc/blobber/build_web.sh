#!/bin/zsh
# Builds the web version into fx-games site (or $1).
OUT=${1:-/Users/felix/Projects/fx-games/site/blobber/play}
mkdir -p "$OUT"
cd "$(dirname $0)" || exit
GOOS=js GOARCH=wasm go build -tags ebitensinglethread -o "$OUT/blobber.wasm" -trimpath -ldflags '-s -w' ./cmd/blobber || exit
gzip -9 -k -f "$OUT/blobber.wasm"
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$OUT/" 2>/dev/null || cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" "$OUT/"

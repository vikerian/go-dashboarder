#!/bin/sh
# Builds a statically linked binary for every component under cmd/ and
# drops it into _release/<GOOS>-<GOARCH>/, ready to copy onto a target
# machine (e.g. scp the whole directory to a FreeBSD box and run it there).
#
# Written in POSIX sh on purpose - FreeBSD's default /bin/sh is not bash,
# so this avoids bashisms ([[, arrays, `local`, etc.).
#
# Usage:
#   ./build.sh                          # defaults to freebsd/amd64
#   GOOS=freebsd GOARCH=arm64 ./build.sh
#   GOOS=linux   GOARCH=arm64 ./build.sh
#
set -eu

GOOS="${GOOS:-freebsd}"
GOARCH="${GOARCH:-amd64}"
export CGO_ENABLED=0
export GOOS
export GOARCH

OUT_DIR="_release/${GOOS}-${GOARCH}"

rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR"

echo "Building for GOOS=$GOOS GOARCH=$GOARCH -> $OUT_DIR"

for dir in cmd/*/; do
    name=$(basename "${dir%/}")
    echo "  - $name"
    go build -trimpath -ldflags="-s -w" -o "$OUT_DIR/$name" "./${dir%/}"
done

echo "Copying runtime assets (configs/, web/)"
cp -r configs "$OUT_DIR/configs"
cp -r web "$OUT_DIR/web"

echo "Done. Release artifacts in $OUT_DIR"

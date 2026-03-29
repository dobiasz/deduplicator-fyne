#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

GO111MODULE=on CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" go build -o deduplicator-fyne .
echo "Built $(pwd)/deduplicator-fyne"

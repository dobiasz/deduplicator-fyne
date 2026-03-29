#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

# Go 1.26.x currently fails to resolve some legacy module packages used by Fyne's test driver deps.
# Keep builds stable by defaulting to the project-compatible 1.22 toolchain.
: "${GOTOOLCHAIN:=go1.22.12}"

GO111MODULE=on CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" GOTOOLCHAIN="$GOTOOLCHAIN" go build -o deduplicator-fyne .
echo "Built $(pwd)/deduplicator-fyne"

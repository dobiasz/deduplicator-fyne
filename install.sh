#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

# Build the binary if not already present
if [ ! -f deduplicator-fyne ]; then
    ./build.sh
fi

# Copy to /usr/local/bin
sudo cp deduplicator-fyne /usr/local/bin/deduplicator-fyne
echo "Installed deduplicator-fyne to /usr/local/bin/deduplicator-fyne"
echo "Run 'deduplicator-fyne' from anywhere to launch the app"

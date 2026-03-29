# deduplicator-fyne

This folder contains a Go + Fyne rewrite of the JavaFX duplicate scanner UI.

## Run

1. Install Go 1.22+ if needed.
2. From this folder:

```sh
cd deduplicator-fyne
go mod tidy
go run .
```

**Note:** The `build.sh` script defaults to Go 1.22.12 because Go 1.26.x has a module resolution issue with legacy packages transitively required by Fyne's test dependencies. You can override this by setting `GOTOOLCHAIN` before running the script (e.g., `GOTOOLCHAIN=go1.27.0 ./build.sh`).

## Features

- Pick folders to scan for duplicate files.
- Optionally remove internal duplicates inside each directory.
- Optionally skip MP3 and M4A files.
- View duplicate groups in a scrollable list.
- Revalidate and sort duplicate groups.
- Open file location for individual duplicate entries.

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

## Features

- Pick folders to scan for duplicate files.
- Optionally remove internal duplicates inside each directory.
- Optionally skip MP3 and M4A files.
- View duplicate groups in a scrollable list.
- Revalidate and sort duplicate groups.
- Open file location for individual duplicate entries.

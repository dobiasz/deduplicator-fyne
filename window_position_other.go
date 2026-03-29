//go:build !darwin

package main

import "fyne.io/fyne/v2"

func restoreWindowPosition(_ fyne.App, _ fyne.Window) {}

func saveWindowPosition(_ fyne.App, _ fyne.Window) {}

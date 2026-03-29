//go:build !darwin

package main

import "fyne.io/fyne/v2"

func chooseFolder(w fyne.Window) (string, error) {
	return "", nil
}

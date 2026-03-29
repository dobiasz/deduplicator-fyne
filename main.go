package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const (
	appID              = "com.dobiasz.deduplicator-fyne"
	windowWidthPrefKey  = "window.width"
	windowHeightPrefKey = "window.height"
	windowXPrefKey      = "window.x"
	windowYPrefKey      = "window.y"
	defaultWindowWidth  = 1200
	defaultWindowHeight = 850
)

type AppUI struct {
	app            fyne.App
	win            fyne.Window
	roots          []string
	selectedRoot   int
	scanRunID      uint64
	rootsList      *widget.List
	duplicateBox   *fyne.Container
	progressBar    *widget.ProgressBar
	statusLabel    *widget.Label
	startBtn       *widget.Button
	stopBtn        *widget.Button
	revalidateBtn  *widget.Button
	sortBtn        *widget.Button
	removeInternal *widget.Check
	skipMp3M4a     *widget.Check
	scanManager    *ScanManager
	panels         []*DuplicatePanel
}

type DuplicatePanel struct {
	container *fyne.Container
	firstPath string
	filePaths []string
}

func main() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	a := app.NewWithID(appID)
	w := a.NewWindow("FX Hello World - Go/Fyne")
	ui := newAppUI(a, w)
	ui.build()
	applySavedWindowSize(a, w)
	restoreWindowPosition(a, w)
	installWindowSizePersistence(a, w)
	w.ShowAndRun()
}

func applySavedWindowSize(a fyne.App, w fyne.Window) {
	prefs := a.Preferences()
	width := prefs.FloatWithFallback(windowWidthPrefKey, defaultWindowWidth)
	height := prefs.FloatWithFallback(windowHeightPrefKey, defaultWindowHeight)

	if width < 600 {
		width = defaultWindowWidth
	}
	if height < 400 {
		height = defaultWindowHeight
	}

	w.Resize(fyne.NewSize(float32(width), float32(height)))
}

func installWindowSizePersistence(a fyne.App, w fyne.Window) {
	w.SetCloseIntercept(func() {
		saveWindowPosition(a, w)

		size := w.Canvas().Size()
		prefs := a.Preferences()
		prefs.SetFloat(windowWidthPrefKey, float64(size.Width))
		prefs.SetFloat(windowHeightPrefKey, float64(size.Height))
		w.Close()
	})
}

func newAppUI(a fyne.App, w fyne.Window) *AppUI {
	ui := &AppUI{
		app:          a,
		win:          w,
		selectedRoot: -1,
		scanManager:  &ScanManager{},
	}
	return ui
}

func (ui *AppUI) build() {
	ui.progressBar = widget.NewProgressBar()
	ui.statusLabel = widget.NewLabel("Ready")
	ui.removeInternal = widget.NewCheck("Remove internal duplicates", nil)
	ui.skipMp3M4a = widget.NewCheck("Skip MP3 & M4A", nil)
	ui.sortBtn = widget.NewButton("Sort", ui.sortDuplicates)
	ui.sortBtn.Disable()
	ui.revalidateBtn = widget.NewButton("Revalidate", ui.revalidateDuplicates)

	ui.rootsList = widget.NewList(
		func() int { return len(ui.roots) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(ui.roots[i])
		},
	)
	ui.rootsList.OnSelected = func(id widget.ListItemID) {
		ui.selectedRoot = int(id)
	}
	rootsScroll := container.NewScroll(ui.rootsList)
	rootsScroll.SetMinSize(fyne.NewSize(420, 96))
	addBtn := widget.NewButton("+", ui.addRoot)
	removeBtn := widget.NewButton("-", ui.removeRoot)
	rootButtons := container.NewVBox(addBtn, removeBtn, layout.NewSpacer())
	rootListPanel := container.NewPadded(container.NewBorder(nil, nil, nil, nil, rootsScroll))
	rootSelector := container.NewBorder(nil, nil, rootButtons, nil, widget.NewCard("", "", rootListPanel))

	ui.startBtn = widget.NewButton("Start", ui.startScan)
	ui.stopBtn = widget.NewButton("Stop", ui.stopScan)
	ui.stopBtn.Disable()

	ui.duplicateBox = container.NewVBox()
	scroll := container.NewScroll(ui.duplicateBox)
	scroll.SetMinSize(fyne.NewSize(1000, 650))

	controls := container.NewVBox(
		rootSelector,
		container.NewHBox(
			ui.startBtn,
			ui.stopBtn,
			ui.revalidateBtn,
			ui.sortBtn,
			ui.removeInternal,
			ui.skipMp3M4a,
		),
	)

	statusBar := container.NewBorder(nil, nil, nil, nil,
		container.NewHBox(ui.progressBar, ui.statusLabel, layout.NewSpacer()),
	)

	content := container.NewBorder(controls, statusBar, nil, nil, scroll)
	ui.win.SetContent(content)
}

func (ui *AppUI) addRoot() {
	if runtime.GOOS == "darwin" {
		root, err := chooseFolder(ui.win)
		if err != nil || root == "" {
			return
		}
		ui.addRootPath(root)
		return
	}

	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		ui.addRootPath(uri.Path())
	}, ui.win)
}

func (ui *AppUI) addRootPath(root string) {
	for _, existing := range ui.roots {
		if existing == root {
			return
		}
	}
	ui.roots = append(ui.roots, root)
	ui.rootsList.Refresh()
}

func (ui *AppUI) removeRoot() {
	if ui.selectedRoot < 0 || ui.selectedRoot >= len(ui.roots) {
		return
	}
	ui.roots = append(ui.roots[:ui.selectedRoot], ui.roots[ui.selectedRoot+1:]...)
	ui.selectedRoot = -1
	ui.rootsList.UnselectAll()
	ui.rootsList.Refresh()
}

func (ui *AppUI) startScan() {
	ui.scanRunID++
	runID := ui.scanRunID
	ui.clearDuplicates()
	ui.sortBtn.Disable()
	ui.startBtn.Disable()
	ui.stopBtn.Enable()

	uicontinue := func() {
		ui.startBtn.Enable()
		ui.stopBtn.Disable()
	}

	ui.scanManager.Start(ui.roots, ui.removeInternal.Checked, ui.skipMp3M4a.Checked,
		func(group []string, musicDates map[string]string) {
			if runID != ui.scanRunID {
				return
			}
			ui.addDuplicateGroup(group, musicDates)
			ui.sortBtn.Enable()
		},
		func(progress float64, message string, finished bool) {
			if runID != ui.scanRunID {
				return
			}
			if !(finished && message == "Cancelled") {
				ui.progressBar.SetValue(progress)
			}
			if finished && message == "Cancelled" {
				ui.setStatus("Ready")
			} else {
				ui.setStatus(message)
			}
			if finished {
				ui.startBtn.Enable()
				ui.stopBtn.Disable()
			}
		})
	go func() {
		// Keep UI state updated in case the scan ends immediately.
		<-ui.scanManager.done()
		fyne.Do(func() {
			if runID != ui.scanRunID {
				return
			}
			uicontinue()
		})
	}()
}

func (ui *AppUI) stopScan() {
	if !ui.scanManager.IsRunning() {
		ui.stopBtn.Disable()
		ui.startBtn.Enable()
		return
	}

	// Invalidate queued callbacks from the active run so they cannot overwrite stop state.
	ui.scanRunID++
	stopRunID := ui.scanRunID

	ui.stopBtn.Disable()
	ui.startBtn.Disable()
	ui.scanManager.Cancel()
	ui.setStatus("Stopping scan...")

	go func() {
		<-ui.scanManager.done()
		fyne.Do(func() {
			if stopRunID != ui.scanRunID {
				return
			}
			ui.startBtn.Enable()
			ui.stopBtn.Disable()
			ui.setStatus("Ready")
		})
	}()
}

func (ui *AppUI) clearDuplicates() {
	ui.duplicateBox.Objects = nil
	ui.duplicateBox.Refresh()
	ui.panels = nil
}

func (ui *AppUI) addDuplicateGroup(files []string, musicDates map[string]string) {
	panel := newDuplicatePanel(files, musicDates)
	ui.panels = append(ui.panels, panel)
	ui.duplicateBox.Add(panel.container)
	ui.duplicateBox.Refresh()
}

func (ui *AppUI) sortDuplicates() {
	sort.Slice(ui.panels, func(i, j int) bool {
		return ui.panels[i].firstPath < ui.panels[j].firstPath
	})
	ui.duplicateBox.Objects = nil
	for _, panel := range ui.panels {
		ui.duplicateBox.Add(panel.container)
	}
	ui.duplicateBox.Refresh()
}

func (ui *AppUI) revalidateDuplicates() {
	validPanels := make([]*DuplicatePanel, 0, len(ui.panels))
	ui.duplicateBox.Objects = nil
	for _, panel := range ui.panels {
		if panel.isValid() {
			validPanels = append(validPanels, panel)
			ui.duplicateBox.Add(panel.container)
		}
	}
	ui.panels = validPanels
	ui.duplicateBox.Refresh()
	if len(ui.panels) == 0 {
		ui.sortBtn.Disable()
	}
}

func (ui *AppUI) setStatus(message string) {
	ui.statusLabel.SetText(message)
}

func newDuplicatePanel(files []string, musicDates map[string]string) *DuplicatePanel {
	rows := make([]fyne.CanvasObject, 0, len(files))
	for _, path := range files {
		currentPath := path
		var dateLabel *widget.Label
		parent := filepath.Dir(currentPath)
		if ts, ok := musicDates[parent]; ok {
			dateLabel = widget.NewLabel(ts)
			dateLabel.TextStyle = fyne.TextStyle{Bold: true}
		} else if info, err := os.Stat(currentPath); err == nil {
			dateLabel = widget.NewLabel(info.ModTime().Format(time.RFC3339))
			dateLabel.TextStyle = fyne.TextStyle{Bold: true}
		}

		pathLabel := widget.NewLabel(currentPath)
		pathLabel.Selectable = true
		pathLabel.Alignment = fyne.TextAlignLeading
		pathLabel.Wrapping = fyne.TextTruncate
		pathLabel.Truncation = fyne.TextTruncateClip

		openBtn := widget.NewButton("Open", func() {
			openFileDirectory(currentPath)
		})

		if dateLabel != nil {
			row := container.NewBorder(nil, nil, nil,
				container.NewHBox(layout.NewSpacer(), dateLabel, openBtn),
				pathLabel,
			)
			rows = append(rows, row)
		} else {
			row := container.NewBorder(nil, nil, nil,
				container.NewHBox(layout.NewSpacer(), openBtn),
				pathLabel,
			)
			rows = append(rows, row)
		}
	}

	groupContainer := container.NewVBox(rows...)
	card := widget.NewCard("", "", groupContainer)
	return &DuplicatePanel{
		container: container.NewVBox(card),
		firstPath: files[0],
		filePaths: files,
	}
}

func (p *DuplicatePanel) isValid() bool {
	for _, path := range p.filePaths {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func openFileDirectory(path string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", "-R", path).Start()
	case "windows":
		exec.Command("explorer", "/select,", path).Start()
	default:
		exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

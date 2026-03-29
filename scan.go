package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
)

type hashResult struct {
	sum [32]byte
	err error
}

type ScanCallback func(group []string, musicDates map[string]string)
type ScanProgress func(progress float64, message string, finished bool)

type ScanManager struct {
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	running bool
	doneCh  chan struct{}
	cancelRequested atomic.Bool
}

func (m *ScanManager) Start(roots []string, removeInternal, skipMp3 bool, onGroup ScanCallback, onProgress ScanProgress) {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	m.running = true
	m.doneCh = make(chan struct{})
	m.cancelRequested.Store(false)
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.running = false
			close(m.doneCh)
			m.mu.Unlock()
		}()

		lastProgressAt := time.Time{}
		isStopRequested := func() bool {
			return m.cancelRequested.Load() || isCancelled(ctx)
		}
		emitProgress := func(progress float64, message string, finished bool, force bool) {
			if !force && isStopRequested() {
				return
			}
			now := time.Now()
			if !force && !finished && !lastProgressAt.IsZero() && now.Sub(lastProgressAt) < 80*time.Millisecond {
				return
			}
			lastProgressAt = now
			runOnMain(func() { onProgress(progress, message, finished) })
		}

		if len(roots) == 0 {
			emitProgress(0, "Add at least one root before starting", true, true)
			return
		}

		bySize := map[int64][]string{}
		musicDates := map[string]string{}
		groupCount := 0
		scannedFiles := 0
		lastScanStatusAt := time.Time{}

		for _, root := range roots {
			if isStopRequested() {
				emitProgress(0, "Cancelled", true, true)
				return
			}
			emitProgress(0, fmt.Sprintf("Scanning %s", root), false, true)
			filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					fmt.Println(err)
					return nil
				}
				if isCancelled(ctx) {
					return fs.SkipDir
				}
				if d.IsDir() {
					if removeInternal {
						_ = removeInternalDuplicates(ctx, path, skipMp3, musicDates)
					}
					return nil
				}

				scannedFiles++
				now := time.Now()
				if lastScanStatusAt.IsZero() || now.Sub(lastScanStatusAt) > 120*time.Millisecond {
					lastScanStatusAt = now
					emitProgress(0, fmt.Sprintf("Scanning %s", path), false, false)
				}

				info, err := d.Info()
				if err != nil {
					fmt.Println(err)
					return nil
				}
				if !info.Mode().IsRegular() {
					return nil
				}
				if !checkExtension(path, info, skipMp3, musicDates) {
					return nil
				}
				if info.Size() == 0 {
					_ = os.Remove(path)
					return nil
				}
				bySize[info.Size()] = append(bySize[info.Size()], path)
				return nil
			})

			emitProgress(0, fmt.Sprintf("Scanned %d files in %s", scannedFiles, root), false, false)
		}

		for _, files := range bySize {
			if len(files) > 1 {
				groupCount++
			}
		}

		completed := 0
		for _, files := range bySize {
			if isStopRequested() {
				emitProgress(0, "Cancelled", true, true)
				return
			}
			if len(files) <= 1 {
				continue
			}
			completed++
			contentMap := map[[32]byte][]string{}
			for fileIdx, path := range files {
				if isStopRequested() {
					emitProgress(0, "Cancelled", true, true)
					return
				}
				// Show progress as group progress + per-file progress within group
				fileProgress := float64(fileIdx) / float64(len(files))
				groupProgress := float64(completed-1) / float64(groupCount)
				overallProgress := (groupProgress + fileProgress/float64(groupCount))
				emitProgress(overallProgress, fmt.Sprintf("Comparing %s", filepath.Base(path)), false, false)
				hash, err := hashFileWithCancel(ctx, path)
				if err != nil {
					if err == context.Canceled {
						emitProgress(0, "Cancelled", true, true)
						return
					}
					fmt.Println(err)
					continue
				}
				contentMap[hash] = append(contentMap[hash], path)
			}
			for _, group := range contentMap {
				if len(group) <= 1 {
					continue
				}
				sort.Strings(group)
				groupForUI := append([]string(nil), group...)
				runOnMain(func() {
					onGroup(groupForUI, musicDates)
				})
			}
		}

		emitProgress(1.0, "Finished", true, true)
	}()
}

func (m *ScanManager) Cancel() {
	m.cancelRequested.Store(true)
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
}

func (m *ScanManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *ScanManager) done() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doneCh
}

func runOnMain(fn func()) {
	fyne.Do(fn)
}

func isCancelled(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}


// hashFileWithCancel computes a file hash in chunks so cancellation can interrupt compare quickly.
func hashFileWithCancel(ctx context.Context, path string) ([32]byte, error) {
	var zero [32]byte
	if isCancelled(ctx) {
		return zero, context.Canceled
	}

	file, err := os.Open(path)
	if err != nil {
		return zero, err
	}

	resultCh := make(chan hashResult, 1)
	go func() {
		defer file.Close()
		h := sha256.New()
		const chunkSize = 1024 * 1024
		chunk := make([]byte, chunkSize)

		for {
			n, readErr := file.Read(chunk)
			if n > 0 {
				if _, writeErr := h.Write(chunk[:n]); writeErr != nil {
					resultCh <- hashResult{err: writeErr}
					return
				}
			}
			if readErr != nil && readErr != io.EOF {
				resultCh <- hashResult{err: readErr}
				return
			}
			if readErr == io.EOF {
				break
			}
		}

		sum := h.Sum(nil)
		var out [32]byte
		copy(out[:], sum)
		resultCh <- hashResult{sum: out}
	}()

	select {
	case <-ctx.Done():
		// Closing the file interrupts blocking reads so hashing can stop promptly.
		_ = file.Close()
		return zero, context.Canceled
	case res := <-resultCh:
		return res.sum, res.err
	}
}

func checkExtension(path string, info fs.FileInfo, skipMp3 bool, musicDates map[string]string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".iso") || strings.HasSuffix(lower, ".nrg") {
		return false
	}
	if strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".m4a") {
		parent := filepath.Dir(path)
		existing, ok := musicDates[parent]
		if !ok || info.ModTime().After(parseTime(existing)) {
			musicDates[parent] = info.ModTime().Format(time.RFC3339)
		}
		if skipMp3 {
			return false
		}
	}
	skipExtensions := []string{".swa", "ds_store", ".exe", ".ico"}
	for _, ext := range skipExtensions {
		if strings.HasSuffix(lower, ext) {
			return false
		}
	}
	return true
}

func removeInternalDuplicates(ctx context.Context, dir string, skipMp3 bool, musicDates map[string]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	bySize := map[int64][]string{}
	for _, entry := range entries {
		if isCancelled(ctx) {
			return nil
		}
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if !checkExtension(path, info, skipMp3, musicDates) {
			continue
		}
		if info.Size() == 0 {
			_ = os.Remove(path)
			continue
		}
		bySize[info.Size()] = append(bySize[info.Size()], path)
	}

	for _, files := range bySize {
		if len(files) <= 1 {
			continue
		}
		sort.Slice(files, func(i, j int) bool {
			if len(files[i]) != len(files[j]) {
				return len(files[i]) < len(files[j])
			}
			return files[i] < files[j]
		})

		contentMap := map[[32]byte][]string{}
		for _, path := range files {
			if isCancelled(ctx) {
				return nil
			}
			hash, err := hashFileWithCancel(ctx, path)
			if err != nil {
				if err == context.Canceled {
					return nil
				}
				continue
			}
			contentMap[hash] = append(contentMap[hash], path)
		}
		for _, group := range contentMap {
			if len(group) <= 1 {
				continue
			}
			sort.Slice(group, func(i, j int) bool {
				if len(group[i]) != len(group[j]) {
					return len(group[i]) < len(group[j])
				}
				return group[i] < group[j]
			})
			for i := 1; i < len(group); i++ {
				if isCancelled(ctx) {
					return nil
				}
				_ = os.Remove(group[i])
			}
		}
	}
	return nil
}

func parseTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

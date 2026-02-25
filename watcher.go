package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a change detected in a JSONL file.
type FileEvent struct {
	Path    string
	NewData []byte
}

// Watcher monitors JSONL files under the Claude projects directory.
type Watcher struct {
	dir      string
	debounce time.Duration
	fileCh   chan FileEvent
	offsets  map[string]int64
	mu       sync.Mutex
	timers   map[string]*time.Timer
}

func NewWatcher(dir string, debounce time.Duration) *Watcher {
	return &Watcher{
		dir:      dir,
		debounce: debounce,
		fileCh:   make(chan FileEvent, 16),
		offsets:  make(map[string]int64),
		timers:   make(map[string]*time.Timer),
	}
}

// Events returns the channel on which file events are delivered.
func (w *Watcher) Events() <-chan FileEvent {
	return w.fileCh
}

// Run starts watching. It blocks until ctx is done or an unrecoverable error occurs.
func (w *Watcher) Run(done <-chan struct{}) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// Walk existing subdirectories and add them.
	if err := w.addDirs(fsw, w.dir); err != nil {
		log.Printf("warning: could not walk %s: %v", w.dir, err)
	}

	for {
		select {
		case <-done:
			return nil
		case ev, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			// If a new directory is created, add it to the watcher.
			if ev.Has(fsnotify.Create) {
				if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
					_ = w.addDirs(fsw, ev.Name)
				}
			}
			if ev.Has(fsnotify.Write) && strings.HasSuffix(ev.Name, ".jsonl") {
				w.scheduleRead(ev.Name)
			}
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

func (w *Watcher) addDirs(fsw *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if info.IsDir() {
			if addErr := fsw.Add(path); addErr != nil {
				log.Printf("warning: cannot watch %s: %v", path, addErr)
			}
		}
		return nil
	})
}

func (w *Watcher) scheduleRead(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if t, ok := w.timers[path]; ok {
		t.Stop()
	}
	w.timers[path] = time.AfterFunc(w.debounce, func() {
		w.readNewData(path)
	})
}

func (w *Watcher) readNewData(path string) {
	w.mu.Lock()
	offset := w.offsets[path]
	w.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		log.Printf("cannot open %s: %v", path, err)
		return
	}
	defer f.Close()

	// Seek to the last known offset.
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			log.Printf("seek error for %s: %v", path, err)
			return
		}
	}

	data, err := io.ReadAll(f)
	if err != nil {
		log.Printf("read error for %s: %v", path, err)
		return
	}

	if len(data) == 0 {
		return
	}

	newOffset := offset + int64(len(data))
	w.mu.Lock()
	w.offsets[path] = newOffset
	w.mu.Unlock()

	w.fileCh <- FileEvent{Path: path, NewData: data}
}

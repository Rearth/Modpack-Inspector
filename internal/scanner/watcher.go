package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// WatchEvent describes a file change in the mods or config folder.
type WatchEvent struct {
	Type string // "mod_added", "mod_removed", "mod_changed", "config_changed"
	Path string
	Name string
}

// Watcher monitors mods and config folders for changes.
type Watcher struct {
	watcher *fsnotify.Watcher
	Events  chan WatchEvent
	Errors  chan error
	done    chan struct{}
}

// NewWatcher creates a filesystem watcher for the given directories.
func NewWatcher(modsDir, configDir string) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher: fw,
		Events:  make(chan WatchEvent, 100),
		Errors:  make(chan error, 10),
		done:    make(chan struct{}),
	}

	if modsDir != "" {
		if err := fw.Add(modsDir); err != nil {
			fw.Close()
			return nil, err
		}
	}
	if configDir != "" {
		if err := addDirRecursive(fw, configDir); err != nil {
			fw.Close()
			return nil, err
		}
	}

	go w.loop(modsDir, configDir)
	return w, nil
}

func (w *Watcher) Close() error {
	close(w.done)
	return w.watcher.Close()
}

func (w *Watcher) loop(modsDir, configDir string) {
	for {
		select {
		case <-w.done:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			we := classifyEvent(event, modsDir, configDir)
			if we != nil {
				w.Events <- *we
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.Errors <- err
		}
	}
}

func classifyEvent(event fsnotify.Event, modsDir, configDir string) *WatchEvent {
	name := filepath.Base(event.Name)

	// Check if it's in the mods directory
	if modsDir != "" && isUnder(event.Name, modsDir) {
		if !strings.HasSuffix(strings.ToLower(name), ".jar") {
			return nil
		}
		switch {
		case event.Op&fsnotify.Create != 0:
			return &WatchEvent{Type: "mod_added", Path: event.Name, Name: name}
		case event.Op&fsnotify.Remove != 0 || event.Op&fsnotify.Rename != 0:
			return &WatchEvent{Type: "mod_removed", Path: event.Name, Name: name}
		case event.Op&fsnotify.Write != 0:
			return &WatchEvent{Type: "mod_changed", Path: event.Name, Name: name}
		}
	}

	// Check if it's in the config directory
	if configDir != "" && isUnder(event.Name, configDir) {
		if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
			return &WatchEvent{Type: "config_changed", Path: event.Name, Name: name}
		}
	}

	return nil
}

func isUnder(path, dir string) bool {
	absPath, _ := filepath.Abs(path)
	absDir, _ := filepath.Abs(dir)
	return strings.HasPrefix(strings.ToLower(absPath), strings.ToLower(absDir))
}

func addDirRecursive(fw *fsnotify.Watcher, dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return fw.Add(path)
		}
		return nil
	})
}

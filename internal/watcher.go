package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FileChange struct {
	Path      string
	Op        string
	Timestamp time.Time
}

type WatchConfig struct {
	Paths     []string
	Extensions []string
	Ignore    []string
	Debounce  time.Duration
	OnChange  func(FileChange)
}

type Watcher struct {
	Config    WatchConfig
	Running   bool
	Changes   []FileChange
	stopChan  chan struct{}
}

func NewWatcher(config WatchConfig) *Watcher {
	if config.Debounce == 0 {
		config.Debounce = 500 * time.Millisecond
	}

	if len(config.Ignore) == 0 {
		config.Ignore = []string{".git", "node_modules", "vendor", ".cache"}
	}

	return &Watcher{
		Config:   config,
		Changes:  make([]FileChange, 0),
		stopChan: make(chan struct{}),
	}
}

func (w *Watcher) Start() error {
	w.Running = true

	go w.watch()

	return nil
}

func (w *Watcher) Stop() {
	w.Running = false
	close(w.stopChan)
}

func (w *Watcher) watch() {
	snapshots := make(map[string]time.Time)

	for _, path := range w.Config.Paths {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			if w.shouldIgnore(p) {
				return nil
			}

			if w.matchesExtension(p) {
				snapshots[p] = info.ModTime()
			}

			return nil
		})
	}

	for w.Running {
		time.Sleep(w.Config.Debounce)

		for _, path := range w.Config.Paths {
			filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}

				if w.shouldIgnore(p) || !w.matchesExtension(p) {
					return nil
				}

				modTime := info.ModTime()
				if lastMod, ok := snapshots[p]; !ok {
					change := FileChange{
						Path:      p,
						Op:        "created",
						Timestamp: time.Now(),
					}
					w.Changes = append(w.Changes, change)
					if w.Config.OnChange != nil {
						w.Config.OnChange(change)
					}
				} else if modTime.After(lastMod) {
					change := FileChange{
						Path:      p,
						Op:        "modified",
						Timestamp: time.Now(),
					}
					w.Changes = append(w.Changes, change)
					if w.Config.OnChange != nil {
						w.Config.OnChange(change)
					}
				}

				snapshots[p] = modTime
				return nil
			})
		}
	}
}

func (w *Watcher) shouldIgnore(path string) bool {
	parts := strings.Split(path, string(os.PathSeparator))
	for _, part := range parts {
		for _, ignore := range w.Config.Ignore {
			if matched, _ := filepath.Match(ignore, part); matched {
				return true
			}
		}
	}
	return false
}

func (w *Watcher) matchesExtension(path string) bool {
	if len(w.Config.Extensions) == 0 {
		return true
	}

	ext := filepath.Ext(path)
	for _, e := range w.Config.Extensions {
		if ext == e {
			return true
		}
	}

	return false
}

func (w *Watcher) GetChanges() []FileChange {
	return w.Changes
}

func (w *Watcher) ClearChanges() {
	w.Changes = make([]FileChange, 0)
}

func (w *Watcher) RenderStatus() string {
	if !w.Running {
		return "\033[1;31m⏹ Watch stopped\033[0m"
	}

	paths := strings.Join(w.Config.Paths, ", ")
	if len(paths) > 40 {
		paths = paths[:37] + "..."
	}

	return fmt.Sprintf("\033[1;32m▶ Watching\033[0m: %s (%d changes)", paths, len(w.Changes))
}

func (w *Watcher) RenderChanges() string {
	if len(w.Changes) == 0 {
		return "No changes detected."
	}

	var out strings.Builder
	out.WriteString("\033[1;35mFile Changes:\033[0m\n\n")

	for _, c := range w.Changes {
		color := "33"
		if c.Op == "created" {
			color = "32"
		}

		out.WriteString(fmt.Sprintf("  \033[1;%sm%s\033[0m %s\n", color, c.Op, c.Path))
	}

	return out.String()
}

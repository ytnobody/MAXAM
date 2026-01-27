package comms

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Event represents a communication event
type Event struct {
	From     string
	To       string
	FilePath string
}

// Watcher monitors the comms directory for new messages
type Watcher struct {
	dir     string
	watcher *fsnotify.Watcher
	Events  chan Event
	Errors  chan error
}

// NewWatcher creates a new file watcher
func NewWatcher(dir string) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, fmt.Errorf("watch dir: %w", err)
	}

	return &Watcher{
		dir:     dir,
		watcher: w,
		Events:  make(chan Event, 10),
		Errors:  make(chan error, 10),
	}, nil
}

// Start begins watching for file changes
func (w *Watcher) Start() {
	go func() {
		for {
			select {
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				// Only handle write events on .md files
				if event.Op&fsnotify.Write != fsnotify.Write {
					continue
				}

				if !strings.HasSuffix(event.Name, ".md") {
					continue
				}

				// Parse filename to get from/to
				base := filepath.Base(event.Name)
				base = strings.TrimSuffix(base, ".md")
				parts := strings.Split(base, "_to_")

				if len(parts) != 2 {
					continue
				}

				w.Events <- Event{
					From:     parts[0],
					To:       parts[1],
					FilePath: event.Name,
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				w.Errors <- err
			}
		}
	}()
}

// Stop closes the watcher
func (w *Watcher) Stop() error {
	close(w.Events)
	close(w.Errors)
	return w.watcher.Close()
}

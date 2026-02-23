// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package credentialsfile // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// fileWatcher implements ValueResolver by watching a file for changes
// and caching its contents atomically.
type fileWatcher struct {
	path       string
	value      atomic.Pointer[string]
	logger     *zap.Logger
	onChange   func(string)
	shutdownCH chan struct{}
	doneCH     chan struct{}
}

func newFileWatcher(path string, logger *zap.Logger, onChange func(string)) *fileWatcher {
	return &fileWatcher{
		path:     path,
		logger:   logger,
		onChange: onChange,
	}
}

func (w *fileWatcher) Value() string {
	if v := w.value.Load(); v != nil {
		return *v
	}
	return ""
}

func (w *fileWatcher) Start(ctx context.Context) error {
	if err := w.reload(); err != nil {
		return fmt.Errorf("failed to read credentials file %q: %w", w.path, err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	w.shutdownCH = make(chan struct{})
	w.doneCH = make(chan struct{})
	go w.watch(ctx, watcher)

	// Watch the file itself. For symlinked files (e.g. Kubernetes projected volumes),
	// fsnotify follows the symlink and watches the underlying inode. On Remove/Chmod
	// events the watcher is re-added to follow the new symlink target.
	return watcher.Add(w.path)
}

func (w *fileWatcher) Shutdown() error {
	if w.shutdownCH != nil {
		close(w.shutdownCH)
		<-w.doneCH
		w.shutdownCH = nil
	}
	return nil
}

func (w *fileWatcher) watch(ctx context.Context, watcher *fsnotify.Watcher) {
	defer close(w.doneCH)
	defer watcher.Close()
	for {
		select {
		case <-w.shutdownCH:
			return
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// NOTE: Kubernetes projected volumes use symlinks. When the backing
			// symlink target is removed, fsnotify may auto-remove the watch.
			// We must re-add it so it follows the new symlink target.
			// See: https://martensson.io/go-fsnotify-and-kubernetes-configmaps/
			if event.Op&(fsnotify.Remove|fsnotify.Chmod) != 0 {
				if err := watcher.Remove(event.Name); err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
					w.logger.Warn("failed to remove watcher", zap.Error(err))
				}
				if err := watcher.Add(w.path); err != nil {
					w.logger.Error("failed to re-add watcher", zap.Error(err))
				}
				w.reloadQuietly()
			}
			if event.Op&fsnotify.Write != 0 {
				w.reloadQuietly()
			}
		}
	}
}

func (w *fileWatcher) reloadQuietly() {
	if err := w.reload(); err != nil {
		w.logger.Warn("failed to reload credentials file, keeping last value",
			zap.String("file", w.path), zap.Error(err))
	}
}

func (w *fileWatcher) reload() error {
	data, err := os.ReadFile(w.path)
	if err != nil {
		return err
	}
	val := strings.TrimSpace(string(data))
	if val == "" {
		return fmt.Errorf("credentials file %q is empty", w.path)
	}
	w.value.Store(&val)
	if w.onChange != nil {
		w.onChange(val)
	}
	return nil
}

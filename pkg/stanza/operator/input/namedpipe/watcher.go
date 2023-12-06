// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package namedpipe // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"

import (
	"context"
	"fmt"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches a file for writes, notifying via `C` when one is observed.
type Watcher struct {
	C       chan struct{}
	watcher *fsnotify.Watcher
}

// NewWatcher creates a new watcher for the given path.
func NewWatcher(path string) (*Watcher, error) {
	watcher, err := newWatcher(path)
	if err != nil {
		return nil, err
	}

	return &Watcher{
		C:       make(chan struct{}),
		watcher: watcher,
	}, nil
}

// Watch starts the watcher, sending a message to `C` when a write is observed.
func (w *Watcher) Watch(ctx context.Context) error {
	defer func() {
		w.watcher.Close()
		close(w.C)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-w.watcher.Events:
			if event.Op&fsnotify.Write == fsnotify.Write {
				select {
				case w.C <- struct{}{}:
				case <-ctx.Done():
					return nil
				}
			}
		case err := <-w.watcher.Errors:
			return fmt.Errorf("watcher error: %w", err)
		}
	}
}

func newWatcher(path string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := watcher.Add(path); err != nil {
		return nil, fmt.Errorf("failed to add path to watcher: %w", err)
	}

	return watcher, nil
}

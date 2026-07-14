// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// ParseFunc turns raw file content into a lookup table. A value is either a
// scalar (for 1:1 lookups) or a map[string]any (for 1:N lookups).
type ParseFunc func(content []byte) (map[string]any, error)

// ReloadCallback runs after each periodic reload. success says whether the
// reload worked. It does not run for the first load done by Start.
type ReloadCallback func(ctx context.Context, success bool)

// FileLookupSettings configures a FileLookup.
type FileLookupSettings struct {
	Path           string
	ReloadInterval time.Duration
	Parse          ParseFunc
	Logger         *zap.Logger
	OnReload       ReloadCallback
}

// FileLookup is a lookup source backed by a file. With ReloadInterval > 0 it
// re-reads the file on that interval so changes apply without a restart.
type FileLookup struct {
	path           string
	reloadInterval time.Duration
	parse          ParseFunc
	logger         *zap.Logger
	onReload       ReloadCallback

	mu   sync.RWMutex
	data map[string]any

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewFileLookup creates a FileLookup. Call Start to load the file and begin reloading.
func NewFileLookup(set FileLookupSettings) *FileLookup {
	logger := set.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FileLookup{
		path:           set.Path,
		reloadInterval: set.ReloadInterval,
		parse:          set.Parse,
		logger:         logger,
		onReload:       set.OnReload,
		data:           map[string]any{},
	}
}

func (f *FileLookup) load() error {
	content, err := os.ReadFile(f.path)
	if err != nil {
		return fmt.Errorf("failed to read file %q: %w", f.path, err)
	}
	data, err := f.parse(content)
	if err != nil {
		return fmt.Errorf("failed to parse file %q: %w", f.path, err)
	}
	if data == nil {
		data = map[string]any{}
	}

	f.mu.Lock()
	f.data = data
	f.mu.Unlock()
	return nil
}

// Start loads the file once and, if ReloadInterval > 0, starts the reload loop.
func (f *FileLookup) Start(_ context.Context, _ component.Host) error {
	if err := f.load(); err != nil {
		return err
	}
	if f.reloadInterval <= 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel
	f.wg.Add(1)
	go f.reloadLoop(ctx)
	return nil
}

func (f *FileLookup) reloadLoop(ctx context.Context) {
	defer f.wg.Done()

	ticker := time.NewTicker(f.reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := f.load()
			if err != nil {
				f.logger.Warn("failed to reload lookup file; keeping previously loaded data",
					zap.String("path", f.path), zap.Error(err))
			}
			if f.onReload != nil {
				f.onReload(ctx, err == nil)
			}
		}
	}
}

// Lookup returns the value stored for key, if present.
func (f *FileLookup) Lookup(_ context.Context, key string) (any, bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	val, found := f.data[key]
	return val, found, nil
}

// Shutdown stops the reload loop and waits for it to exit.
func (f *FileLookup) Shutdown(_ context.Context) error {
	if f.cancel != nil {
		f.cancel()
	}
	f.wg.Wait()
	return nil
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package yaml provides a YAML file-based lookup source.
package yaml // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/source/yaml"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

const sourceType = "yaml"

// Config is the configuration for the YAML lookup source.
type Config struct {
	// Path is the path to the YAML file containing key-value mappings.
	// The file should contain a flat map of string keys to values.
	// Required.
	Path string `mapstructure:"path"`

	// Watch enables file-watch-based reloading. When true, the source
	// watches the file for changes and reloads its contents in the
	// background. Lookups continue to serve the last successfully loaded
	// data if a reload fails.
	// Optional. Default: false.
	Watch bool `mapstructure:"watch"`
}

// Validate implements lookupsource.SourceConfig.
func (c *Config) Validate() error {
	if c.Path == "" {
		return errors.New("path is required")
	}
	return nil
}

// NewFactory creates a factory for the YAML source.
func NewFactory() lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		sourceType,
		createDefaultConfig,
		createSource,
	)
}

func createDefaultConfig() lookupsource.SourceConfig {
	return &Config{}
}

func createSource(
	_ context.Context,
	settings lookupsource.CreateSettings,
	cfg lookupsource.SourceConfig,
) (lookupsource.Source, error) {
	yamlCfg := cfg.(*Config)

	logger := zap.NewNop()
	if settings.TelemetrySettings.Logger != nil {
		logger = settings.TelemetrySettings.Logger
	}

	s := &yamlSource{
		path:   yamlCfg.Path,
		watch:  yamlCfg.Watch,
		logger: logger,
		data:   make(map[string]any),
	}

	return lookupsource.NewSource(
		s.lookup,
		func() string { return sourceType },
		s.start,
		s.shutdown,
	), nil
}

// yamlSource holds the loaded YAML data.
//
// The file is read once during Start. When Watch is enabled, a background
// goroutine watches the file for changes and atomically swaps the data map
// while concurrent lookups are in progress, guarded by the RWMutex.
type yamlSource struct {
	path   string
	watch  bool
	logger *zap.Logger

	mu   sync.RWMutex
	data map[string]any

	watcher    *fsnotify.Watcher
	shutdownCH chan struct{}
	doneCH     chan struct{}
}

// start loads the YAML file and, if configured, begins watching it for changes.
func (s *yamlSource) start(_ context.Context, _ component.Host) error {
	if err := s.reload(); err != nil {
		return err
	}

	if !s.watch {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher for %q: %w", s.path, err)
	}

	if err := watcher.Add(s.path); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("failed to watch YAML file %q: %w", s.path, err)
	}

	s.watcher = watcher
	s.shutdownCH = make(chan struct{})
	s.doneCH = make(chan struct{})
	go s.watchLoop()

	return nil
}

// shutdown stops the file watcher, if running.
func (s *yamlSource) shutdown(_ context.Context) error {
	if s.shutdownCH != nil {
		close(s.shutdownCH)
		<-s.doneCH
		s.shutdownCH = nil
	}
	return nil
}

// watchLoop reacts to filesystem events and reloads the file on change.
func (s *yamlSource) watchLoop() {
	defer close(s.doneCH)
	defer func() { _ = s.watcher.Close() }()

	for {
		select {
		case <-s.shutdownCH:
			return
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			// Kubernetes projected volumes (e.g. ConfigMaps) use symlinks.
			// When the backing symlink target is replaced, fsnotify may
			// auto-remove the watch, so re-add it to follow the new target.
			// See: https://martensson.io/go-fsnotify-and-kubernetes-configmaps/
			if event.Op&(fsnotify.Remove|fsnotify.Chmod) != 0 {
				if err := s.watcher.Remove(event.Name); err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
					s.logger.Warn("failed to remove watcher", zap.String("file", s.path), zap.Error(err))
				}
				if err := s.watcher.Add(s.path); err != nil {
					s.logger.Error("failed to re-add watcher", zap.String("file", s.path), zap.Error(err))
				}
				s.reloadQuietly()
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				s.reloadQuietly()
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.logger.Warn("file watcher error", zap.String("file", s.path), zap.Error(err))
		}
	}
}

// reloadQuietly reloads the file, logging and keeping the last value on failure.
func (s *yamlSource) reloadQuietly() {
	if err := s.reload(); err != nil {
		s.logger.Warn("failed to reload YAML file, keeping last loaded data",
			zap.String("file", s.path), zap.Error(err))
		return
	}
	s.logger.Debug("reloaded YAML lookup file", zap.String("file", s.path))
}

// reload reads and parses the YAML file, atomically swapping the data map.
func (s *yamlSource) reload() error {
	content, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("failed to read YAML file %q: %w", s.path, err)
	}

	var data map[string]any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("failed to parse YAML file %q: %w", s.path, err)
	}

	s.mu.Lock()
	s.data = data
	s.mu.Unlock()

	return nil
}

// lookup retrieves a value from the loaded YAML data.
func (s *yamlSource) lookup(_ context.Context, key string) (any, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, found := s.data[key]
	return val, found, nil
}

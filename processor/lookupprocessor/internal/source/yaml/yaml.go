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

	"go.opentelemetry.io/collector/component"
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
	_ lookupsource.CreateSettings,
	cfg lookupsource.SourceConfig,
) (lookupsource.Source, error) {
	yamlCfg := cfg.(*Config)
	s := &yamlSource{
		path: yamlCfg.Path,
		data: make(map[string]any),
	}

	return lookupsource.NewSource(
		s.lookup,
		func() string { return sourceType },
		s.start,
		nil, // no shutdown needed
	), nil
}

// yamlSource holds the loaded YAML data.
//
// The file is read once during Start and never reloaded. The RWMutex is
// present to allow a future file-watch/reload mechanism to swap the data
// map safely while concurrent lookups are in progress.
// TODO: support periodic or file-watch-based reload.
type yamlSource struct {
	path string
	mu   sync.RWMutex
	data map[string]any
}

// start loads the YAML file.
func (s *yamlSource) start(_ context.Context, _ component.Host) error {
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

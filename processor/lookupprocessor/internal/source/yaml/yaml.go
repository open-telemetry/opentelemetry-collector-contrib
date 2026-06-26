// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package yaml provides a YAML file-based lookup source.
package yaml // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/source/yaml"

import (
	"context"
	"errors"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

const sourceType = "yaml"

// Config is the configuration for the YAML lookup source.
type Config struct {
	// Path is the path to the YAML file containing key-value mappings.
	// The file should contain a flat map of string keys to values.
	// Required.
	Path string `mapstructure:"path"`

	// ReloadInterval, when > 0, re-reads the file on this interval so changes
	// take effect without a collector restart. 0 (default) disables reloading.
	ReloadInterval time.Duration `mapstructure:"reload_interval"`
}

// Validate implements lookupsource.SourceConfig.
func (c *Config) Validate() error {
	if c.Path == "" {
		return errors.New("path is required")
	}
	if c.ReloadInterval < 0 {
		return errors.New("reload_interval must not be negative")
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

func parse(content []byte) (map[string]any, error) {
	var data map[string]any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func createSource(
	_ context.Context,
	settings lookupsource.CreateSettings,
	cfg lookupsource.SourceConfig,
) (lookupsource.Source, error) {
	yamlCfg := cfg.(*Config)

	reloadMetrics, err := lookupsource.NewReloadMetrics(settings.TelemetrySettings, metadata.ScopeName)
	if err != nil {
		return nil, err
	}

	fl := lookupsource.NewFileLookup(lookupsource.FileLookupSettings{
		Path:           yamlCfg.Path,
		ReloadInterval: yamlCfg.ReloadInterval,
		Parse:          parse,
		Logger:         settings.TelemetrySettings.Logger,
		OnReload:       reloadMetrics.Record,
	})

	return lookupsource.NewSource(
		fl.Lookup,
		func() string { return sourceType },
		fl.Start,
		fl.Shutdown,
	), nil
}

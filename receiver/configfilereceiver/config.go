// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/configfilereceiver"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/configfilereceiver/internal/configfile"
)

// Config configures the configfile logs receiver.
type Config struct {
	StatePath      string                   `mapstructure:"state_path"`
	PollInterval   time.Duration            `mapstructure:"poll_interval"`
	MaxKeysPerFile int                      `mapstructure:"max_keys_per_file"`
	ExcludeKeys    []string                 `mapstructure:"exclude_keys"`
	Files          []configfile.FileEntry   `mapstructure:"files"`
}

func createDefaultConfig() component.Config {
	return &Config{
		StatePath:      configfile.DefaultStatePath,
		PollInterval:   60 * time.Second,
		MaxKeysPerFile: 500,
		ExcludeKeys:    configfile.DefaultExcludeKeyGlobs,
	}
}

func (c *Config) pollerConfig() configfile.PollerConfig {
	maxKeys := c.MaxKeysPerFile
	if maxKeys <= 0 {
		maxKeys = 500
	}
	pollSeconds := int(c.PollInterval.Seconds())
	if pollSeconds <= 0 {
		pollSeconds = 60
	}
	excludeKeys := c.ExcludeKeys
	if len(excludeKeys) == 0 {
		excludeKeys = configfile.DefaultExcludeKeyGlobs
	}
	return configfile.PollerConfig{
		Files:              c.Files,
		ExcludeKeys:        excludeKeys,
		MaxKeysPerFile:     maxKeys,
		PollIntervalSecond: pollSeconds,
		StatePath:          c.StatePath,
	}
}

func (c *Config) Validate() error {
	if c.StatePath == "" {
		return errors.New("state_path is required")
	}
	if c.PollInterval <= 0 {
		return errors.New("poll_interval must be positive")
	}
	if len(c.Files) == 0 {
		return errors.New("files must not be empty")
	}
	for i, f := range c.Files {
		if f.Path == "" {
			return fmt.Errorf("files[%d].path is required", i)
		}
	}
	return nil
}

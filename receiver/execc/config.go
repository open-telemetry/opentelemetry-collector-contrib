// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package execc // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/execc"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/confmap"
)

// ParserConfig controls how each line of command output becomes a log record.
type ParserConfig struct {
	// Type is one of "line" (default) or "json".
	Type string `mapstructure:"type"`
	// BodyKey is the JSON key promoted to the log body (json parser only).
	BodyKey string `mapstructure:"body_key"`
	// Layout is a Go time layout parsed from the "timestamp" JSON field
	// (json parser only). Empty means use the receive time.
	Layout string `mapstructure:"layout"`
}

// ScheduleConfig controls how often the command runs.
type ScheduleConfig struct {
	// Interval between runs. Required.
	Interval time.Duration `mapstructure:"interval"`
	// InitialDelay before the first run. Defaults to Interval/2.
	InitialDelay time.Duration `mapstructure:"initial_delay"`
}

type Config struct {
	Schedule   ScheduleConfig         `mapstructure:"schedule"`
	Command    string                 `mapstructure:"command"`
	Args       []string               `mapstructure:"args"`
	Env        map[string]string      `mapstructure:"env"`
	WorkingDir string                 `mapstructure:"working_dir"`
	Timeout    time.Duration          `mapstructure:"timeout"`
	Parser     ParserConfig           `mapstructure:"parser"`
	Attributes map[string]interface{} `mapstructure:"attributes"`
}

var _ confmap.Validator = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.Command == "" {
		return fmt.Errorf("command must be set")
	}
	if cfg.Schedule.Interval <= 0 {
		return fmt.Errorf("schedule.interval must be positive")
	}
	switch cfg.Parser.Type {
	case "":
		cfg.Parser.Type = "line"
	case "line", "json":
	default:
		return fmt.Errorf("parser.type %q is not one of: line, json", cfg.Parser.Type)
	}
	if cfg.Timeout < 0 {
		return fmt.Errorf("timeout must not be negative")
	}
	return nil
}

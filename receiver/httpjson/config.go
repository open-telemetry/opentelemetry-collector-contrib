// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpjson // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpjson"

import (
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/confmap"
)

type BasicAuth struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type Config struct {
	Endpoint string            `mapstructure:"endpoint"`
	Method   string            `mapstructure:"method"`
	Headers  map[string]string `mapstructure:"headers"`
	Query    map[string]string `mapstructure:"query"`
	Body     string            `mapstructure:"body"`

	BearerToken string    `mapstructure:"bearer_token"`
	BasicAuth   BasicAuth `mapstructure:"basic_auth"`

	// Interval between polls. Required.
	Interval time.Duration `mapstructure:"interval"`
	// InitialDelay before the first poll. Defaults to Interval/2.
	InitialDelay time.Duration `mapstructure:"initial_delay"`
	// Timeout per request. Defaults to Interval.
	Timeout time.Duration `mapstructure:"timeout"`

	// ArrayPath is a dot-delimited path to an array within the response
	// (e.g. "data.items") yielding one log record per element. Empty means
	// the whole response becomes one record.
	ArrayPath string `mapstructure:"array_path"`

	// BodyKey is the JSON key promoted to the log body per record.
	BodyKey string `mapstructure:"body_key"`

	// TimestampKey/Layout parse an event timestamp from each record.
	TimestampKey string `mapstructure:"timestamp_key"`
	Layout       string `mapstructure:"layout"`

	// Attributes are added to every record.
	Attributes map[string]interface{} `mapstructure:"attributes"`
}

var _ confmap.Validator = (*Config)(nil)

func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("endpoint must be set")
	}
	if !strings.HasPrefix(cfg.Endpoint, "http://") && !strings.HasPrefix(cfg.Endpoint, "https://") {
		return fmt.Errorf("endpoint must start with http:// or https://")
	}
	if cfg.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	switch cfg.Method {
	case "":
		cfg.Method = "GET"
	case "GET", "POST", "HEAD", "PATCH":
	default:
		return fmt.Errorf("method %q is not one of: GET, POST, HEAD, PATCH", cfg.Method)
	}
	if cfg.Timeout < 0 {
		return fmt.Errorf("timeout must not be negative")
	}
	switch {
	case cfg.BearerToken != "" && cfg.BasicAuth.Username != "":
		return fmt.Errorf("bearer_token and basic_auth are mutually exclusive")
	}
	return nil
}

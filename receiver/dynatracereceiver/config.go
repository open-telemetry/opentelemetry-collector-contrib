// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatracereceiver

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

type Config struct {
	component.Config `mapstructure:",squash"`
	APIEndpoint      string        `mapstructure:"API_ENDPOINT"`
	APIToken         string        `mapstructure:"API_TOKEN"`
	MetricSelectors  []string      `mapstructure:"metric_selectors"`
	Resolution       string        `mapstructure:"resolution"`
	From             string        `mapstructure:"from"`
	To               string        `mapstructure:"to"`
	PollInterval     time.Duration `mapstructure:"poll_interval"`
	MaxRetries       int           `mapstructure:"max_retries"`
	HTTPTimeout      time.Duration `mapstructure:"http_timeout"`
}

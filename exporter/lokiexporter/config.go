// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Loki exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`
}

func (c *Config) Validate() error {
	if err := c.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	if _, err := url.Parse(c.Endpoint); c.Endpoint == "" || err != nil {
		return fmt.Errorf("\"endpoint\" must be a valid URL")
	}
	return nil
}

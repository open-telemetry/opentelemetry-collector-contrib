// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"errors"
	"fmt"
	"net/url"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Loki exporter.
type Config struct {
	confighttp.ClientConfig   `mapstructure:",squash"`        // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`

	DefaultLabelsEnabled map[string]bool `mapstructure:"default_labels_enabled"`
}

func (c *Config) Validate() error {
	if err := c.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	if _, err := url.Parse(c.Endpoint); c.Endpoint == "" || err != nil {
		return errors.New("\"endpoint\" must be a valid URL")
	}
	return nil
}

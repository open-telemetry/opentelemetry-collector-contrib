// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

// Config defines configuration settings for the Faro exporter.
type Config struct {
	confighttp.ClientConfig `mapstructure:",squash"`
	QueueConfig             exporterhelper.QueueConfig `mapstructure:"sending_queue"`
	RetryConfig             configretry.BackOffConfig  `mapstructure:"retry_on_failure"`
}

var (
	_ component.Config    = (*Config)(nil)
	_ confmap.Unmarshaler = (*Config)(nil)
)

func (c *Config) Validate() error {
	var errs error
	if c.Endpoint == "" {
		errs = multierr.Append(errs, errors.New("endpoint is required"))
	}
	return errs
}

func (c *Config) Unmarshal(component *confmap.Conf) error {
	if component == nil {
		return nil
	}

	if err := component.Unmarshal(c); err != nil {
		return err
	}

	return nil
}

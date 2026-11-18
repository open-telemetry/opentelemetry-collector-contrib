// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awskinesisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

const (
	defaultIdleConnTimeout = 90 * time.Second
)

// AWSConfig contains AWS specific configuration such as awskinesis stream, region, etc.
type AWSConfig struct {
	StreamName      string `mapstructure:"stream_name"`
	KinesisEndpoint string `mapstructure:"kinesis_endpoint"`
	Region          string `mapstructure:"region"`
	Role            string `mapstructure:"role"`
}

type Encoding struct {
	Name        string `mapstructure:"name"`
	Compression string `mapstructure:"compression"`
}

// Config contains the main configuration options for the awskinesis exporter
type Config struct {
	TimeoutSettings exporterhelper.TimeoutConfig                             `mapstructure:",squash"`
	QueueSettings   configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	BackOffConfig   configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`

	Encoding           Encoding  `mapstructure:"encoding"`
	AWS                AWSConfig `mapstructure:"aws"`
	MaxRecordsPerBatch int       `mapstructure:"max_records_per_batch"`
	MaxRecordSize      int       `mapstructure:"max_record_size"`
	// IdleConnTimeout specifies the maximum duration an idle (keep-alive) connection will remain open before being closed.
	// If set to 0 or omitted, the default AWS SDK idle connection timeout (90s) is used.
	IdleConnTimeout time.Duration `mapstructure:"idle_conn_timeout"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	var errs error
	if err := cfg.QueueSettings.Validate(); err != nil {
		errs = multierr.Append(errs, fmt.Errorf("queue settings has invalid configuration: %w", err))
	}
	if cfg.IdleConnTimeout < 0 {
		errs = multierr.Append(errs, errors.New("idle_conn_timeout must be non-negative"))
	}

	return errs
}

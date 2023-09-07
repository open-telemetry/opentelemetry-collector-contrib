// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awskinesisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
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
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	Encoding           `mapstructure:"encoding"`
	AWS                AWSConfig `mapstructure:"aws"`
	MaxRecordsPerBatch int       `mapstructure:"max_records_per_batch"`
	MaxRecordSize      int       `mapstructure:"max_record_size"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if err := cfg.QueueSettings.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	return nil
}

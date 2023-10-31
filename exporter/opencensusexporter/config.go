// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for OpenCensus exporter.
type Config struct {
	configgrpc.GRPCClientSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings  `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings  `mapstructure:"retry_on_failure"`

	// The number of workers that send the gRPC requests.
	NumWorkers int `mapstructure:"num_workers"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	return nil
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"errors"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Config defines configuration for Carbon exporter.
type Config struct {
	// Specifies the connection endpoint config. The default value is "localhost:2003".
	confignet.TCPAddr `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	// MaxIdleConns is used to set a limit to the maximum idle TCP connections the client can keep open. Default value is 100.
	// If `sending_queue` is enabled, it is recommended to use same value as `sending_queue::num_consumers`.
	MaxIdleConns int `mapstructure:"max_idle_conns"`

	// Timeout is the maximum duration allowed to connecting and sending the
	// data to the Carbon/Graphite backend. The default value is 5s.
	exporterhelper.TimeoutSettings `mapstructure:",squash"`     // squash ensures fields are correctly decoded in embedded struct.
	QueueConfig                    exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	RetryConfig                    configretry.BackOffConfig    `mapstructure:"retry_on_failure"`

	// ResourceToTelemetrySettings defines configuration for converting resource attributes to metric labels.
	ResourceToTelemetryConfig resourcetotelemetry.Settings `mapstructure:"resource_to_telemetry_conversion"`
}

func (cfg *Config) Validate() error {
	// Resolve TCP address just to ensure that it is a valid one. It is better
	// to fail here than at when the exporter is started.
	if _, err := net.ResolveTCPAddr("tcp", cfg.Endpoint); err != nil {
		return fmt.Errorf("exporter has an invalid TCP endpoint: %w", err)
	}

	// Negative timeouts are not acceptable, since all sends will fail.
	if cfg.Timeout < 0 {
		return errors.New("'timeout' must be non-negative")
	}

	if cfg.MaxIdleConns < 0 {
		return errors.New("'max_idle_conns' must be non-negative")
	}

	return nil
}

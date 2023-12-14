// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"errors"
	"fmt"
	"net"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Defaults for not specified configuration settings.
const (
	defaultEndpoint = "localhost:2003"
)

// Config defines configuration for Carbon exporter.
type Config struct {
	// Specifies the connection endpoint config. The default value is "localhost:2003".
	confignet.TCPAddr `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// Timeout is the maximum duration allowed to connecting and sending the
	// data to the Carbon/Graphite backend. The default value is 5s.
	exporterhelper.TimeoutSettings `mapstructure:",squash"`     // squash ensures fields are correctly decoded in embedded struct.
	QueueConfig                    exporterhelper.QueueSettings `mapstructure:"sending_queue"`
	RetryConfig                    exporterhelper.RetrySettings `mapstructure:"retry_on_failure"`
}

func (cfg *Config) Validate() error {
	// Resolve TCP address just to ensure that it is a valid one. It is better
	// to fail here than at when the exporter is started.
	if _, err := net.ResolveTCPAddr("tcp", cfg.Endpoint); err != nil {
		return fmt.Errorf("exporter has an invalid TCP endpoint: %w", err)
	}

	// Negative timeouts are not acceptable, since all sends will fail.
	if cfg.Timeout < 0 {
		return errors.New("exporter requires a positive timeout")
	}

	return nil
}

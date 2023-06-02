// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// Defaults for not specified configuration settings.
const (
	DefaultEndpoint    = "localhost:2003"
	DefaultSendTimeout = 5 * time.Second
)

// Config defines configuration for Carbon exporter.
type Config struct {

	// Endpoint specifies host and port to send metrics in the Carbon plaintext
	// format. The default value is defined by the DefaultEndpoint constant.
	Endpoint string `mapstructure:"endpoint"`

	// Timeout is the maximum duration allowed to connecting and sending the
	// data to the Carbon/Graphite backend.
	// The default value is defined by the DefaultSendTimeout constant.
	Timeout time.Duration `mapstructure:"timeout"`
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

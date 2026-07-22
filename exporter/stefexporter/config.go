// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter"

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for STEF exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"`
	QueueConfig                  configoptional.Optional[exporterhelper.QueueBatchConfig] `mapstructure:"sending_queue"`
	RetryConfig                  configretry.BackOffConfig                                `mapstructure:"retry_on_failure"`
	ClientConfig                 configgrpc.ClientConfig                                  `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (c *Config) Validate() error {
	endpoint := c.sanitizedEndpoint()
	if endpoint == "" {
		return errors.New(`requires a non-empty "endpoint"`)
	}

	// Validate that the port is in the address
	_, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return err
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf(`invalid port "%s"`, port)
	}

	switch c.ClientConfig.Compression {
	case "":
	case "zstd":
	default:
		return fmt.Errorf("unsupported compression method %q", c.ClientConfig.Compression)
	}

	return nil
}

// TODO: move this to configgrpc.ClientConfig to avoid this code duplication (copied from OTLP exporter).
func (c *Config) sanitizedEndpoint() string {
	switch {
	case strings.HasPrefix(c.ClientConfig.Endpoint, "http://"):
		return strings.TrimPrefix(c.ClientConfig.Endpoint, "http://")
	case strings.HasPrefix(c.ClientConfig.Endpoint, "https://"):
		return strings.TrimPrefix(c.ClientConfig.Endpoint, "https://")
	case strings.HasPrefix(c.ClientConfig.Endpoint, "dns://"):
		r := regexp.MustCompile(`^dns:///?`)
		return r.ReplaceAllString(c.ClientConfig.Endpoint, "")
	default:
		return c.ClientConfig.Endpoint
	}
}

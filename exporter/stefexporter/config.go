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
)

// Config defines configuration for logging exporter.
type Config struct {
	configgrpc.ClientConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
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

	return nil
}

// TODO: move this to configgrpc.ClientConfig to avoid this code duplication (copied from OTLP exporter).
func (c *Config) sanitizedEndpoint() string {
	switch {
	case strings.HasPrefix(c.Endpoint, "http://"):
		return strings.TrimPrefix(c.Endpoint, "http://")
	case strings.HasPrefix(c.Endpoint, "https://"):
		return strings.TrimPrefix(c.Endpoint, "https://")
	case strings.HasPrefix(c.Endpoint, "dns://"):
		r := regexp.MustCompile("^dns://[/]?")
		return r.ReplaceAllString(c.Endpoint, "")
	default:
		return c.Endpoint
	}
}

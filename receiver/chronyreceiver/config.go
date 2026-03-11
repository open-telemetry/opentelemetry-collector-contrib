// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chronyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver"

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/chrony"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	// Endpoint is the published address or unix socket
	// that allows clients to connect to:
	// The allowed format is:
	//   unix:///path/to/chronyd/unix.sock
	//   udp://localhost:323
	//
	// The default value is unix:///var/run/chrony/chronyd.sock
	Endpoint string `mapstructure:"endpoint"`

	// LocalEndpoint is the local address to bind when using Unix datagram sockets.
	// Required when the collector and chronyd run in separate network namespaces
	// (e.g., different containers) sharing a filesystem volume for the socket.
	// When empty, Go's default abstract socket autobind is used (same-namespace only).
	// Example: unix:///run/chrony/otel-reply.sock
	LocalEndpoint string `mapstructure:"local_endpoint"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var (
	_ component.Config = (*Config)(nil)

	errInvalidValue = errors.New("invalid value")
)

func newDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.Timeout = 10 * time.Second
	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),

		Endpoint: "unix:///var/run/chrony/chronyd.sock",
	}
}

func (c *Config) Validate() error {
	if c.Timeout < 1 {
		return fmt.Errorf("must have a positive timeout: %w", errInvalidValue)
	}
	network, _, err := chrony.SplitNetworkEndpoint(c.Endpoint)
	if err != nil {
		return err
	}
	if c.LocalEndpoint != "" {
		if network != "unixgram" {
			return fmt.Errorf("local_endpoint is only supported with unix/unixgram endpoints: %w", errInvalidValue)
		}
		_, endpoint, err := chrony.ParseEndpointPath(c.LocalEndpoint)
		if err != nil {
			return err
		}
		dir := filepath.Dir(endpoint)
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("local_endpoint parent directory %q: %w", dir, err)
		}
	}
	return nil
}

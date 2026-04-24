// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package chronyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver"

import (
	"errors"
	"fmt"
	"os"
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

	// FileMountPath is the directory where the receiver creates a random
	// unixgram reply socket.
	// Use it only when the collector and chronyd run in different network
	// namespaces but share a filesystem volume.
	// This directory should be dedicated to chronyd and the collector.
	// Prefer an ephemeral mount because ungraceful exits can leave stale socket
	// files behind.
	// When empty, Go's default abstract socket autobind is used (same-namespace only).
	// Example: /run/chrony
	FileMountPath string `mapstructure:"file_mount_path"`

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
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),

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
	if c.FileMountPath != "" {
		if network != "unixgram" {
			return fmt.Errorf("file_mount_path is only supported with unix/unixgram endpoints: %w", errInvalidValue)
		}
		fi, err := os.Stat(c.FileMountPath)
		if err != nil {
			return fmt.Errorf("file_mount_path directory %q: %w", c.FileMountPath, err)
		}
		if !fi.IsDir() {
			return fmt.Errorf("file_mount_path %q is not a directory: %w", c.FileMountPath, errInvalidValue)
		}
	}
	return nil
}

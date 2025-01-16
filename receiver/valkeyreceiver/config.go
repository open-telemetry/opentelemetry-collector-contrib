// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package valkeyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver"

import (
	"fmt"
	"net"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// The target endpoint.
	confignet.AddrConfig `mapstructure:",squash"`

	TLS configtls.ClientConfig `mapstructure:"tls,omitempty"`

	MetricsBuilderConfig metadata.MetricsBuilderConfig `mapstructure:",squash"`
}

// configInfo holds configuration information to be used as resource/metrics attributes.
type configInfo struct {
	Address string
	Port    string
}

func newConfigInfo(cfg *Config) (configInfo, error) {
	address, port, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return configInfo{}, fmt.Errorf("invalid endpoint %q: %w", cfg.Endpoint, err)
	}
	return configInfo{Address: address, Port: port}, nil
}

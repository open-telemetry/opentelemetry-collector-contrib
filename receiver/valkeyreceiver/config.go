// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package valkeyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver"

import (
	"fmt"
	"net"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver/internal/metadata"
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	// TODO: Use one of the configs from core.
	// The target endpoint.
	confignet.AddrConfig `mapstructure:",squash"`

	// Optional username. Use the specified Username to authenticate the current connection
	// with one of the connections defined in the ACL list when connecting
	// to a valkey instance that is using the valkey ACL system.
	Username string `mapstructure:"username"`

	// Optional password. Must match the password specified in the
	// requirepass server configuration option, or the user's password when connecting
	// to a valkey instance that is using the valkey ACL system.
	Password configopaque.String `mapstructure:"password"`

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

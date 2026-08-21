// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"golang.org/x/time/rate"
)

const defaultEndpoint = "localhost:12001"

type Config struct {
	ServerConfig confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// Limit is a float that indicates the maximum number of messages repeated
	// through the websocket by this processor in messages per second. Defaults to 1.
	Limit rate.Limit `mapstructure:"limit"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func createDefaultConfig() component.Config {
	netAddr := confignet.NewDefaultAddrConfig()
	netAddr.Transport = confignet.TransportTypeTCP
	netAddr.Endpoint = defaultEndpoint
	serverConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig.WriteTimeout = 0
	serverConfig.ReadHeaderTimeout = 0
	serverConfig.IdleTimeout = 0
	serverConfig.KeepAlivesEnabled = false
	serverConfig.NetAddr = netAddr
	return &Config{
		ServerConfig: serverConfig,
		Limit:        1,
	}
}

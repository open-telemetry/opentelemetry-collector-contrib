// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension/internal/httpserver"

import "go.opentelemetry.io/collector/config/confighttp"

const (
	// DefaultServerEndpoint is the default port for the local metadata HTTP server.
	DefaultServerEndpoint = "localhost:9875"
)

// Config contains the information necessary for configuring the local metadata HTTP server.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	Path                    string `mapstructure:"path"`
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarder // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder"

import (
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for http forwarder extension.
type Config struct {

	// Ingress holds config settings for HTTP server listening for requests.
	Ingress confighttp.HTTPServerSettings `mapstructure:"ingress"`

	// Egress holds config settings to use for forwarded requests.
	Egress confighttp.HTTPClientSettings `mapstructure:"egress"`
}

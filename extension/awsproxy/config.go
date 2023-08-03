// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsproxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

// Config defines the configuration for an AWS X-Ray proxy.
type Config struct {

	// ProxyServer defines configurations related to the local TCP proxy server.
	ProxyConfig proxy.Config `mapstructure:",squash"`
}

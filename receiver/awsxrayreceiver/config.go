// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver"

import (
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"
)

// Config defines the configurations for an AWS X-Ray receiver.
type Config struct {
	// The `NetAddr` represents the UDP address
	// and port on which this receiver listens for X-Ray segment documents
	// emitted by the X-Ray SDK.
	confignet.NetAddr `mapstructure:",squash"`

	// ProxyServer defines configurations related to the local TCP proxy server.
	ProxyServer *proxy.Config `mapstructure:"proxy_server"`
}

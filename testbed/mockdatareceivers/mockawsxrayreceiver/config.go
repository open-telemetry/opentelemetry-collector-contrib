// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mockawsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver"

import (
	"go.opentelemetry.io/collector/config/configtls"
)

// Config defines configuration for xray receiver.
type Config struct {

	// The target endpoint.
	Endpoint string `mapstructure:"endpoint"`

	// Configures the receiver to use TLS.
	// The default value is nil, which will cause the receiver to not use TLS.
	TLSCredentials *configtls.TLSSetting `mapstructure:"tls, omitempty"`
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package envoyalsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
)

type Config struct {
	ServerConfig configgrpc.ServerConfig `mapstructure:",squash"`
	// prevent unkeyed literal initialization
	_ struct{}
}

var _ component.Config = (*Config)(nil)

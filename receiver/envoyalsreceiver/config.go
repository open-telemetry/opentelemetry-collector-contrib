// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package envoyalsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
)

type Config struct {
	configgrpc.ServerConfig `mapstructure:",squash"`
}

var _ component.Config = (*Config)(nil)

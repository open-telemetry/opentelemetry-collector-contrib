// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"

import "go.opentelemetry.io/collector/config/configgrpc"

// Config defines configuration for STEF receiver.
type Config struct {
	configgrpc.ServerConfig `mapstructure:",squash"`
}

func (c *Config) Validate() error {
	return nil
}

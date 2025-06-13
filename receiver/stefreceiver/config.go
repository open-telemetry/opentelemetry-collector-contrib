// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stefreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/configgrpc"
)

// Config defines configuration for STEF receiver.
type Config struct {
	configgrpc.ServerConfig `mapstructure:",squash"`
	AckInterval             time.Duration `mapstructure:"ack_interval"`
}

func (c *Config) Validate() error {
	return nil
}

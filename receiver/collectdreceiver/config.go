// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confignet"
)

// Config defines configuration for Collectd receiver.
type Config struct {
	confignet.TCPAddr `mapstructure:",squash"`

	Timeout          time.Duration `mapstructure:"timeout"`
	AttributesPrefix string        `mapstructure:"attributes_prefix"`
	Encoding         string        `mapstructure:"encoding"`
}

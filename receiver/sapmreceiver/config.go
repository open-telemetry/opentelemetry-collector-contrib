// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver"

import (
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for SAPM receiver.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
}

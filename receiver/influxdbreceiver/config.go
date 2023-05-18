// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package influxdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver"

import (
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for the InfluxDB receiver.
type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"`
}

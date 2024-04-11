// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusapiserverextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension"

import (
	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	PrometheusReceiverName string `mapstructure:"prometheus_receiver_name"`
	Server confighttp.ServerConfig `mapstructure:"server"` // squash ensures fields are correctly decoded in embedded struct
}

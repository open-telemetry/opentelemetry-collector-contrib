// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachedruidreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachedruidreceiver"

import (
	"go.opentelemetry.io/collector/config/confighttp"
)

// Config defines configuration for the Apache Druid receiver.
type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`

	// MetricsPath for metrics data collection, default is '/services/collector/metrics'
	MetricsPath string `mapstructure:"metrics_path"`

	// The name of Druid cluster
	ClusterName string `mapstructure:"cluster_name"`
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver/internal/metadata"

// WithCurrentConnectionsAsGauge sets the current connections metric as a gauge.
func WithCurrentConnectionsAsGauge() resourceMetricsBuilderOption {
	return func(rmb *ResourceMetricsBuilder) {
		if rmb.metricNginxConnectionsCurrent.config.Enabled {
			rmb.metricNginxConnectionsCurrent.config.Enabled = false
			rmb.metricTempConnectionsCurrent.config.Enabled = true
			rmb.metricTempConnectionsCurrent.data.SetName(rmb.metricNginxConnectionsCurrent.data.Name())
			rmb.metricTempConnectionsCurrent.data.SetDescription(rmb.metricNginxConnectionsCurrent.data.Description())
		}
	}
}

// WithCurrentConnectionsAsGaugeDisabled disables the current connections metric as a gauge.
// This is necessary because the metric must be enabled by default in order to be able to apply the other option.
func WithCurrentConnectionsAsGaugeDisabled() resourceMetricsBuilderOption {
	return func(rmb *ResourceMetricsBuilder) {
		rmb.metricTempConnectionsCurrent.config.Enabled = false
	}
}

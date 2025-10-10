// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import "go.opentelemetry.io/collector/config/confignet"

// LogsConfig defines logs exporter specific configuration
type LogsConfig struct {
	// TCPAddr.Endpoint is the host of the Datadog intake server to send logs to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddrConfig `mapstructure:",squash"`

	// UseCompression enables the logs agent to compress logs before sending them.
	// Note: this config option does not apply when the `exporter.datadogexporter.UseLogsAgentExporter` feature flag is disabled.
	UseCompression bool `mapstructure:"use_compression"`

	// CompressionLevel accepts values from 0 (no compression) to 9 (maximum compression but higher resource usage).
	// Only takes effect if UseCompression is set to true.
	// Note: this config option does not apply when the `exporter.datadogexporter.UseLogsAgentExporter` feature flag is disabled.
	CompressionLevel int `mapstructure:"compression_level"`

	// BatchWait represents the maximum time the logs agent waits to fill each batch of logs before sending.
	// Note: this config option does not apply when the `exporter.datadogexporter.UseLogsAgentExporter` feature flag is disabled.
	BatchWait int `mapstructure:"batch_wait"`
}

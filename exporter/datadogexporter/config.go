// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"encoding"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

const (
	// Deprecated: [v0.110.0] Use `datadogconfig.TypeStr` instead.
	// DefaultSite is the default site of the Datadog intake to send data to
	DefaultSite = datadogconfig.DefaultSite
)

// Deprecated: [v0.110.0] Use `datadogconfig.TypeStr` instead.
// APIConfig defines the API configuration options
type APIConfig = datadogconfig.APIConfig

// Deprecated: [v0.110.0] Use `datadogconfig.MetricsConfig` instead.
// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig = datadogconfig.MetricsConfig

// Deprecated: [v0.110.0] Use `datadogconfig.HistogramMode` instead.
// HistogramMode is the export mode for OTLP Histogram metrics.
type HistogramMode = datadogconfig.HistogramMode

const (
	// HistogramModeNoBuckets reports no bucket histogram metrics. .sum and .count metrics will still be sent
	// if `send_count_sum_metrics` is enabled.
	HistogramModeNoBuckets HistogramMode = datadogconfig.HistogramModeNoBuckets
	// HistogramModeCounters reports histograms as Datadog counts, one metric per bucket.
	HistogramModeCounters HistogramMode = datadogconfig.HistogramModeCounters
	// HistogramModeDistributions reports histograms as Datadog distributions (recommended).
	HistogramModeDistributions HistogramMode = datadogconfig.HistogramModeDistributions
)

var _ encoding.TextUnmarshaler = (*HistogramMode)(nil)

// Deprecated: [v0.110.0] Use `datadogconfig.HistogramConfig` instead.
// HistogramConfig customizes export of OTLP Histograms.
type HistogramConfig = datadogconfig.HistogramConfig

// Deprecated: [v0.110.0] Use `datadogconfig.CumulativeMonotonicMode` instead.
// CumulativeMonotonicSumMode is the export mode for OTLP Sum metrics.
type CumulativeMonotonicSumMode = datadogconfig.CumulativeMonotonicSumMode

const (
	// Deprecated: [v0.110.0] Use `datadogconfig.CumulativeMonotonicSumMode` instead.
	// CumulativeMonotonicSumModeToDelta calculates delta for
	// cumulative monotonic sum metrics in the client side and reports
	// them as Datadog counts.
	CumulativeMonotonicSumModeToDelta CumulativeMonotonicSumMode = datadogconfig.CumulativeMonotonicSumModeToDelta

	// Deprecated: [v0.110.0] Use `datadogconfig.CumulativeMonotonicSumMode` instead.
	// CumulativeMonotonicSumModeRawValue reports the raw value for
	// cumulative monotonic sum metrics as a Datadog gauge.
	CumulativeMonotonicSumModeRawValue CumulativeMonotonicSumMode = datadogconfig.CumulativeMonotonicSumModeRawValue
)

var _ encoding.TextUnmarshaler = (*CumulativeMonotonicSumMode)(nil)

// Deprecated: [v0.110.0] Use `datadogconfig.InitialValueMode` instead.
// InitialValueMode defines what the exporter should do with the initial value
// of a time series when transforming from cumulative to delta.
type InitialValueMode = datadogconfig.InitialValueMode

const (
	// InitialValueModeAuto reports the initial value if its start timestamp
	// is set and it happens after the process was started.
	InitialValueModeAuto InitialValueMode = datadogconfig.InitialValueModeAuto

	// InitialValueModeDrop always drops the initial value.
	InitialValueModeDrop InitialValueMode = datadogconfig.InitialValueModeDrop

	// InitialValueModeKeep always reports the initial value.
	InitialValueModeKeep InitialValueMode = datadogconfig.InitialValueModeKeep
)

var _ encoding.TextUnmarshaler = (*InitialValueMode)(nil)

// Deprecated: [v0.110.0] Use `datadogconfig.SumConfig` instead.
// SumConfig customizes export of OTLP Sums.
type SumConfig = datadogconfig.SumConfig

// Deprecated: [v0.110.0] Use `datadogconfig.SummaryMode` instead.
// SummaryMode is the export mode for OTLP Summary metrics.
type SummaryMode = datadogconfig.SummaryMode

const (
	// SummaryModeNoQuantiles sends no `.quantile` metrics. `.sum` and `.count` metrics will still be sent.
	SummaryModeNoQuantiles SummaryMode = datadogconfig.SummaryModeNoQuantiles
	// SummaryModeGauges sends `.quantile` metrics as gauges tagged by the quantile.
	SummaryModeGauges SummaryMode = datadogconfig.SummaryModeGauges
)

var _ encoding.TextUnmarshaler = (*SummaryMode)(nil)

// Deprecated: [v0.110.0] Use `datadogconfig.SummaryMode` instead.
// SummaryConfig customizes export of OTLP Summaries.
type SummaryConfig = datadogconfig.SummaryConfig

// Deprecated: [v0.110.0] Use `datadogconfig.MetricsExporterConfig` instead.
// MetricsExporterConfig provides options for a user to customize the behavior of the
// metrics exporter
type MetricsExporterConfig = datadogconfig.MetricsExporterConfig

// Deprecated: [v0.110.0] Use `datadogconfig.TracesExporterConfig` instead.
// TracesConfig defines the traces exporter specific configuration options
type TracesConfig = datadogconfig.TracesExporterConfig

// Deprecated: [v0.110.0] Use `datadogconfig.LogsConfig` instead.
// LogsConfig defines logs exporter specific configuration
type LogsConfig = datadogconfig.LogsConfig

// Deprecated: [v0.110.0] Use `datadogconfig.TagsConfig` instead.
// TagsConfig defines the tag-related configuration
// It is embedded in the configuration
type TagsConfig = datadogconfig.TagsConfig

// Deprecated: [v0.110.0] Use `datadogconfig.HostnameSource` instead.
// HostnameSource is the source for the hostname of host metadata.
type HostnameSource = datadogconfig.HostnameSource

const (
	// Deprecated: [v0.110.0] Use `datadogconfig.HostnameSource` instead.
	// HostnameSourceFirstResource picks the host metadata hostname from the resource
	// attributes on the first OTLP payload that gets to the exporter. If it is lacking any
	// hostname-like attributes, it will fallback to 'config_or_system' behavior (see below).
	//
	// Do not use this hostname source if receiving data from multiple hosts.
	HostnameSourceFirstResource HostnameSource = datadogconfig.HostnameSourceFirstResource //nolint:staticcheck

	// Deprecated: [v0.110.0] Use `datadogconfig.HostnameSource` instead.
	// HostnameSourceConfigOrSystem picks the host metadata hostname from the 'hostname' setting,
	// and if this is empty, from available system APIs and cloud provider endpoints.
	HostnameSourceConfigOrSystem HostnameSource = datadogconfig.HostnameSourceConfigOrSystem
)

var _ encoding.TextUnmarshaler = (*HostnameSource)(nil)

// Deprecated: [v0.110.0] Use `datadogconfig.HostMetadataConfig` instead.
// HostMetadataConfig defines the host metadata related configuration.
// Host metadata is the information used for populating the infrastructure list,
// the host map and providing host tags functionality.
//
// The exporter will send host metadata for a single host, whose name is chosen
// according to `host_metadata::hostname_source`.
type HostMetadataConfig = datadogconfig.HostMetadataConfig

// Deprecated: [v0.110.0] Use `datadogconfig.Config` instead.
// Config defines configuration for the Datadog exporter.
type Config = datadogconfig.Config

var _ component.Config = (*Config)(nil)

var _ confmap.Unmarshaler = (*Config)(nil)

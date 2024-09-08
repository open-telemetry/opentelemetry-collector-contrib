// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
)

const (
	// DefaultSite is the default site of the Datadog intake to send data to
	DefaultSite = "datadoghq.com"
)

// APIConfig defines the API configuration options
type APIConfig = datadog.APIConfig

// Deprecated: Use `datadog.MetricsConfig` instead.
// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig = datadog.MetricsConfig

// Deprecated: Use `datadog.HistogramMode` instead.
// HistogramMode is the export mode for OTLP Histogram metrics.
type HistogramMode = datadog.HistogramMode

const (
	// HistogramModeNoBuckets reports no bucket histogram metrics. .sum and .count metrics will still be sent
	// if `send_count_sum_metrics` is enabled.
	HistogramModeNoBuckets HistogramMode = datadog.HistogramModeNoBuckets
	// HistogramModeCounters reports histograms as Datadog counts, one metric per bucket.
	HistogramModeCounters HistogramMode = datadog.HistogramModeCounters
	// HistogramModeDistributions reports histograms as Datadog distributions (recommended).
	HistogramModeDistributions HistogramMode = datadog.HistogramModeDistributions
)

var _ encoding.TextUnmarshaler = (*HistogramMode)(nil)

// Deprecated: Use `datadog.HistogramConfig` instead.
// HistogramConfig customizes export of OTLP Histograms.
type HistogramConfig = datadog.HistogramConfig

// Deprecated: Use `datadog.CumulativeMonotonicMode` instead.
// CumulativeMonotonicSumMode is the export mode for OTLP Sum metrics.
type CumulativeMonotonicSumMode = datadog.CumulativeMonotonicSumMode

const (
	// Deprecated: Use `datadog.CumulativeMonotonicSumMode` instead.
	// CumulativeMonotonicSumModeToDelta calculates delta for
	// cumulative monotonic sum metrics in the client side and reports
	// them as Datadog counts.
	CumulativeMonotonicSumModeToDelta CumulativeMonotonicSumMode = datadog.CumulativeMonotonicSumModeToDelta

	// Deprecated: Use `datadog.CumulativeMonotonicSumMode` instead.
	// CumulativeMonotonicSumModeRawValue reports the raw value for
	// cumulative monotonic sum metrics as a Datadog gauge.
	CumulativeMonotonicSumModeRawValue CumulativeMonotonicSumMode = datadog.CumulativeMonotonicSumModeRawValue
)

var _ encoding.TextUnmarshaler = (*CumulativeMonotonicSumMode)(nil)

// Deprecated: Use `datadog.InitialValueMode` instead.
// InitialValueMode defines what the exporter should do with the initial value
// of a time series when transforming from cumulative to delta.
type InitialValueMode = datadog.InitialValueMode

const (
	// InitialValueModeAuto reports the initial value if its start timestamp
	// is set and it happens after the process was started.
	InitialValueModeAuto InitialValueMode = datadog.InitialValueModeAuto

	// InitialValueModeDrop always drops the initial value.
	InitialValueModeDrop InitialValueMode = datadog.InitialValueModeDrop

	// InitialValueModeKeep always reports the initial value.
	InitialValueModeKeep InitialValueMode = datadog.InitialValueModeKeep
)

var _ encoding.TextUnmarshaler = (*InitialValueMode)(nil)

// Deprecated: Use `datadog.SumConfig` instead.
// SumConfig customizes export of OTLP Sums.
type SumConfig = datadog.SumConfig

// Deprecated: Use `datadog.SummaryMode` instead.
// SummaryMode is the export mode for OTLP Summary metrics.
type SummaryMode = datadog.SummaryMode

const (
	// SummaryModeNoQuantiles sends no `.quantile` metrics. `.sum` and `.count` metrics will still be sent.
	SummaryModeNoQuantiles SummaryMode = datadog.SummaryModeNoQuantiles
	// SummaryModeGauges sends `.quantile` metrics as gauges tagged by the quantile.
	SummaryModeGauges SummaryMode = datadog.SummaryModeGauges
)

var _ encoding.TextUnmarshaler = (*SummaryMode)(nil)

// Deprecated: Use `datadog.SummaryMode` instead.
// SummaryConfig customizes export of OTLP Summaries.
type SummaryConfig = datadog.SummaryConfig

// Deprecated: Use `datadog.MetricsExporterConfig` instead.
// MetricsExporterConfig provides options for a user to customize the behavior of the
// metrics exporter
type MetricsExporterConfig = datadog.MetricsExporterConfig

// Deprecated: Use `datadog.TracesConfig` instead.
// TracesConfig defines the traces exporter specific configuration options
type TracesConfig = datadog.TracesConfig

// Deprecated: Use `datadog.LogsConfig` instead.
// LogsConfig defines logs exporter specific configuration
type LogsConfig = datadog.LogsConfig

// Deprecated: Use `datadog.TagsConfig` instead.
// TagsConfig defines the tag-related configuration
// It is embedded in the configuration
type TagsConfig = datadog.TagsConfig

// Deprecated: Use `datadog.HostnameSource` instead.
// HostnameSource is the source for the hostname of host metadata.
type HostnameSource = datadog.HostnameSource

const (
	// Deprecated: Use `datadog.HostnameSource` instead.
	// HostnameSourceFirstResource picks the host metadata hostname from the resource
	// attributes on the first OTLP payload that gets to the exporter. If it is lacking any
	// hostname-like attributes, it will fallback to 'config_or_system' behavior (see below).
	//
	// Do not use this hostname source if receiving data from multiple hosts.
	HostnameSourceFirstResource HostnameSource = datadog.HostnameSourceFirstResource

	// Deprecated: Use `datadog.HostnameSource` instead.
	// HostnameSourceConfigOrSystem picks the host metadata hostname from the 'hostname' setting,
	// and if this is empty, from available system APIs and cloud provider endpoints.
	HostnameSourceConfigOrSystem HostnameSource = datadog.HostnameSourceConfigOrSystem
)

var _ encoding.TextUnmarshaler = (*HostnameSource)(nil)

// Deprecated: Use `datadog.HostMetadataConfig` instead.
// HostMetadataConfig defines the host metadata related configuration.
// Host metadata is the information used for populating the infrastructure list,
// the host map and providing host tags functionality.
//
// The exporter will send host metadata for a single host, whose name is chosen
// according to `host_metadata::hostname_source`.
type HostMetadataConfig = datadog.HostMetadataConfig

// Deprecated: Use `datadog.Config` instead.
// Config defines configuration for the Datadog exporter.
type Config = datadog.Config

var _ component.Config = (*Config)(nil)

var _ confmap.Unmarshaler = (*Config)(nil)

// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package config defines the Datadog exporter configuration.
// Deprecated: [v0.55.0] Use datadogexporter structs and types instead.
package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
)

const (
	// DefaultSite is the default site of the Datadog intake to send data to
	// Deprecated: [v0.55.0] use datadogexporter.DefaultSite instead.
	DefaultSite = datadogexporter.DefaultSite
)

// APIConfig defines the API configuration options
// Deprecated: [v0.55.0] use datadogexporter.APIConfig instead.
type APIConfig = datadogexporter.APIConfig

// MetricsConfig defines the metrics exporter specific configuration options
// Deprecated: [v0.55.0] use datadogexporter.MetricsConfig instead.
type MetricsConfig = datadogexporter.MetricsConfig

// HistogramMode defines the modes for exporting OTLP Histograms via the Datadog exporter.
// Deprecated: [v0.55.0] use datadogexporter.HistogramMode instead.
type HistogramMode = datadogexporter.HistogramMode

const (
	// HistogramModeNoBuckets reports no bucket histogram metrics. .sum and .count metrics will still be sent
	// if `send_count_sum_metrics` is enabled.
	// Deprecated: [v0.55.0] use datadogexporter.HistogramModeNoBuckets instead.
	HistogramModeNoBuckets HistogramMode = datadogexporter.HistogramModeNoBuckets
	// HistogramModeCounters reports histograms as Datadog counts, one metric per bucket.
	// Deprecated: [v0.55.0] use datadogexporter.HistogramModeCounters instead.
	HistogramModeCounters HistogramMode = datadogexporter.HistogramModeCounters
	// HistogramModeDistributions reports histograms as Datadog distributions (recommended).
	// Deprecated: [v0.55.0] use datadogexporter.HistogramModeDistributions instead.
	HistogramModeDistributions HistogramMode = datadogexporter.HistogramModeDistributions
)

// HistogramConfig customizes export of OTLP Histograms.
// Deprecated: [v0.55.0] use datadogexporter.HistogramConfig instead.
type HistogramConfig = datadogexporter.HistogramConfig

// CumulativeMonotonicSumMode is the export mode for OTLP Sum metrics.
// Deprecated: [v0.55.0] use datadogexporter.CumulativeMonotonicSumMode instead.
type CumulativeMonotonicSumMode = datadogexporter.CumulativeMonotonicSumMode

const (
	// CumulativeMonotonicSumModeToDelta calculates delta for
	// cumulative monotonic sum metrics in the client side and reports
	// them as Datadog counts.
	// Deprecated: [v0.55.0] use datadogexporter.CumulativeMonotonicSumModeToDelta instead.
	CumulativeMonotonicSumModeToDelta CumulativeMonotonicSumMode = datadogexporter.CumulativeMonotonicSumModeToDelta

	// CumulativeMonotonicSumModeRawValue reports the raw value for
	// cumulative monotonic sum metrics as a Datadog gauge.
	// Deprecated: [v0.55.0] use datadogexporter.CumulativeMonotonicSumModeRawValue instead.
	CumulativeMonotonicSumModeRawValue CumulativeMonotonicSumMode = datadogexporter.CumulativeMonotonicSumModeRawValue
)

// SumConfig customizes export of OTLP Sums.
// Deprecated: [v0.55.0] use datadogexporter.SumConfig instead.
type SumConfig = datadogexporter.SumConfig

// SummaryMode is the export mode for OTLP Summary metrics.
// Deprecated: [v0.55.0] use datadogexporter.SummaryMode instead.
type SummaryMode = datadogexporter.SummaryMode

const (
	// SummaryModeNoQuantiles sends no `.quantile` metrics. `.sum` and `.count` metrics will still be sent.
	// Deprecated: [v0.55.0] use datadogexporter.SummaryModeNoQuantiles instead.
	SummaryModeNoQuantiles SummaryMode = datadogexporter.SummaryModeNoQuantiles
	// SummaryModeGauges sends `.quantile` metrics as gauges tagged by the quantile.
	// Deprecated: [v0.55.0] use datadogexporter.SummaryModeGauges instead.
	SummaryModeGauges SummaryMode = datadogexporter.SummaryModeGauges
)

// SummaryConfig customizes export of OTLP Summaries.
// Deprecated: [v0.55.0] use datadogexporter.SummaryConfig instead.
type SummaryConfig = datadogexporter.SummaryConfig

// MetricsExporterConfig provides options for a user to customize the behavior of the
// metrics exporter
// Deprecated: [v0.55.0] use datadogexporter.MetricsExporterConfig instead.
type MetricsExporterConfig = datadogexporter.MetricsExporterConfig

// TracesConfig defines the traces exporter specific configuration options
// Deprecated: [v0.55.0] use datadogexporter.TracesConfig instead.
type TracesConfig = datadogexporter.TracesConfig

// TagsConfig defines the tag-related configuration
// It is embedded in the configuration
// Deprecated: [v0.55.0] use datadogexporter.TagsConfig instead.
type TagsConfig = datadogexporter.TagsConfig

// HostnameSource is the source for the hostname of host metadata.
// Deprecated: [v0.55.0] use datadogexporter.HostnameSource instead.
type HostnameSource = datadogexporter.HostnameSource

const (
	// HostnameSourceFirstResource picks the host metadata hostname from the resource
	// attributes on the first OTLP payload that gets to the exporter. If it is lacking any
	// hostname-like attributes, it will fallback to 'config_or_system' behavior (see below).
	//
	// Do not use this hostname source if receiving data from multiple hosts.
	// Deprecated: [v0.55.0] use datadogexporter.HostnameSourceFirstResource instead.
	HostnameSourceFirstResource HostnameSource = datadogexporter.HostnameSourceFirstResource

	// HostnameSourceConfigOrSystem picks the host metadata hostname from the 'hostname' setting,
	// and if this is empty, from available system APIs and cloud provider endpoints.
	// Deprecated: [v0.55.0] use datadogexporter.HostnameSourceConfigOrSystem
	HostnameSourceConfigOrSystem HostnameSource = datadogexporter.HostnameSourceConfigOrSystem
)

// HostMetadataConfig defines the host metadata related configuration.
// Host metadata is the information used for populating the infrastructure list,
// the host map and providing host tags functionality.
//
// The exporter will send host metadata for a single host, whose name is chosen
// according to `host_metadata::hostname_source`.
// Deprecated: [v0.55.0] use datadogexporter.HostMetadataConfig instead.
type HostMetadataConfig = datadogexporter.HostMetadataConfig

// LimitedTLSClientSetting is a subset of TLSClientSetting, see LimitedHTTPClientSettings for more details
// Deprecated: [v0.55.0] use datadogexporter.LimitedTLSClientSettings instead.
type LimitedTLSClientSettings = datadogexporter.LimitedTLSClientSettings

// Deprecated: [v0.55.0] use datadogexporter.LimitedHTTPClientSettings instead.
type LimitedHTTPClientSettings = datadogexporter.LimitedHTTPClientSettings

// Config defines configuration for the Datadog exporter.
// Deprecated: [v0.55.0] use datadogexporter.Config instead.
type Config = datadogexporter.Config

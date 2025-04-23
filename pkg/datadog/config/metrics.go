// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import (
	"encoding"
	"errors"
	"fmt"

	otlpmetrics "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"go.opentelemetry.io/collector/config/confignet"
)

// MetricsConfig defines the metrics exporter specific configuration options
type MetricsConfig struct {
	// DeltaTTL defines the time that previous points of a cumulative monotonic
	// metric are kept in memory to calculate deltas
	DeltaTTL int64 `mapstructure:"delta_ttl"`

	// TCPAddr.Endpoint is the host of the Datadog intake server to send metrics to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddrConfig `mapstructure:",squash"`

	ExporterConfig MetricsExporterConfig `mapstructure:",squash"`

	// HistConfig defines the export of OTLP Histograms.
	HistConfig HistogramConfig `mapstructure:"histograms"`

	// SumConfig defines the export of OTLP Sums.
	SumConfig SumConfig `mapstructure:"sums"`

	// SummaryConfig defines the export for OTLP Summaries.
	SummaryConfig SummaryConfig `mapstructure:"summaries"`
}

type HistogramMode string

const (
	// HistogramModeNoBuckets reports no bucket histogram metrics. .sum and .count metrics will still be sent
	// if `send_count_sum_metrics` is enabled.
	HistogramModeNoBuckets HistogramMode = "nobuckets"
	// HistogramModeCounters reports histograms as Datadog counts, one metric per bucket.
	HistogramModeCounters HistogramMode = "counters"
	// HistogramModeDistributions reports histograms as Datadog distributions (recommended).
	HistogramModeDistributions HistogramMode = "distributions"
)

var _ encoding.TextUnmarshaler = (*HistogramMode)(nil)

func (hm *HistogramMode) UnmarshalText(in []byte) error {
	switch mode := HistogramMode(in); mode {
	case HistogramModeCounters, HistogramModeDistributions, HistogramModeNoBuckets:
		*hm = mode
		return nil
	default:
		return fmt.Errorf("invalid histogram mode %q", mode)
	}
}

// HistogramConfig customizes export of OTLP Histograms.
type HistogramConfig struct {
	// Mode for exporting histograms. Valid values are 'distributions', 'counters' or 'nobuckets'.
	//  - 'distributions' sends histograms as Datadog distributions (recommended).
	//  - 'counters' sends histograms as Datadog counts, one metric per bucket.
	//  - 'nobuckets' sends no bucket histogram metrics. Aggregation metrics will still be sent
	//    if `send_aggregation_metrics` is enabled.
	//
	// The current default is 'distributions'.
	Mode HistogramMode `mapstructure:"mode"`

	// SendCountSum states if the export should send .sum and .count metrics for histograms.
	// The default is false.
	// Deprecated: [v0.75.0] Use `send_aggregation_metrics` (HistogramConfig.SendAggregations) instead.
	SendCountSum bool `mapstructure:"send_count_sum_metrics"`

	// SendAggregations states if the exporter should send .sum, .count, .min and .max metrics for histograms.
	// The default is false.
	SendAggregations bool `mapstructure:"send_aggregation_metrics"`
}

func (c *HistogramConfig) validate() error {
	if c.Mode == HistogramModeNoBuckets && !c.SendAggregations {
		return errors.New("'nobuckets' mode and `send_aggregation_metrics` set to false will send no histogram metrics")
	}
	return nil
}

// CumulativeMonotonicSumMode is the export mode for OTLP Sum metrics.
type CumulativeMonotonicSumMode string

const (
	// CumulativeMonotonicSumModeToDelta calculates delta for
	// cumulative monotonic sum metrics in the client side and reports
	// them as Datadog counts.
	CumulativeMonotonicSumModeToDelta CumulativeMonotonicSumMode = "to_delta"

	// CumulativeMonotonicSumModeRawValue reports the raw value for
	// cumulative monotonic sum metrics as a Datadog gauge.
	CumulativeMonotonicSumModeRawValue CumulativeMonotonicSumMode = "raw_value"
)

var _ encoding.TextUnmarshaler = (*CumulativeMonotonicSumMode)(nil)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (sm *CumulativeMonotonicSumMode) UnmarshalText(in []byte) error {
	switch mode := CumulativeMonotonicSumMode(in); mode {
	case CumulativeMonotonicSumModeToDelta,
		CumulativeMonotonicSumModeRawValue:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("invalid cumulative monotonic sum mode %q", mode)
	}
}

// InitialValueMode defines what the exporter should do with the initial value
// of a time series when transforming from cumulative to delta.
type InitialValueMode string

const (
	// InitialValueModeAuto reports the initial value if its start timestamp
	// is set and it happens after the process was started.
	InitialValueModeAuto InitialValueMode = "auto"

	// InitialValueModeDrop always drops the initial value.
	InitialValueModeDrop InitialValueMode = "drop"

	// InitialValueModeKeep always reports the initial value.
	InitialValueModeKeep InitialValueMode = "keep"
)

var _ encoding.TextUnmarshaler = (*InitialValueMode)(nil)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (iv *InitialValueMode) UnmarshalText(in []byte) error {
	switch mode := InitialValueMode(in); mode {
	case InitialValueModeAuto,
		InitialValueModeDrop,
		InitialValueModeKeep:
		*iv = mode
		return nil
	default:
		return fmt.Errorf("invalid initial value mode %q", mode)
	}
}

// SumConfig customizes export of OTLP Sums.
type SumConfig struct {
	// CumulativeMonotonicMode is the mode for exporting OTLP Cumulative Monotonic Sums.
	// Valid values are 'to_delta' or 'raw_value'.
	//  - 'to_delta' calculates delta for cumulative monotonic sums and sends it as a Datadog count.
	//  - 'raw_value' sends the raw value of cumulative monotonic sums as Datadog gauges.
	//
	// The default is 'to_delta'.
	// See https://docs.datadoghq.com/metrics/otlp/?tab=sum#mapping for details and examples.
	CumulativeMonotonicMode CumulativeMonotonicSumMode `mapstructure:"cumulative_monotonic_mode"`

	// InitialCumulativeMonotonicMode defines the behavior of the exporter when receiving the first value
	// of a cumulative monotonic sum.
	InitialCumulativeMonotonicMode InitialValueMode `mapstructure:"initial_cumulative_monotonic_value"`
}

// SummaryMode is the export mode for OTLP Summary metrics.
type SummaryMode string

const (
	// SummaryModeNoQuantiles sends no `.quantile` metrics. `.sum` and `.count` metrics will still be sent.
	SummaryModeNoQuantiles SummaryMode = "noquantiles"
	// SummaryModeGauges sends `.quantile` metrics as gauges tagged by the quantile.
	SummaryModeGauges SummaryMode = "gauges"
)

var _ encoding.TextUnmarshaler = (*SummaryMode)(nil)

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (sm *SummaryMode) UnmarshalText(in []byte) error {
	switch mode := SummaryMode(in); mode {
	case SummaryModeNoQuantiles,
		SummaryModeGauges:
		*sm = mode
		return nil
	default:
		return fmt.Errorf("invalid summary mode %q", mode)
	}
}

// SummaryConfig customizes export of OTLP Summaries.
type SummaryConfig struct {
	// Mode is the the mode for exporting OTLP Summaries.
	// Valid values are 'noquantiles' or 'gauges'.
	//  - 'noquantiles' sends no `.quantile` metrics. `.sum` and `.count` metrics will still be sent.
	//  - 'gauges' sends `.quantile` metrics as gauges tagged by the quantile.
	//
	// The default is 'gauges'.
	// See https://docs.datadoghq.com/metrics/otlp/?tab=summary#mapping for details and examples.
	Mode SummaryMode `mapstructure:"mode"`
}

// MetricsExporterConfig provides options for a user to customize the behavior of the
// metrics exporter
type MetricsExporterConfig struct {
	// ResourceAttributesAsTags, if set to true, will use the exporterhelper feature to transform all
	// resource attributes into metric labels, which are then converted into tags
	ResourceAttributesAsTags bool `mapstructure:"resource_attributes_as_tags"`

	// InstrumentationScopeMetadataAsTags, if set to true, adds the name and version of the
	// instrumentation scope that created a metric to the metric tags
	InstrumentationScopeMetadataAsTags bool `mapstructure:"instrumentation_scope_metadata_as_tags"`
}

// ToTranslatorOpts returns a list of metrics translator options from the metrics config
func (mcfg MetricsConfig) ToTranslatorOpts() []otlpmetrics.TranslatorOption {
	options := []otlpmetrics.TranslatorOption{
		otlpmetrics.WithDeltaTTL(mcfg.DeltaTTL),
	}

	if mcfg.HistConfig.SendAggregations {
		options = append(options, otlpmetrics.WithHistogramAggregations())
	}

	if mcfg.SummaryConfig.Mode == SummaryModeGauges {
		options = append(options, otlpmetrics.WithQuantiles())
	}

	if mcfg.ExporterConfig.InstrumentationScopeMetadataAsTags {
		options = append(options, otlpmetrics.WithInstrumentationScopeMetadataAsTags())
	}

	options = append(options, otlpmetrics.WithHistogramMode(otlpmetrics.HistogramMode(mcfg.HistConfig.Mode)))

	var numberMode otlpmetrics.NumberMode
	switch mcfg.SumConfig.CumulativeMonotonicMode {
	case CumulativeMonotonicSumModeRawValue:
		numberMode = otlpmetrics.NumberModeRawValue
	case CumulativeMonotonicSumModeToDelta:
		numberMode = otlpmetrics.NumberModeCumulativeToDelta
	}
	options = append(options, otlpmetrics.WithNumberMode(numberMode))
	options = append(options, otlpmetrics.WithInitialCumulMonoValueMode(
		otlpmetrics.InitialCumulMonoValueMode(mcfg.SumConfig.InitialCumulativeMonotonicMode)))

	return options
}

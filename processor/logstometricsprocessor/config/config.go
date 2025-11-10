// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/config"

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logstometricsprocessor/internal/customottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

const (
	// defaultExponentialHistogramMaxSize is the default maximum number
	// of buckets per positive or negative number range. 160 buckets
	// default supports a high-resolution histogram able to cover a
	// long-tail latency distribution from 1ms to 100s with a relative
	// error of less than 5%.
	// Ref: https://opentelemetry.io/docs/specs/otel/metrics/sdk/#base2-exponential-bucket-histogram-aggregation
	defaultExponentialHistogramMaxSize = 160
)

var defaultHistogramBuckets = []float64{
	2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000,
}

// Regex for [key] selector after ExtractGrokPatterns
var grokPatternKey = regexp.MustCompile(`ExtractGrokPatterns\([^)]*\)\s*\[[^\]]+\]`)

var _ confmap.Unmarshaler = (*Config)(nil)

// Config for the processor. Only logs configuration is supported.
type Config struct {
	// MetricsExporter is the component ID of the metrics exporter to send extracted metrics to
	MetricsExporter component.ID `mapstructure:"metrics_exporter"`
	// DropLogs controls whether logs are forwarded to the next consumer after processing.
	// If true, logs are dropped after metrics are extracted. Default: false
	DropLogs bool `mapstructure:"drop_logs"`
	// Logs defines the metrics to produce from log records
	Logs []MetricInfo `mapstructure:"logs"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if len(c.Logs) == 0 {
		return errors.New("no logs configuration provided, at least one log metric definition must be specified")
	}
	if c.MetricsExporter == (component.ID{}) {
		return errors.New("metrics_exporter must be specified")
	}

	var multiError error
	parser, err := ottllog.NewParser(
		customottl.LogFuncs(),
		component.TelemetrySettings{Logger: zap.NewNop()},
	)
	if err != nil {
		return fmt.Errorf("failed to create parser for OTTL logs: %w", err)
	}
	for i := range c.Logs {
		log := &c.Logs[i]
		if err := validateMetricInfo(log, parser); err != nil {
			multiError = errors.Join(multiError, fmt.Errorf("failed to validate logs configuration: %w", err))
		}
	}
	return multiError
}

// Unmarshal implements the confmap.Unmarshaler interface. It allows
// unmarshaling the config with a custom logic to allow setting
// default values when/if required.
func (c *Config) Unmarshal(collectorCfg *confmap.Conf) error {
	if collectorCfg == nil {
		return nil
	}
	if err := collectorCfg.Unmarshal(c); err != nil {
		return err
	}
	for i := range c.Logs {
		info := c.Logs[i]
		info.ensureDefaults()
		c.Logs[i] = info
	}
	return nil
}

type Attribute struct {
	Key          string `mapstructure:"key"`
	Optional     bool   `mapstructure:"optional"`
	DefaultValue any    `mapstructure:"default_value"`
}

type Histogram struct {
	Buckets []float64 `mapstructure:"buckets"`
	Count   string    `mapstructure:"count"`
	Value   string    `mapstructure:"value"`
}

type ExponentialHistogram struct {
	MaxSize int32  `mapstructure:"max_size"`
	Count   string `mapstructure:"count"`
	Value   string `mapstructure:"value"`
}

type Sum struct {
	Value string `mapstructure:"value"`
}

type Gauge struct {
	Value string `mapstructure:"value"`
}

// MetricInfo defines the structure of the metric produced by the processor.
type MetricInfo struct {
	Name        string `mapstructure:"name"`
	Description string `mapstructure:"description"`
	// Unit, if not-empty, will set the unit associated with the metric.
	// See: https://github.com/open-telemetry/opentelemetry-collector/blob/b06236cc794982916cc956f20828b3e18eb33264/pdata/pmetric/generated_metric.go#L72-L81
	Unit string `mapstructure:"unit"`
	// IncludeResourceAttributes is a list of resource attributes that
	// needs to be included in the generated metric. If no resource
	// attribute is included in the list then all attributes are included.
	IncludeResourceAttributes []Attribute `mapstructure:"include_resource_attributes"`
	Attributes                []Attribute `mapstructure:"attributes"`
	// Conditions are a set of OTTL conditions which are ORed. Data is
	// processed into metrics only if the sequence evaluates to true.
	Conditions           []string                                      `mapstructure:"conditions"`
	Histogram            configoptional.Optional[Histogram]            `mapstructure:"histogram"`
	ExponentialHistogram configoptional.Optional[ExponentialHistogram] `mapstructure:"exponential_histogram"`
	Sum                  configoptional.Optional[Sum]                  `mapstructure:"sum"`
	Gauge                configoptional.Optional[Gauge]                `mapstructure:"gauge"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (mi *MetricInfo) ensureDefaults() {
	if mi.Histogram.HasValue() {
		// Add default buckets if explicit histogram is defined
		if len(mi.Histogram.Get().Buckets) == 0 {
			mi.Histogram.Get().Buckets = defaultHistogramBuckets
		}
	}
	if mi.ExponentialHistogram.HasValue() {
		if mi.ExponentialHistogram.Get().MaxSize == 0 {
			mi.ExponentialHistogram.Get().MaxSize = defaultExponentialHistogramMaxSize
		}
	}
}

func (mi *MetricInfo) validateAttributes() error {
	tmp := pcommon.NewValueEmpty()
	duplicate := map[string]struct{}{}
	for _, attr := range mi.Attributes {
		if attr.Key == "" {
			return errors.New("attribute key missing")
		}
		if attr.DefaultValue != nil && attr.Optional {
			return errors.New("only one of default_value or optional should be set")
		}
		if _, ok := duplicate[attr.Key]; ok {
			return fmt.Errorf("duplicate key found in attributes config: %s", attr.Key)
		}
		if err := tmp.FromRaw(attr.DefaultValue); err != nil {
			return fmt.Errorf("invalid default value specified for attribute %s", attr.Key)
		}
		duplicate[attr.Key] = struct{}{}
	}
	return nil
}

func (mi *MetricInfo) validateHistogram() error {
	if mi.Histogram.HasValue() {
		h := mi.Histogram.Get()
		if len(h.Buckets) == 0 {
			return errors.New("histogram buckets missing")
		}
		if h.Value == "" {
			return errors.New("value OTTL statement is required")
		}
	}
	if mi.ExponentialHistogram.HasValue() {
		eh := mi.ExponentialHistogram.Get()
		if _, err := structure.NewConfig(
			structure.WithMaxSize(eh.MaxSize),
		).Validate(); err != nil {
			return err
		}
		if eh.Value == "" {
			return errors.New("value OTTL statement is required")
		}
	}
	return nil
}

func (mi *MetricInfo) validateSum() error {
	if mi.Sum.HasValue() {
		if mi.Sum.Get().Value == "" {
			return errors.New("value must be defined for sum metrics")
		}
	}
	return nil
}

func (mi *MetricInfo) validateGauge() error {
	if mi.Gauge.HasValue() {
		if mi.Gauge.Get().Value == "" {
			return errors.New("value must be defined for gauge metrics")
		}
	}
	return nil
}

// validateMetricInfo is an utility method validate all supported metric
// types defined for the metric info including any ottl expressions.
func validateMetricInfo[K any](mi *MetricInfo, parser ottl.Parser[K]) error {
	if mi.Name == "" {
		return errors.New("missing required metric name configuration")
	}
	if err := mi.validateAttributes(); err != nil {
		return fmt.Errorf("attributes validation failed: %w", err)
	}
	if err := mi.validateHistogram(); err != nil {
		return fmt.Errorf("histogram validation failed: %w", err)
	}
	if err := mi.validateSum(); err != nil {
		return fmt.Errorf("sum validation failed: %w", err)
	}
	if err := mi.validateGauge(); err != nil {
		return fmt.Errorf("gauge validation failed: %w", err)
	}

	// Exactly one metric should be defined. Also, validate OTTL expressions,
	// note that, here we only evaluate if statements are valid. Check for
	// required statements are left to the other validations.
	var metricsDefinedCount int
	if mi.Histogram.HasValue() {
		metricsDefinedCount++
		h := mi.Histogram.Get()
		if h.Count != "" {
			if _, err := parser.ParseValueExpression(h.Count); err != nil {
				return fmt.Errorf("failed to parse count OTTL expression for explicit histogram: %w", err)
			}
		}
		if _, err := parser.ParseValueExpression(h.Value); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for explicit histogram: %w", err)
		}
	}
	if mi.ExponentialHistogram.HasValue() {
		metricsDefinedCount++
		eh := mi.ExponentialHistogram.Get()
		if eh.Count != "" {
			if _, err := parser.ParseValueExpression(eh.Count); err != nil {
				return fmt.Errorf("failed to parse count OTTL expression for exponential histogram: %w", err)
			}
		}
		if _, err := parser.ParseValueExpression(eh.Value); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for exponential histogram: %w", err)
		}
	}
	if mi.Sum.HasValue() {
		metricsDefinedCount++
		if _, err := parser.ParseValueExpression(mi.Sum.Get().Value); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for summary: %w", err)
		}
	}
	if mi.Gauge.HasValue() {
		metricsDefinedCount++
		g := mi.Gauge.Get()
		if _, err := parser.ParseValueExpression(g.Value); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for gauge: %w", err)
		}
		// if ExtractGrokPatterns is used, validate the key selector
		if strings.Contains(g.Value, "ExtractGrokPatterns") {
			// Ensure a [key] selector is present after ExtractGrokPatterns
			if !grokPatternKey.MatchString(g.Value) {
				return errors.New("ExtractGrokPatterns: a single key selector[key] is required for signal to gauge")
			}
		}
	}
	if metricsDefinedCount != 1 {
		return fmt.Errorf("exactly one of the metrics must be defined, %d found", metricsDefinedCount)
	}

	// validate OTTL conditions
	if _, err := parser.ParseConditions(mi.Conditions); err != nil {
		return fmt.Errorf("failed to parse OTTL conditions: %w", err)
	}
	return nil
}


// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"

import (
	"errors"
	"fmt"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
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

var _ confmap.Unmarshaler = (*Config)(nil)

// Config for the connector. Each configuration field describes the metrics
// to produce from a specific signal.
type Config struct {
	Spans      []MetricInfo `mapstructure:"spans"`
	Datapoints []MetricInfo `mapstructure:"datapoints"`
	Logs       []MetricInfo `mapstructure:"logs"`
	Profiles   []MetricInfo `mapstructure:"profiles"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if len(c.Spans) == 0 && len(c.Datapoints) == 0 && len(c.Logs) == 0 && len(c.Profiles) == 0 {
		return errors.New("no configuration provided, at least one should be specified")
	}
	var multiError error // collect all errors at once
	if len(c.Spans) > 0 {
		parser, err := ottlspan.NewParser(
			customottl.SpanFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL spans: %w", err)
		}
		for _, span := range c.Spans {
			if err := validateMetricInfo(span, parser); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate spans configuration: %w", err))
			}
		}
	}
	if len(c.Datapoints) > 0 {
		parser, err := ottldatapoint.NewParser(
			customottl.DatapointFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL datapoints: %w", err)
		}
		for _, dp := range c.Datapoints {
			if err := validateMetricInfo(dp, parser); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate datapoints configuration: %w", err))
			}
		}
	}
	if len(c.Logs) > 0 {
		parser, err := ottllog.NewParser(
			customottl.LogFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL logs: %w", err)
		}
		for _, log := range c.Logs {
			if err := validateMetricInfo(log, parser); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate logs configuration: %w", err))
			}
		}
	}
	if len(c.Profiles) > 0 {
		parser, err := ottlprofile.NewParser(
			customottl.ProfileFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL profiles: %w", err)
		}
		for _, profile := range c.Profiles {
			if err := validateMetricInfo(profile, parser); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate profiles configuration: %w", err))
			}
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
	if err := collectorCfg.Unmarshal(c, confmap.WithIgnoreUnused()); err != nil {
		return err
	}
	for i, info := range c.Spans {
		info.ensureDefaults()
		c.Spans[i] = info
	}
	for i, info := range c.Datapoints {
		info.ensureDefaults()
		c.Datapoints[i] = info
	}
	for i, info := range c.Logs {
		info.ensureDefaults()
		c.Logs[i] = info
	}
	for i, info := range c.Profiles {
		info.ensureDefaults()
		c.Profiles[i] = info
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

// MetricInfo defines the structure of the metric produced by the connector.
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
	Conditions           []string              `mapstructure:"conditions"`
	Histogram            *Histogram            `mapstructure:"histogram"`
	ExponentialHistogram *ExponentialHistogram `mapstructure:"exponential_histogram"`
	Sum                  *Sum                  `mapstructure:"sum"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (mi *MetricInfo) ensureDefaults() {
	if mi.Histogram != nil {
		// Add default buckets if explicit histogram is defined
		if len(mi.Histogram.Buckets) == 0 {
			mi.Histogram.Buckets = defaultHistogramBuckets
		}
	}
	if mi.ExponentialHistogram != nil {
		if mi.ExponentialHistogram.MaxSize == 0 {
			mi.ExponentialHistogram.MaxSize = defaultExponentialHistogramMaxSize
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
	if mi.Histogram != nil {
		if len(mi.Histogram.Buckets) == 0 {
			return errors.New("histogram buckets missing")
		}
		if mi.Histogram.Value == "" {
			return errors.New("value OTTL statement is required")
		}
	}
	if mi.ExponentialHistogram != nil {
		if _, err := structure.NewConfig(
			structure.WithMaxSize(mi.ExponentialHistogram.MaxSize),
		).Validate(); err != nil {
			return err
		}
		if mi.ExponentialHistogram.Value == "" {
			return errors.New("value OTTL statement is required")
		}
	}
	return nil
}

func (mi *MetricInfo) validateSum() error {
	if mi.Sum != nil {
		if mi.Sum.Value == "" {
			return errors.New("value must be defined for sum metrics")
		}
	}
	return nil
}

// validateMetricInfo is an utility method validate all supported metric
// types defined for the metric info including any ottl expressions.
func validateMetricInfo[K any](mi MetricInfo, parser ottl.Parser[K]) error {
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

	// Exactly one metric should be defined. Also, validate OTTL expressions,
	// note that, here we only evaluate if statements are valid. Check for
	// required statements are left to the other validations.
	var metricsDefinedCount int
	if mi.Histogram != nil {
		metricsDefinedCount++
		if mi.Histogram.Count != "" {
			if _, err := parser.ParseValueExpression(mi.Histogram.Count); err != nil {
				return fmt.Errorf("failed to parse count OTTL expression for explicit histogram: %w", err)
			}
		}
		if _, err := parser.ParseValueExpression(mi.Histogram.Value); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for explicit histogram: %w", err)
		}
	}
	if mi.ExponentialHistogram != nil {
		metricsDefinedCount++
		if mi.ExponentialHistogram.Count != "" {
			if _, err := parser.ParseValueExpression(mi.ExponentialHistogram.Count); err != nil {
				return fmt.Errorf("failed to parse count OTTL expression for exponential histogram: %w", err)
			}
		}
		if _, err := parser.ParseValueExpression(mi.ExponentialHistogram.Value); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for exponential histogram: %w", err)
		}
	}
	if mi.Sum != nil {
		metricsDefinedCount++
		if _, err := parser.ParseValueExpression(mi.Sum.Value); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for summary: %w", err)
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

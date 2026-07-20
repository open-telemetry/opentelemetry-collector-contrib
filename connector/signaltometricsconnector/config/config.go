// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
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

// Regex for [key] selector after ExtractGrokPatterns
var grokPatternKey = regexp.MustCompile(`ExtractGrokPatterns\([^)]*\)\s*\[[^\]]+\]`)

var _ confmap.Unmarshaler = (*Config)(nil)

// Config for the connector. Each configuration field describes the metrics
// to produce from a specific signal.
type Config struct {
	Spans      []MetricInfo `mapstructure:"spans"`
	Datapoints []MetricInfo `mapstructure:"datapoints"`
	Logs       []MetricInfo `mapstructure:"logs"`
	Profiles   []MetricInfo `mapstructure:"profiles"`
	// ErrorMode determines how the connector reacts to errors that occur while processing an OTTL
	// condition or statement during runtime data consumption. This setting does NOT affect errors
	// during OTTL statement parsing at configuration time - those will always cause startup failures.
	// Valid values are `propagate`, `ignore`, and `silent`.
	// `propagate` means the connector returns the error up the pipeline. This will result in the
	// payload being dropped from the collector.
	// `ignore` means the connector ignores errors returned by conditions and continues processing.
	// If an error occurs, the record is skipped and the error is logged.
	// `silent` means the connector ignores errors returned by conditions and continues processing.
	// If an error occurs, the record is skipped and the error is not logged.
	// The default value is `propagate`.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	if len(c.Spans) == 0 && len(c.Datapoints) == 0 && len(c.Logs) == 0 && len(c.Profiles) == 0 {
		return errors.New("no configuration provided, at least one should be specified")
	}
	var multiError error // collect all errors at once
	nopSettings := component.TelemetrySettings{Logger: zap.NewNop()}
	if len(c.Spans) > 0 {
		ottlParser, err := ottlspan.NewParser(customottl.SpanFuncs(), nopSettings, ottlspan.EnablePathContextNames())
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL spans: %w", err)
		}
		pc, err := ottl.NewParserCollection(
			nopSettings,
			ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottlspan.TransformContext]](true),
			ottl.WithParserCollectionContext(
				ottlspan.ContextName,
				&ottlParser,
				ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottlspan.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottlspan.TransformContext]) (*ottl.ValueExpression[*ottlspan.TransformContext], error) {
					return parsed[0], nil
				}),
			),
		)
		if err != nil {
			return fmt.Errorf("failed to create parser collection for OTTL spans: %w", err)
		}
		for i := range c.Spans {
			span := &c.Spans[i]
			if err := validateMetricInfo(span, pc, ottlspan.ContextName); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate spans configuration: %w", err))
				continue
			}
			if _, err := filterottl.NewBoolExprForSpanWithPathContextNames(span.Conditions, customottl.SpanFuncs(), ottl.PropagateError, nopSettings); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate spans configuration: failed to parse OTTL conditions: %w", err))
			}
		}
	}
	if len(c.Datapoints) > 0 {
		ottlParser, err := ottldatapoint.NewParser(customottl.DatapointFuncs(), nopSettings, ottldatapoint.EnablePathContextNames())
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL datapoints: %w", err)
		}
		pc, err := ottl.NewParserCollection(
			nopSettings,
			ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottldatapoint.TransformContext]](true),
			ottl.WithParserCollectionContext(
				ottldatapoint.ContextName,
				&ottlParser,
				ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottldatapoint.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottldatapoint.TransformContext]) (*ottl.ValueExpression[*ottldatapoint.TransformContext], error) {
					return parsed[0], nil
				}),
			),
		)
		if err != nil {
			return fmt.Errorf("failed to create parser collection for OTTL datapoints: %w", err)
		}
		for i := range c.Datapoints {
			dp := &c.Datapoints[i]
			if err := validateMetricInfo(dp, pc, ottldatapoint.ContextName); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate datapoints configuration: %w", err))
				continue
			}
			if _, err := filterottl.NewBoolExprForDataPointWithPathContextNames(dp.Conditions, customottl.DatapointFuncs(), ottl.PropagateError, nopSettings); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate datapoints configuration: failed to parse OTTL conditions: %w", err))
			}
		}
	}
	if len(c.Logs) > 0 {
		ottlParser, err := ottllog.NewParser(customottl.LogFuncs(), nopSettings, ottllog.EnablePathContextNames())
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL logs: %w", err)
		}
		pc, err := ottl.NewParserCollection(
			nopSettings,
			ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottllog.TransformContext]](true),
			ottl.WithParserCollectionContext(
				ottllog.ContextName,
				&ottlParser,
				ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottllog.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottllog.TransformContext]) (*ottl.ValueExpression[*ottllog.TransformContext], error) {
					return parsed[0], nil
				}),
			),
		)
		if err != nil {
			return fmt.Errorf("failed to create parser collection for OTTL logs: %w", err)
		}
		for i := range c.Logs {
			log := &c.Logs[i]
			if err := validateMetricInfo(log, pc, ottllog.ContextName); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate logs configuration: %w", err))
				continue
			}
			if _, err := filterottl.NewBoolExprForLogWithPathContextNames(log.Conditions, customottl.LogFuncs(), ottl.PropagateError, nopSettings); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate logs configuration: failed to parse OTTL conditions: %w", err))
			}
		}
	}
	if len(c.Profiles) > 0 {
		ottlParser, err := ottlprofile.NewParser(customottl.ProfileFuncs(), nopSettings, ottlprofile.EnablePathContextNames())
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL profiles: %w", err)
		}
		pc, err := ottl.NewParserCollection(
			nopSettings,
			ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ValueExpression[*ottlprofile.TransformContext]](true),
			ottl.WithParserCollectionContext(
				ottlprofile.ContextName,
				&ottlParser,
				ottl.WithValueExpressionConverter(func(_ *ottl.ParserCollection[*ottl.ValueExpression[*ottlprofile.TransformContext]], _ ottl.ValueExpressionsGetter, parsed []*ottl.ValueExpression[*ottlprofile.TransformContext]) (*ottl.ValueExpression[*ottlprofile.TransformContext], error) {
					return parsed[0], nil
				}),
			),
		)
		if err != nil {
			return fmt.Errorf("failed to create parser collection for OTTL profiles: %w", err)
		}
		for i := range c.Profiles {
			profile := &c.Profiles[i]
			if err := validateMetricInfo(profile, pc, ottlprofile.ContextName); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate profiles configuration: %w", err))
				continue
			}
			if _, err := filterottl.NewBoolExprForProfileWithPathContextNames(profile.Conditions, customottl.ProfileFuncs(), ottl.PropagateError, nopSettings); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate profiles configuration: failed to parse OTTL conditions: %w", err))
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
	if err := collectorCfg.Unmarshal(c); err != nil {
		return err
	}
	for i := range c.Spans {
		info := c.Spans[i]
		info.ensureDefaults()
		c.Spans[i] = info
	}
	for i := range c.Datapoints {
		info := c.Datapoints[i]
		info.ensureDefaults()
		c.Datapoints[i] = info
	}
	for i := range c.Logs {
		info := c.Logs[i]
		info.ensureDefaults()
		c.Logs[i] = info
	}
	for i := range c.Profiles {
		info := c.Profiles[i]
		info.ensureDefaults()
		c.Profiles[i] = info
	}
	return nil
}

type Attribute struct {
	Key string `mapstructure:"key"`
	// KeysExpression is an OTTL value expression that resolves to a list
	// of attribute keys at runtime. The expression must return a
	// pcommon.Slice or []string. Exactly one of Key or KeysExpression
	// must be set.
	KeysExpression string `mapstructure:"keys_expression"`
	Optional       bool   `mapstructure:"optional"`
	DefaultValue   any    `mapstructure:"default_value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Histogram struct {
	Buckets []float64 `mapstructure:"buckets"`
	Count   string    `mapstructure:"count"`
	Value   string    `mapstructure:"value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type ExponentialHistogram struct {
	MaxSize int32  `mapstructure:"max_size"`
	Count   string `mapstructure:"count"`
	Value   string `mapstructure:"value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Sum struct {
	Value       string `mapstructure:"value"`
	IsMonotonic bool   `mapstructure:"monotonic"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Gauge struct {
	Value string `mapstructure:"value"`
	// prevent unkeyed literal initialization
	_ struct{}
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

// validateAttributeConfigs validates a list of Attribute configs. Each entry
// must have exactly one of Key or KeysExpression set. OTTL expressions are
// parsed to verify syntax. The label parameter is used in error messages.
func validateAttributeConfigs[K any](attrs []Attribute, pc *ottl.ParserCollection[*ottl.ValueExpression[K]], contextName, label string) error {
	tmp := pcommon.NewValueEmpty()
	duplicate := map[string]struct{}{}
	for _, attr := range attrs {
		hasKey := attr.Key != ""
		hasExpr := attr.KeysExpression != ""
		if hasKey == hasExpr {
			return fmt.Errorf("exactly one of key or keys_expression must be set for %s", label)
		}
		if hasExpr {
			if _, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{attr.KeysExpression}), true); err != nil {
				return fmt.Errorf("failed to parse keys_expression for %s: %w", label, err)
			}
		}
		if hasKey {
			if _, ok := duplicate[attr.Key]; ok {
				return fmt.Errorf("duplicate key found in %s config: %s", label, attr.Key)
			}
			duplicate[attr.Key] = struct{}{}
		}
		if attr.DefaultValue != nil && attr.Optional {
			return fmt.Errorf("only one of default_value or optional should be set for %s", label)
		}
		if attr.DefaultValue != nil {
			if err := tmp.FromRaw(attr.DefaultValue); err != nil {
				return fmt.Errorf("invalid default value specified for %s attribute %s", label, attr.Key)
			}
		}
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
// Condition parsing is handled by the caller because it needs a
// signal-specific filterottl helper that is not generic over K.
func validateMetricInfo[K any](mi *MetricInfo, pc *ottl.ParserCollection[*ottl.ValueExpression[K]], contextName string) error {
	if mi.Name == "" {
		return errors.New("missing required metric name configuration")
	}
	if err := validateAttributeConfigs(mi.IncludeResourceAttributes, pc, contextName, "include_resource_attributes"); err != nil {
		return fmt.Errorf("include_resource_attributes validation failed: %w", err)
	}
	if err := validateAttributeConfigs(mi.Attributes, pc, contextName, "attributes"); err != nil {
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
			if _, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{h.Count}), true); err != nil {
				return fmt.Errorf("failed to parse count OTTL expression for explicit histogram: %w", err)
			}
		}
		if _, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{h.Value}), true); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for explicit histogram: %w", err)
		}
	}
	if mi.ExponentialHistogram.HasValue() {
		metricsDefinedCount++
		eh := mi.ExponentialHistogram.Get()
		if eh.Count != "" {
			if _, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{eh.Count}), true); err != nil {
				return fmt.Errorf("failed to parse count OTTL expression for exponential histogram: %w", err)
			}
		}
		if _, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{eh.Value}), true); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for exponential histogram: %w", err)
		}
	}
	if mi.Sum.HasValue() {
		metricsDefinedCount++
		if _, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{mi.Sum.Get().Value}), true); err != nil {
			return fmt.Errorf("failed to parse value OTTL expression for summary: %w", err)
		}
	}
	if mi.Gauge.HasValue() {
		metricsDefinedCount++
		g := mi.Gauge.Get()
		if _, err := pc.ParseValueExpressionsWithContext(contextName, ottl.NewValueExpressionsGetter([]string{g.Value}), true); err != nil {
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
	return nil
}

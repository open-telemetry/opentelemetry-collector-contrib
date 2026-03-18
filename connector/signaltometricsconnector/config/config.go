// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
	if len(c.Spans) > 0 {
		parser, err := ottlspan.NewParser(
			c.SpanParserFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL spans: %w", err)
		}
		synCtx := syntheticSpanCtx()
		defer synCtx.Close()
		for i := range c.Spans {
			span := &c.Spans[i]
			if err := validateMetricInfo(span, parser, synCtx); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate spans configuration: %w", err))
			}
		}
	}
	if len(c.Datapoints) > 0 {
		parser, err := ottldatapoint.NewParser(
			c.DatapointParserFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL datapoints: %w", err)
		}
		synCtx := syntheticDatapointCtx()
		defer synCtx.Close()
		for i := range c.Datapoints {
			dp := &c.Datapoints[i]
			if err := validateMetricInfo(dp, parser, synCtx); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate datapoints configuration: %w", err))
			}
		}
	}
	if len(c.Logs) > 0 {
		parser, err := ottllog.NewParser(
			c.LogParserFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL logs: %w", err)
		}
		synCtx := syntheticLogCtx()
		defer synCtx.Close()
		for i := range c.Logs {
			log := &c.Logs[i]
			if err := validateMetricInfo(log, parser, synCtx); err != nil {
				multiError = errors.Join(multiError, fmt.Errorf("failed to validate logs configuration: %w", err))
			}
		}
	}
	if len(c.Profiles) > 0 {
		parser, err := ottlprofile.NewParser(
			c.ProfileParserFuncs(),
			component.TelemetrySettings{Logger: zap.NewNop()},
		)
		if err != nil {
			return fmt.Errorf("failed to create parser for OTTL profiles: %w", err)
		}
		synCtx := syntheticProfileCtx()
		defer synCtx.Close()
		for i := range c.Profiles {
			profile := &c.Profiles[i]
			if err := validateMetricInfo(profile, parser, synCtx); err != nil {
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
	Key          string `mapstructure:"key"`
	Optional     bool   `mapstructure:"optional"`
	DefaultValue any    `mapstructure:"default_value"`
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
	Value string `mapstructure:"value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type Gauge struct {
	Value string `mapstructure:"value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// DynamicResourceAttributes configures a dynamic OTTL expression for
// selecting resource attributes at runtime.
type DynamicResourceAttributes struct {
	// Statement is an OTTL value expression that must evaluate to a
	// pcommon.Map. The resulting key-value pairs are merged into the
	// metric's resource attributes.
	Statement string `mapstructure:"statement"`
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
	Conditions []string `mapstructure:"conditions"`
	// DynamicResourceAttributes configures an optional OTTL value
	// expression whose result (a pcommon.Map) is merged into the
	// metric's resource attributes alongside the statically configured
	// IncludeResourceAttributes.
	DynamicResourceAttributes *DynamicResourceAttributes                    `mapstructure:"dynamic_resource_attributes"`
	Histogram                 configoptional.Optional[Histogram]            `mapstructure:"histogram"`
	ExponentialHistogram      configoptional.Optional[ExponentialHistogram] `mapstructure:"exponential_histogram"`
	Sum                       configoptional.Optional[Sum]                  `mapstructure:"sum"`
	Gauge                     configoptional.Optional[Gauge]                `mapstructure:"gauge"`
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

func (mi *MetricInfo) validateIncludeResourceAttributes() error {
	tmp := pcommon.NewValueEmpty()
	duplicate := map[string]struct{}{}
	for _, attr := range mi.IncludeResourceAttributes {
		if err := validateAttribute(attr, tmp, duplicate); err != nil {
			return err
		}
	}
	return nil
}

func (mi *MetricInfo) validateAttributes() error {
	tmp := pcommon.NewValueEmpty()
	duplicate := map[string]struct{}{}
	for _, attr := range mi.Attributes {
		if err := validateAttribute(attr, tmp, duplicate); err != nil {
			return err
		}
	}
	return nil
}

func validateAttribute(attr Attribute, tmp pcommon.Value, duplicate map[string]struct{}) error {
	if attr.Key == "" {
		return errors.New("key must be set for an attribute")
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

// validateMetricInfo validates all supported metric types defined for the
// metric info including any OTTL expressions. syntheticCtx is evaluated
// against the dynamic_resource_attributes expression (if set) to catch
// return-type errors at config time.
func validateMetricInfo[K any](mi *MetricInfo, parser ottl.Parser[K], syntheticCtx K) error {
	if mi.Name == "" {
		return errors.New("missing required metric name configuration")
	}
	if err := mi.validateIncludeResourceAttributes(); err != nil {
		return fmt.Errorf("include_resource_attributes validation failed: %w", err)
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

	if mi.DynamicResourceAttributes != nil && mi.DynamicResourceAttributes.Statement != "" {
		expr, err := parser.ParseValueExpression(mi.DynamicResourceAttributes.Statement)
		if err != nil {
			return fmt.Errorf("failed to parse dynamic_resource_attributes OTTL expression: %w", err)
		}
		if err := validateDynResAttrReturnType(expr, syntheticCtx); err != nil {
			return err
		}
	}
	return nil
}

// validateDynResAttrReturnType evaluates a parsed dynamic_resource_attributes
// expression against a synthetic transform context to verify the return type
// at config time. If the expression cannot be evaluated with synthetic data
// (e.g. it calls a custom function that requires real signal data), the check
// is skipped and the runtime type assertion will catch it.
func validateDynResAttrReturnType[K any](expr *ottl.ValueExpression[K], syntheticCtx K) error {
	result, err := expr.Eval(context.Background(), syntheticCtx)
	if err != nil {
		return nil
	}
	if _, ok := result.(pcommon.Map); !ok {
		return fmt.Errorf(
			"dynamic_resource_attributes statement must return a pcommon.Map, "+
				"but evaluated to %T; ensure the expression "+
				"returns a Map (e.g. resource.attributes)",
			result,
		)
	}
	return nil
}

func syntheticSpanCtx() *ottlspan.TransformContext {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	s := ss.Spans().AppendEmpty()
	return ottlspan.NewTransformContextPtr(rs, ss, s)
}

func syntheticDatapointCtx() *ottldatapoint.TransformContext {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetEmptyGauge().DataPoints().AppendEmpty()
	return ottldatapoint.NewTransformContextPtr(rm, sm, m, m.Gauge().DataPoints().At(0))
}

func syntheticLogCtx() *ottllog.TransformContext {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	return ottllog.NewTransformContextPtr(rl, sl, lr)
}

func syntheticProfileCtx() *ottlprofile.TransformContext {
	pd := pprofile.NewProfiles()
	rp := pd.ResourceProfiles().AppendEmpty()
	sp := rp.ScopeProfiles().AppendEmpty()
	p := sp.Profiles().AppendEmpty()
	return ottlprofile.NewTransformContextPtr(rp, sp, p, pd.Dictionary())
}

// SpanParserFuncs returns the OTTL function map for span parsing.
func (c *Config) SpanParserFuncs() map[string]ottl.Factory[*ottlspan.TransformContext] {
	return customottl.SpanFuncs()
}

// DatapointParserFuncs returns the OTTL function map for datapoint parsing.
func (c *Config) DatapointParserFuncs() map[string]ottl.Factory[*ottldatapoint.TransformContext] {
	return customottl.DatapointFuncs()
}

// LogParserFuncs returns the OTTL function map for log parsing.
func (c *Config) LogParserFuncs() map[string]ottl.Factory[*ottllog.TransformContext] {
	return customottl.LogFuncs()
}

// ProfileParserFuncs returns the OTTL function map for profile parsing.
func (c *Config) ProfileParserFuncs() map[string]ottl.Factory[*ottlprofile.TransformContext] {
	return customottl.ProfileFuncs()
}

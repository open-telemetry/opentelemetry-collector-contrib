// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package countconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// Default metrics are emitted if no conditions are specified.
const (
	defaultMetricNameSpans      = "trace.span.count"
	defaultMetricDescSpans      = "The number of spans observed."
	defaultMetricNameSpanEvents = "trace.span.event.count"
	defaultMetricDescSpanEvents = "The number of span events observed."

	defaultMetricNameMetrics    = "metric.count"
	defaultMetricDescMetrics    = "The number of metrics observed."
	defaultMetricNameDataPoints = "metric.datapoint.count"
	defaultMetricDescDataPoints = "The number of data points observed."

	defaultMetricNameLogs = "log.record.count"
	defaultMetricDescLogs = "The number of log records observed."

	defaultMetricNameProfiles = "profile.count"
	defaultMetricDescProfiles = "The number of profiles observed."
)

// Config for the connector
type Config struct {
	Spans      map[string]MetricInfo `mapstructure:"spans"`
	SpanEvents map[string]MetricInfo `mapstructure:"spanevents"`
	Metrics    map[string]MetricInfo `mapstructure:"metrics"`
	DataPoints map[string]MetricInfo `mapstructure:"datapoints"`
	Logs       map[string]MetricInfo `mapstructure:"logs"`
	Profiles   map[string]MetricInfo `mapstructure:"profiles"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// MetricInfo for a data type
type MetricInfo struct {
	Description string            `mapstructure:"description"`
	Conditions  []string          `mapstructure:"conditions"`
	Attributes  []AttributeConfig `mapstructure:"attributes"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type AttributeConfig struct {
	Key          string `mapstructure:"key"`
	DefaultValue any    `mapstructure:"default_value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	for name, info := range c.Spans {
		if name == "" {
			return errors.New("spans: metric name missing")
		}
		if _, err := filterottl.NewBoolExprForSpan(info.Conditions, filterottl.StandardSpanFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			return fmt.Errorf("spans condition: metric %q: %w", name, err)
		}
		if err := info.validateAttributes(); err != nil {
			return fmt.Errorf("spans attributes: metric %q: %w", name, err)
		}
	}
	for name, info := range c.SpanEvents {
		if name == "" {
			return errors.New("spanevents: metric name missing")
		}
		if _, err := filterottl.NewBoolExprForSpanEvent(info.Conditions, filterottl.StandardSpanEventFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			return fmt.Errorf("spanevents condition: metric %q: %w", name, err)
		}
		if err := info.validateAttributes(); err != nil {
			return fmt.Errorf("spanevents attributes: metric %q: %w", name, err)
		}
	}
	for name, info := range c.Metrics {
		if name == "" {
			return errors.New("metrics: metric name missing")
		}
		if _, err := filterottl.NewBoolExprForMetric(info.Conditions, filterottl.StandardMetricFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			return fmt.Errorf("metrics condition: metric %q: %w", name, err)
		}
		if len(info.Attributes) > 0 {
			return fmt.Errorf("metrics attributes not supported: metric %q", name)
		}
	}

	for name, info := range c.DataPoints {
		if name == "" {
			return errors.New("datapoints: metric name missing")
		}
		if _, err := filterottl.NewBoolExprForDataPoint(info.Conditions, filterottl.StandardDataPointFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			return fmt.Errorf("datapoints condition: metric %q: %w", name, err)
		}
		if err := info.validateAttributes(); err != nil {
			return fmt.Errorf("spans attributes: metric %q: %w", name, err)
		}
	}
	for name, info := range c.Logs {
		if name == "" {
			return errors.New("logs: metric name missing")
		}
		if _, err := filterottl.NewBoolExprForLog(info.Conditions, filterottl.StandardLogFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			return fmt.Errorf("logs condition: metric %q: %w", name, err)
		}
		if err := info.validateAttributes(); err != nil {
			return fmt.Errorf("logs attributes: metric %q: %w", name, err)
		}
	}
	for name, info := range c.Profiles {
		if name == "" {
			return errors.New("profiles: metric name missing")
		}
		if _, err := filterottl.NewBoolExprForProfile(info.Conditions, filterottl.StandardProfileFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			return fmt.Errorf("profiles condition: metric %q: %w", name, err)
		}
		if err := info.validateAttributes(); err != nil {
			return fmt.Errorf("profiles attributes: metric %q: %w", name, err)
		}
	}
	return nil
}

func (i *MetricInfo) validateAttributes() error {
	for _, attr := range i.Attributes {
		if attr.Key == "" {
			return errors.New("attribute key missing")
		}
	}
	return nil
}

var _ confmap.Unmarshaler = (*Config)(nil)

// Unmarshal with custom logic to set default values.
// This is necessary to ensure that default metrics are
// not configured if the user has specified any custom metrics.
func (c *Config) Unmarshal(componentParser *confmap.Conf) error {
	if componentParser == nil {
		// Nothing to do if there is no config given.
		return nil
	}
	if err := componentParser.Unmarshal(c, confmap.WithIgnoreUnused()); err != nil {
		return err
	}
	if !componentParser.IsSet("spans") {
		c.Spans = defaultSpansConfig()
	}
	if !componentParser.IsSet("spanevents") {
		c.SpanEvents = defaultSpanEventsConfig()
	}
	if !componentParser.IsSet("metrics") {
		c.Metrics = defaultMetricsConfig()
	}
	if !componentParser.IsSet("datapoints") {
		c.DataPoints = defaultDataPointsConfig()
	}
	if !componentParser.IsSet("logs") {
		c.Logs = defaultLogsConfig()
	}
	if !componentParser.IsSet("profiles") {
		c.Profiles = defaultProfilesConfig()
	}
	return nil
}

func defaultSpansConfig() map[string]MetricInfo {
	return map[string]MetricInfo{
		defaultMetricNameSpans: {
			Description: defaultMetricDescSpans,
		},
	}
}

func defaultSpanEventsConfig() map[string]MetricInfo {
	return map[string]MetricInfo{
		defaultMetricNameSpanEvents: {
			Description: defaultMetricDescSpanEvents,
		},
	}
}

func defaultMetricsConfig() map[string]MetricInfo {
	return map[string]MetricInfo{
		defaultMetricNameMetrics: {
			Description: defaultMetricDescMetrics,
		},
	}
}

func defaultDataPointsConfig() map[string]MetricInfo {
	return map[string]MetricInfo{
		defaultMetricNameDataPoints: {
			Description: defaultMetricDescDataPoints,
		},
	}
}

func defaultLogsConfig() map[string]MetricInfo {
	return map[string]MetricInfo{
		defaultMetricNameLogs: {
			Description: defaultMetricDescLogs,
		},
	}
}

func defaultProfilesConfig() map[string]MetricInfo {
	return map[string]MetricInfo{
		defaultMetricNameProfiles: {
			Description: defaultMetricDescProfiles,
		},
	}
}

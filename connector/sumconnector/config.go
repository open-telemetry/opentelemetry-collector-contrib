// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/sumconnector"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// Config for the connector
type Config struct {
	Spans      map[string]MetricInfo `mapstructure:"spans"`
	SpanEvents map[string]MetricInfo `mapstructure:"spanevents"`
	Metrics    map[string]MetricInfo `mapstructure:"metrics"`
	DataPoints map[string]MetricInfo `mapstructure:"datapoints"`
	Logs       map[string]MetricInfo `mapstructure:"logs"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// MetricInfo for a data type
type MetricInfo struct {
	Description     string            `mapstructure:"description"`
	Conditions      []string          `mapstructure:"conditions"`
	Attributes      []AttributeConfig `mapstructure:"attributes"`
	SourceAttribute string            `mapstructure:"source_attribute"`
	// prevent unkeyed literal initialization
	_ struct{}
}

type AttributeConfig struct {
	Key          string `mapstructure:"key"`
	DefaultValue any    `mapstructure:"default_value"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() (combinedErrors error) {
	for name, info := range c.Spans {
		if name == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("spans: metric name missing"))
		}
		if info.SourceAttribute == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("spans: metric source_attribute missing"))
		}
		if _, err := filterottl.NewBoolExprForSpan(info.Conditions, filterottl.StandardSpanFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("spans condition: metric %q: %w", name, err))
		}
		if err := info.validateAttributes(); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("spans attributes: metric %q: %w", name, err))
		}
	}
	for name, info := range c.SpanEvents {
		if name == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("spanevents: metric name missing"))
		}
		if info.SourceAttribute == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("spanevents: metric source_attribute missing"))
		}
		if _, err := filterottl.NewBoolExprForSpanEvent(info.Conditions, filterottl.StandardSpanEventFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("spanevents condition: metric %q: %w", name, err))
		}
		if err := info.validateAttributes(); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("spanevents attributes: metric %q: %w", name, err))
		}
	}
	for name, info := range c.Metrics {
		if name == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("metrics: metric name missing"))
		}
		if info.SourceAttribute == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("metrics: metric source_attribute missing"))
		}
		if _, err := filterottl.NewBoolExprForMetric(info.Conditions, filterottl.StandardMetricFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("metrics condition: metric %q: %w", name, err))
		}
		if len(info.Attributes) > 0 {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("metrics attributes not supported: metric %q", name))
		}
	}
	for name, info := range c.DataPoints {
		if name == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("datapoints: metric name missing"))
		}
		if info.SourceAttribute == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("datapoints: metric source_attribute missing"))
		}
		if _, err := filterottl.NewBoolExprForDataPoint(info.Conditions, filterottl.StandardDataPointFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("datapoints condition: metric %q: %w", name, err))
		}
		if err := info.validateAttributes(); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("datapoints attributes: metric %q: %w", name, err))
		}
	}
	for name, info := range c.Logs {
		if name == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("logs: metric name missing"))
		}
		if info.SourceAttribute == "" {
			combinedErrors = errors.Join(combinedErrors, errors.New("logs: metric source_attribute missing"))
		}
		if _, err := filterottl.NewBoolExprForLog(info.Conditions, filterottl.StandardLogFuncs(), ottl.PropagateError, component.TelemetrySettings{Logger: zap.NewNop()}); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("logs condition: metric %q: %w", name, err))
		}
		if err := info.validateAttributes(); err != nil {
			combinedErrors = errors.Join(combinedErrors, fmt.Errorf("logs attributes: metric %q: %w", name, err))
		}
	}
	return combinedErrors
}

func (i *MetricInfo) validateAttributes() error {
	for _, attr := range i.Attributes {
		if attr.Key == "" {
			return errors.New("attribute key missing")
		}
	}
	return nil
}

var _ xconfmap.Validator = (*Config)(nil)

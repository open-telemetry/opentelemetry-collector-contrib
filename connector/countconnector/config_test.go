// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package countconnector

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	testCases := []struct {
		name   string
		expect *Config
	}{
		{
			name: "",
			expect: &Config{
				Spans: map[string]MetricInfo{
					defaultMetricNameSpans: {
						Description: defaultMetricDescSpans,
					},
				},
				SpanEvents: map[string]MetricInfo{
					defaultMetricNameSpanEvents: {
						Description: defaultMetricDescSpanEvents,
					},
				},
				Metrics: map[string]MetricInfo{
					defaultMetricNameMetrics: {
						Description: defaultMetricDescMetrics,
					},
				},
				DataPoints: map[string]MetricInfo{
					defaultMetricNameDataPoints: {
						Description: defaultMetricDescDataPoints,
					},
				},
				Logs: map[string]MetricInfo{
					defaultMetricNameLogs: {
						Description: defaultMetricDescLogs,
					},
				},
			},
		},
		{
			name: "custom_description",
			expect: &Config{
				Spans: map[string]MetricInfo{
					defaultMetricNameSpans: {
						Description: "My description for default span count metric.",
					},
				},
				SpanEvents: map[string]MetricInfo{
					defaultMetricNameSpanEvents: {
						Description: "My description for default span event count metric.",
					},
				},
				Metrics: map[string]MetricInfo{
					defaultMetricNameMetrics: {
						Description: "My description for default metric count metric.",
					},
				},
				DataPoints: map[string]MetricInfo{
					defaultMetricNameDataPoints: {
						Description: "My description for default datapoint count metric.",
					},
				},
				Logs: map[string]MetricInfo{
					defaultMetricNameLogs: {
						Description: "My description for default log count metric.",
					},
				},
			},
		},
		{
			name: "custom_metric",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.count": {
						Description: "My span count.",
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.count": {
						Description: "My span event count.",
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.count": {
						Description: "My metric count.",
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.count": {
						Description: "My data point count.",
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.count": {
						Description: "My log record count.",
					},
				},
			},
		},
		{
			name: "condition",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.count": {
						Description: "My span count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-s")`},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.count": {
						Description: "My span event count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-e")`},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.count": {
						Description: "My metric count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-m")`},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.count": {
						Description: "My data point count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-d")`},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.count": {
						Description: "My log record count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-l")`},
					},
				},
			},
		},
		{
			name: "multiple_condition",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.count": {
						Description: "My span count.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-s")`,
							`IsMatch(resource.attributes["foo"], "bar-s")`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.count": {
						Description: "My span event count.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-e")`,
							`IsMatch(resource.attributes["foo"], "bar-e")`,
						},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.count": {
						Description: "My metric count.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-m")`,
							`IsMatch(resource.attributes["foo"], "bar-m")`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.count": {
						Description: "My data point count.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-d")`,
							`IsMatch(resource.attributes["foo"], "bar-d")`,
						},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.count": {
						Description: "My log record count.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-l")`,
							`IsMatch(resource.attributes["foo"], "bar-l")`,
						},
					},
				},
			},
		},
		{
			name: "attribute",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.count": {
						Description: "My span count by environment.",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.count": {
						Description: "My span event count by environment.",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.count": {
						Description: "My metric count.",
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.count": {
						Description: "My data point count by environment.",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.count": {
						Description: "My log record count by environment.",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
			},
		},
		{
			name: "multiple_metrics",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.count": {
						Description: "My span count.",
					},
					"limited.span.count": {
						Description: "Limited span count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-s")`},
						Attributes: []AttributeConfig{
							{
								Key: "env",
							},
							{
								Key:          "component",
								DefaultValue: "other",
							},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.count": {
						Description: "My span event count.",
					},
					"limited.spanevent.count": {
						Description: "Limited span event count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-e")`},
						Attributes: []AttributeConfig{
							{
								Key: "env",
							},
							{
								Key:          "component",
								DefaultValue: "other",
							},
						},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.count": {
						Description: "My metric count."},
					"limited.metric.count": {
						Description: "Limited metric count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-m")`},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.count": {
						Description: "My data point count.",
					},
					"limited.datapoint.count": {
						Description: "Limited data point count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-d")`},
						Attributes: []AttributeConfig{
							{
								Key: "env",
							},
							{
								Key:          "component",
								DefaultValue: "other",
							},
						},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.count": {
						Description: "My log record count.",
					},
					"limited.logrecord.count": {
						Description: "Limited log record count.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-l")`},
						Attributes: []AttributeConfig{
							{
								Key: "env",
							},
							{
								Key:          "component",
								DefaultValue: "other",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.Equal(t, tc.expect, cfg)
		})
	}
}

func TestConfigErrors(t *testing.T) {
	testCases := []struct {
		name   string
		input  *Config
		expect string
	}{
		{
			name: "missing_metric_name_span",
			input: &Config{
				Spans: map[string]MetricInfo{
					"": {
						Description: defaultMetricDescSpans,
					},
				},
			},
			expect: "spans: metric name missing",
		},
		{
			name: "missing_metric_name_spanevent",
			input: &Config{
				SpanEvents: map[string]MetricInfo{
					"": {
						Description: defaultMetricDescSpans,
					},
				},
			},
			expect: "spanevents: metric name missing",
		},
		{
			name: "missing_metric_name_metric",
			input: &Config{
				Metrics: map[string]MetricInfo{
					"": {
						Description: defaultMetricDescSpans,
					},
				},
			},
			expect: "metrics: metric name missing",
		},
		{
			name: "missing_metric_name_datapoint",
			input: &Config{
				DataPoints: map[string]MetricInfo{
					"": {
						Description: defaultMetricDescSpans,
					},
				},
			},
			expect: "datapoints: metric name missing",
		},
		{
			name: "missing_metric_name_log",
			input: &Config{
				Logs: map[string]MetricInfo{
					"": {
						Description: defaultMetricDescSpans,
					},
				},
			},
			expect: "logs: metric name missing",
		},
		{
			name: "invalid_condition_span",
			input: &Config{
				Spans: map[string]MetricInfo{
					defaultMetricNameSpans: {
						Description: defaultMetricDescSpans,
						Conditions:  []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("spans condition: metric %q: unable to parse OTTL statement", defaultMetricNameSpans),
		},
		{
			name: "invalid_condition_spanevent",
			input: &Config{
				SpanEvents: map[string]MetricInfo{
					defaultMetricNameSpanEvents: {
						Description: defaultMetricDescSpans,
						Conditions:  []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("spanevents condition: metric %q: unable to parse OTTL statement", defaultMetricNameSpanEvents),
		},
		{
			name: "invalid_condition_metric",
			input: &Config{
				Metrics: map[string]MetricInfo{
					defaultMetricNameMetrics: {
						Description: defaultMetricDescSpans,
						Conditions:  []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("metrics condition: metric %q: unable to parse OTTL statement", defaultMetricNameMetrics),
		},
		{
			name: "invalid_condition_datapoint",
			input: &Config{
				DataPoints: map[string]MetricInfo{
					defaultMetricNameDataPoints: {
						Description: defaultMetricDescSpans,
						Conditions:  []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("datapoints condition: metric %q: unable to parse OTTL statement", defaultMetricNameDataPoints),
		},
		{
			name: "invalid_condition_log",
			input: &Config{
				Logs: map[string]MetricInfo{
					defaultMetricNameLogs: {
						Description: defaultMetricDescSpans,
						Conditions:  []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("logs condition: metric %q: unable to parse OTTL statement", defaultMetricNameLogs),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expect)
		})
	}
}

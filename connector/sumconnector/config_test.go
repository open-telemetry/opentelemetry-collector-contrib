// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumconnector

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/sumconnector/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	testCases := []struct {
		name   string
		expect *Config
	}{
		{
			name: "custom_description",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.sum": {
						Description:     "My span record sum.",
						SourceAttribute: "my.attribute",
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						Description:     "My spanevent sum.",
						SourceAttribute: "my.attribute",
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						Description:     "My metric sum.",
						SourceAttribute: "my.attribute",
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						Description:     "My datapoint sum.",
						SourceAttribute: "my.attribute",
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						Description:     "My log sum.",
						SourceAttribute: "my.attribute",
					},
				},
			},
		},
		{
			name: "custom_metric",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.sum": {
						Description: "My span sum.",
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						Description: "My span event sum.",
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						Description: "My metric sum.",
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						Description: "My data point sum.",
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						Description: "My log record sum.",
					},
				},
			},
		},
		{
			name: "condition",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.sum": {
						Description: "My span sum.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-s")`},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						Description: "My span event sum.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-e")`},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						Description: "My metric sum.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-m")`},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						Description: "My data point sum.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-d")`},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						Description: "My log record sum.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-l")`},
					},
				},
			},
		},
		{
			name: "multiple_condition",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.sum": {
						Description: "My span sum.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-s")`,
							`IsMatch(resource.attributes["foo"], "bar-s")`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						Description: "My span event sum.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-e")`,
							`IsMatch(resource.attributes["foo"], "bar-e")`,
						},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						Description: "My metric sum.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-m")`,
							`IsMatch(resource.attributes["foo"], "bar-m")`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						Description: "My data point sum.",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-d")`,
							`IsMatch(resource.attributes["foo"], "bar-d")`,
						},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						Description: "My log record sum.",
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
					"my.span.sum": {
						Description: "My span sum by environment.",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						Description: "My span event sum by environment.",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						Description: "My metric sum.",
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						Description: "My data point sum by environment.",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						Description: "My log record sum by environment.",
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
					"my.span.sum": {
						Description: "My span sum.",
					},
					"limited.span.sum": {
						Description: "Limited span sum.",
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
					"my.spanevent.sum": {
						Description: "My span event sum.",
					},
					"limited.spanevent.sum": {
						Description: "Limited span event sum.",
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
					"my.metric.sum": {
						Description: "My metric sum."},
					"limited.metric.sum": {
						Description: "Limited metric sum.",
						Conditions:  []string{`IsMatch(resource.attributes["host.name"], "pod-m")`},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						Description: "My data point sum.",
					},
					"limited.datapoint.sum": {
						Description: "Limited data point sum.",
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
					"my.logrecord.sum": {
						Description: "My log record sum.",
					},
					"limited.logrecord.sum": {
						Description: "Limited log record sum.",
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
			require.NoError(t, sub.Unmarshal(cfg))

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
						SourceAttribute: "my.attribute",
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
						SourceAttribute: "my.attribute",
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
						SourceAttribute: "my.attribute",
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
						SourceAttribute: "my.attribute",
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
						SourceAttribute: "my.attribute",
					},
				},
			},
			expect: "logs: metric name missing",
		},
		{
			name: "invalid_condition_span",
			input: &Config{
				Spans: map[string]MetricInfo{
					"metric.name.spans": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("spans condition: metric %q: unable to parse OTTL condition", "metric.name.spans"),
		},
		{
			name: "invalid_condition_spanevent",
			input: &Config{
				SpanEvents: map[string]MetricInfo{
					"metric.name.spanevents": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("spanevents condition: metric %q: unable to parse OTTL condition", "metric.name.spanevents"),
		},
		{
			name: "invalid_condition_metric",
			input: &Config{
				Metrics: map[string]MetricInfo{
					"metric.name.metrics": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("metrics condition: metric %q: unable to parse OTTL condition", "metric.name.metrics"),
		},
		{
			name: "invalid_condition_datapoint",
			input: &Config{
				DataPoints: map[string]MetricInfo{
					"metric.name.datapoints": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("datapoints condition: metric %q: unable to parse OTTL condition", "metric.name.datapoints"),
		},
		{
			name: "invalid_condition_log",
			input: &Config{
				Logs: map[string]MetricInfo{
					"metric.name.logs": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{"invalid condition"},
					},
				},
			},
			expect: fmt.Sprintf("logs condition: metric %q: unable to parse OTTL condition", "metric.name.logs"),
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

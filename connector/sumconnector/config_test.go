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
						SourceAttribute: "my.attribute",
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						SourceAttribute: "my.attribute",
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						SourceAttribute: "my.attribute",
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						SourceAttribute: "my.attribute",
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						SourceAttribute: "my.attribute",
					},
				},
			},
		},
		{
			name: "condition",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.sum": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-s")`},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-e")`},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-m")`},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-d")`},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-l")`},
					},
				},
			},
		},
		{
			name: "multiple_condition",
			expect: &Config{
				Spans: map[string]MetricInfo{
					"my.span.sum": {
						SourceAttribute: "my.attribute",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-s")`,
							`IsMatch(resource.attributes["foo"], "bar-s")`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						SourceAttribute: "my.attribute",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-e")`,
							`IsMatch(resource.attributes["foo"], "bar-e")`,
						},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						SourceAttribute: "my.attribute",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-m")`,
							`IsMatch(resource.attributes["foo"], "bar-m")`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						SourceAttribute: "my.attribute",
						Conditions: []string{
							`IsMatch(resource.attributes["host.name"], "pod-d")`,
							`IsMatch(resource.attributes["foo"], "bar-d")`,
						},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						SourceAttribute: "my.attribute",
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
						SourceAttribute: "my.attribute",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"my.spanevent.sum": {
						SourceAttribute: "my.attribute",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				Metrics: map[string]MetricInfo{
					"my.metric.sum": {
						SourceAttribute: "my.attribute",
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						SourceAttribute: "my.attribute",
						Attributes: []AttributeConfig{
							{Key: "env"},
						},
					},
				},
				Logs: map[string]MetricInfo{
					"my.logrecord.sum": {
						SourceAttribute: "my.attribute",
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
						Description:     "My span sum.",
						SourceAttribute: "my.attribute",
					},
					"limited.span.sum": {
						Description:     "Limited span sum.",
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-s")`},
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
						Description:     "My span event sum.",
						SourceAttribute: "my.attribute",
					},
					"limited.spanevent.sum": {
						Description:     "Limited span event sum.",
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-e")`},
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
						Description:     "My metric sum.",
						SourceAttribute: "my.attribute",
					},
					"limited.metric.sum": {
						Description:     "Limited metric sum.",
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-m")`},
					},
				},
				DataPoints: map[string]MetricInfo{
					"my.datapoint.sum": {
						Description:     "My data point sum.",
						SourceAttribute: "my.attribute",
					},
					"limited.datapoint.sum": {
						Description:     "Limited data point sum.",
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-d")`},
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
						Description:     "My log record sum.",
						SourceAttribute: "my.attribute",
					},
					"limited.logrecord.sum": {
						Description:     "Limited log record sum.",
						SourceAttribute: "my.attribute",
						Conditions:      []string{`IsMatch(resource.attributes["host.name"], "pod-l")`},
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
			name: "missing_source_attribute_span",
			input: &Config{
				Spans: map[string]MetricInfo{
					"span.missing.source.attribute": {},
				},
			},
			expect: "spans: metric source_attribute missing",
		},
		{
			name: "missing_source_attribute_spanevent",
			input: &Config{
				SpanEvents: map[string]MetricInfo{
					"spanevent.missing.source.attribute": {},
				},
			},
			expect: "spanevents: metric source_attribute missing",
		},
		{
			name: "missing_source_attribute_metric",
			input: &Config{
				Metrics: map[string]MetricInfo{
					"metric.missing.source.attribute": {},
				},
			},
			expect: "metrics: metric source_attribute missing",
		},
		{
			name: "missing_source_attribute_datapoint",
			input: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.missing.source.attribute": {},
				},
			},
			expect: "datapoints: metric source_attribute missing",
		},
		{
			name: "missing_source_attribute_log",
			input: &Config{
				Logs: map[string]MetricInfo{
					"log.missing.source.attribute": {},
				},
			},
			expect: "logs: metric source_attribute missing",
		},
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
		{
			name: "multi_error_span",
			input: &Config{
				Spans: map[string]MetricInfo{
					"": {
						SourceAttribute: "",
						Conditions:      []string{"invalid condition"},
						Attributes: []AttributeConfig{
							{Key: ""},
						},
					},
				},
			},
			expect: `spans: metric name missing` + "\n" + `spans: metric source_attribute missing` + "\n" + `spans condition: metric "": unable to parse OTTL condition "invalid condition": condition has invalid syntax: 1:9: unexpected token "condition" (expected <opcomparison> Value)` + "\n" + `spans attributes: metric "": attribute key missing`,
		},
		{
			name: "multi_error_spanevent",
			input: &Config{
				SpanEvents: map[string]MetricInfo{
					"": {
						SourceAttribute: "",
						Conditions:      []string{"invalid condition"},
						Attributes: []AttributeConfig{
							{Key: ""},
						},
					},
				},
			},
			expect: `spanevents: metric name missing` + "\n" + `spanevents: metric source_attribute missing` + "\n" + `spanevents condition: metric "": unable to parse OTTL condition "invalid condition": condition has invalid syntax: 1:9: unexpected token "condition" (expected <opcomparison> Value)` + "\n" + `spanevents attributes: metric "": attribute key missing`,
		},
		{
			name: "multi_error_metric",
			input: &Config{
				Metrics: map[string]MetricInfo{
					"": {
						SourceAttribute: "",
						Conditions:      []string{"invalid condition"},
						Attributes: []AttributeConfig{
							{Key: ""},
						},
					},
				},
			},
			expect: `metrics: metric name missing` + "\n" + `metrics: metric source_attribute missing` + "\n" + `metrics condition: metric "": unable to parse OTTL condition "invalid condition": condition has invalid syntax: 1:9: unexpected token "condition" (expected <opcomparison> Value)` + "\n" + `metrics attributes not supported: metric ""`,
		},
		{
			name: "multi_error_datapoint",
			input: &Config{
				DataPoints: map[string]MetricInfo{
					"": {
						SourceAttribute: "",
						Conditions:      []string{"invalid condition"},
						Attributes: []AttributeConfig{
							{Key: ""},
						},
					},
				},
			},
			expect: `datapoints: metric name missing` + "\n" + `datapoints: metric source_attribute missing` + "\n" + `datapoints condition: metric "": unable to parse OTTL condition "invalid condition": condition has invalid syntax: 1:9: unexpected token "condition" (expected <opcomparison> Value)` + "\n" + `datapoints attributes: metric "": attribute key missing`,
		},
		{
			name: "multi_error_log",
			input: &Config{
				Logs: map[string]MetricInfo{
					"": {
						SourceAttribute: "",
						Conditions:      []string{"invalid condition"},
						Attributes: []AttributeConfig{
							{Key: ""},
						},
					},
				},
			},
			expect: `logs: metric name missing` + "\n" + `logs: metric source_attribute missing` + "\n" + `logs condition: metric "": unable to parse OTTL condition "invalid condition": condition has invalid syntax: 1:9: unexpected token "condition" (expected <opcomparison> Value)` + "\n" + `logs attributes: metric "": attribute key missing`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			assert.ErrorContains(t, err, tc.expect)
		})
	}
}

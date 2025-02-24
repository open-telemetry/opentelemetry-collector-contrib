// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumconnector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/sumconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

// The test input file has a repetitive structure:
// - There are four resources, each with four spans, each with four span events.
// - The four resources have the following sets of attributes:
//   - resource.required: foo, resource.optional: bar
//   - resource.required: foo, resource.optional: notbar
//   - resource.required: notfoo
//   - (no attributes)
//
// - The four spans on each resource have the following sets of attributes:
//   - span.required: foo, span.optional: bar
//   - span.required: foo, span.optional: notbar
//   - span.required: notfoo
//   - (no attributes)
//
// - The four span events on each span have the following sets of attributes:
//   - event.required: foo, event.optional: bar
//   - event.required: foo, event.optional: notbar
//   - event.required: notfoo
//   - (no attributes)
func TestTracesToMetrics(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "zero_conditions",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"trace.span.sum": {
						Description:     "The sum of beep values observed in spans.",
						SourceAttribute: "beep",
					},
				},
				SpanEvents: map[string]MetricInfo{
					"trace.span.event.sum": {
						Description:     "The sum of beep values observed in span events.",
						SourceAttribute: "beep",
					},
				},
			},
		},
		{
			name: "one_condition",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.sum.if": {
						Description:     "Span sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.sum.if": {
						Description:     "Span event sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "one_attribute",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.sum.by_attr": {
						Description:     "Span sum by attribute",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "span.required",
							},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.sum.by_attr": {
						Description:     "Span event sum by attribute",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "event.required",
							},
						},
					},
				},
			},
		},
		{
			name: "multiple_conditions",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.sum.if": {
						Description:     "Span sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["span.optional"] != nil`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.sum.if": {
						Description:     "Span event sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["event.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_attributes",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.sum.by_attr": {
						Description:     "Span sum by attributes",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "span.required",
							},
							{
								Key: "span.optional",
							},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.sum.by_attr": {
						Description:     "Span event sum by attributes",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "event.required",
							},
							{
								Key: "event.optional",
							},
						},
					},
				},
			},
		},
		{
			name: "multiple_metrics",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.sum.all": {
						Description:     "All spans sum",
						SourceAttribute: "beep",
					},
					"span.sum.if": {
						Description:     "Span sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["span.optional"] != nil`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.sum.all": {
						Description:     "All span events sum",
						SourceAttribute: "beep",
					},
					"spanevent.sum.if": {
						Description:     "Span event sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["event.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "condition_and_attribute",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.sum.if.by_attr": {
						Description:     "Span sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
						Attributes: []AttributeConfig{
							{
								Key: "span.required",
							},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.sum.if.by_attr": {
						Description:     "Span event sum by attribute if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
						Attributes: []AttributeConfig{
							{
								Key: "event.required",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.cfg.Validate())
			factory := NewFactory()
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateTracesToMetrics(context.Background(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testSpans, err := golden.ReadTraces(filepath.Join("testdata", "traces", "input.yaml"))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeTraces(context.Background(), testSpans))

			allMetrics := sink.AllMetrics()
			assert.Len(t, allMetrics, 1)

			expected, err := golden.ReadMetrics(filepath.Join("testdata", "traces", tc.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricFloatPrecision(3),
				pmetrictest.IgnoreMetricDataPointsOrder()))
		})
	}
}

// The test input file has a repetitive structure:
// - There are four resources, each with six metrics, each with four data points.
// - The four resources have the following sets of attributes:
//   - resource.required: foo, resource.optional: bar
//   - resource.required: foo, resource.optional: notbar
//   - resource.required: notfoo
//   - (no attributes)
//
// - The size metrics have the following sets of types:
//   - int gauge, double gauge, int sum, double sum, histogram, summary
//
// - The four data points on each metric have the following sets of attributes:
//   - datapoint.required: foo, datapoint.optional: bar
//   - datapoint.required: foo, datapoint.optional: notbar
//   - datapoint.required: notfoo
//   - (no attributes)
func TestMetricsToMetrics(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "one_attribute",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.sum.by_attr": {
						Description:     "Data point sum by attribute",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "datapoint.required",
							},
						},
					},
				},
			},
		},
		{
			name: "one_condition",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.sum.if": {
						Description:     "Data point sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_conditions",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.sum.if": {
						Description:     "Data point sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["datapoint.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_metrics",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.sum.all": {
						Description:     "All data points sum",
						SourceAttribute: "beep",
					},
					"datapoint.sum.if": {
						Description:     "Data point sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["datapoint.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_attributes",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.sum.by_attr": {
						Description:     "Data point sum by attributes",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "datapoint.required",
							},
							{
								Key: "datapoint.optional",
							},
						},
					},
				},
			},
		},
		{
			name: "default_attribute_value",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.sum.by_attr": {
						Description:     "Data point sum by attribute with default",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "datapoint.required",
							},
							{
								Key:          "datapoint.optional",
								DefaultValue: "other",
							},
						},
					},
				},
			},
		},
		{
			name: "condition_and_attribute",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.sum.if.by_attr": {
						Description:     "Data point sum by attribute if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
						Attributes: []AttributeConfig{
							{
								Key: "datapoint.required",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.cfg.Validate())
			factory := NewFactory()
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateMetricsToMetrics(context.Background(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testMetrics, err := golden.ReadMetrics(filepath.Join("testdata", "metrics", "input.yaml"))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeMetrics(context.Background(), testMetrics))

			allMetrics := sink.AllMetrics()
			assert.Len(t, allMetrics, 1)

			expected, err := golden.ReadMetrics(filepath.Join("testdata", "metrics", tc.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricFloatPrecision(3),
				pmetrictest.IgnoreMetricDataPointsOrder()))
		})
	}
}

// The test input file has a repetitive structure:
// - There are four resources, each with four logs.
// - The four resources have the following sets of attributes:
//   - resource.required: foo, resource.optional: bar
//   - resource.required: foo, resource.optional: notbar
//   - resource.required: notfoo
//   - (no attributes)
//
// - The four logs on each resource have the following sets of attributes:
//   - log.required: foo, log.optional: bar
//   - log.required: foo, log.optional: notbar
//   - log.required: notfoo
//   - (no attributes)
func TestLogsToMetrics(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "one_attribute",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"log.sum.by_attr": {
						Description:     "Log sum by attribute",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "log.required",
							},
						},
					},
				},
			},
		},
		{
			name: "one_condition",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"sum.if": {
						Description:     "Sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_conditions",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"sum.if": {
						Description:     "Sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["log.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_metrics",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"sum.all": {
						Description:     "All logs Sum",
						SourceAttribute: "beep",
					},
					"sum.if": {
						Description:     "Sum if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_attributes",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"log.sum.by_attr": {
						Description:     "Log sum by attributes",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "log.required",
							},
							{
								Key: "log.optional",
							},
						},
					},
				},
			},
		},
		{
			name: "default_attribute_value",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"log.sum.by_attr": {
						Description:     "Log sum by attribute with default",
						SourceAttribute: "beep",
						Attributes: []AttributeConfig{
							{
								Key: "log.required",
							},
							{
								Key:          "log.optional",
								DefaultValue: "other",
							},
						},
					},
				},
			},
		},
		{
			name: "condition_and_attribute",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"log.sum.if.by_attr": {
						Description:     "Log sum by attribute if ...",
						SourceAttribute: "beep",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
						Attributes: []AttributeConfig{
							{
								Key: "log.required",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.cfg.Validate())
			factory := NewFactory()
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateLogsToMetrics(context.Background(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testLogs, err := golden.ReadLogs(filepath.Join("testdata", "logs", "input.yaml"))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs))

			allMetrics := sink.AllMetrics()
			assert.Len(t, allMetrics, 1)

			expected, err := golden.ReadMetrics(filepath.Join("testdata", "logs", tc.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricFloatPrecision(3),
				pmetrictest.IgnoreMetricDataPointsOrder()))
		})
	}
}

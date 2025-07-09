// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package countconnector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/connector/xconnector"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector/internal/metadata"
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
				Spans:      defaultSpansConfig(),
				SpanEvents: defaultSpanEventsConfig(),
			},
		},
		{
			name: "one_condition",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.count.if": {
						Description: "Span count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.count.if": {
						Description: "Span event count if ...",
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
				Spans: map[string]MetricInfo{
					"span.count.if": {
						Description: "Span count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["span.optional"] != nil`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.count.if": {
						Description: "Span event count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["event.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "multiple_metrics",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.count.all": {
						Description: "All spans count",
					},
					"span.count.if": {
						Description: "Span count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["span.optional"] != nil`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.count.all": {
						Description: "All span events count",
					},
					"spanevent.count.if": {
						Description: "Span event count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["event.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "one_attribute",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.count.by_attr": {
						Description: "Span count by attribute",
						Attributes: []AttributeConfig{
							{
								Key: "span.required",
							},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.count.by_attr": {
						Description: "Span event count by attribute",
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
			name: "multiple_attributes",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.count.by_attr": {
						Description: "Span count by attributes",
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
					"spanevent.count.by_attr": {
						Description: "Span event count by attributes",
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
			name: "default_attribute_value",
			cfg: &Config{
				Spans: map[string]MetricInfo{
					"span.count.by_attr": {
						Description: "Span count by attribute with default",
						Attributes: []AttributeConfig{
							{
								Key: "span.required",
							},
							{
								Key:          "span.optional",
								DefaultValue: "other",
							},
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.count.by_attr": {
						Description: "Span event count by attribute with default",
						Attributes: []AttributeConfig{
							{
								Key: "event.required",
							},
							{
								Key:          "event.optional",
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
				Spans: map[string]MetricInfo{
					"span.count.if.by_attr": {
						Description: "Span count if ...",
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
					"spanevent.count.if.by_attr": {
						Description: "Span event count by attribute if ...",
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

			// golden.WriteMetrics(t, filepath.Join("testdata", "traces", tc.name+".yaml"), allMetrics[0])
			expected, err := golden.ReadMetrics(filepath.Join("testdata", "traces", tc.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder()))
		})
	}
}

// The test input file has a repetitive structure:
// - There are four resources, each with six metrics, each with four data point.
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
			name: "zero_conditions",
			cfg: &Config{
				Metrics:    defaultMetricsConfig(),
				DataPoints: defaultDataPointsConfig(),
			},
		},
		{
			name: "one_condition",
			cfg: &Config{
				Metrics: map[string]MetricInfo{
					"metric.count.if": {
						Description: "Metric count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"datapoint.count.if": {
						Description: "Data point count if ...",
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
				Metrics: map[string]MetricInfo{
					"metric.count.if": {
						Description: "Metric count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`type == METRIC_DATA_TYPE_HISTOGRAM`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"datapoint.count.if": {
						Description: "Data point count if ...",
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
				Metrics: map[string]MetricInfo{
					"metric.count.all": {
						Description: "All metrics count",
					},
					"metric.count.if": {
						Description: "Metric count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`type == METRIC_DATA_TYPE_HISTOGRAM`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"datapoint.count.all": {
						Description: "All data points count",
					},
					"datapoint.count.if": {
						Description: "Data point count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`attributes["datapoint.optional"] != nil`,
						},
					},
				},
			},
		},
		{
			name: "one_attribute",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.count.by_attr": {
						Description: "Data point count by attribute",
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
			name: "multiple_attributes",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.count.by_attr": {
						Description: "Data point count by attributes",
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
					"datapoint.count.by_attr": {
						Description: "Data point count by attribute with default",
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
					"datapoint.count.if.by_attr": {
						Description: "Data point count by attribute if ...",
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
		{
			name: "int_attribute_value",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.count.by_attr": {
						Description: "Data point count by int attribute",
						Attributes: []AttributeConfig{
							{
								Key: "datapoint.int",
							},
						},
					},
				},
			},
		},
		{
			name: "default_int_attribute_value",
			cfg: &Config{
				DataPoints: map[string]MetricInfo{
					"datapoint.count.by_attr": {
						Description: "Data point count by default int attribute",
						Attributes: []AttributeConfig{
							{
								Key: "datapoint.int",
							},
							{
								Key:          "datapoint.optional_int",
								DefaultValue: 10,
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

			// golden.WriteMetrics(t, filepath.Join("testdata", "metrics", tc.name+".yaml"), allMetrics[0])
			expected, err := golden.ReadMetrics(filepath.Join("testdata", "metrics", tc.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
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
			name: "zero_conditions",
			cfg:  &Config{Logs: defaultLogsConfig()},
		},
		{
			name: "one_condition",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"count.if": {
						Description: "Count if ...",
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
					"count.if": {
						Description: "Count if ...",
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
					"count.all": {
						Description: "All logs count",
					},
					"count.if": {
						Description: "Count if ...",
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
				Logs: map[string]MetricInfo{
					"log.count.by_attr": {
						Description: "Log count by attribute",
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
			name: "multiple_attributes",
			cfg: &Config{
				Logs: map[string]MetricInfo{
					"log.count.by_attr": {
						Description: "Log count by attributes",
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
					"log.count.by_attr": {
						Description: "Log count by attribute with default",
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
					"log.count.if.by_attr": {
						Description: "Log count by attribute if ...",
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

			// golden.WriteMetrics(t, filepath.Join("testdata", "logs", tc.name+".yaml"), allMetrics[0])
			expected, err := golden.ReadMetrics(filepath.Join("testdata", "logs", tc.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder()))
		})
	}
}

// The test input file has a repetitive structure:
// - There are four resources, each with four profiles, each with one sample.
// - The four resources have the following sets of attributes:
//   - resource.required: foo, resource.optional: bar
//   - resource.required: foo, resource.optional: notbar
//   - resource.required: notfoo
//   - (no attributes)
//
// - The four profiles on each resource have the following sets of attributes:
//   - profile.required: foo, profile.optional: bar
//   - profile.required: foo, profile.optional: notbar
//   - profile.required: notfoo
//   - (no attributes)
func TestProfilesToMetrics(t *testing.T) {
	testCases := []struct {
		name string
		cfg  *Config
	}{
		{
			name: "zero_conditions",
			cfg:  &Config{Profiles: defaultProfilesConfig()},
		},
		{
			name: "one_condition",
			cfg: &Config{
				Profiles: map[string]MetricInfo{
					"count.if": {
						Description: "Count if ...",
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
				Profiles: map[string]MetricInfo{
					"count.if": {
						Description: "Count if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
							`profile.duration_unix_nano > 1000`,
						},
					},
				},
			},
		},
		{
			name: "multiple_metrics",
			cfg: &Config{
				Profiles: map[string]MetricInfo{
					"count.all": {
						Description: "All profiles count",
					},
					"count.if": {
						Description: "Count if ...",
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
				Profiles: map[string]MetricInfo{
					"profile.count.by_attr": {
						Description: "Profile count by attribute",
						Attributes: []AttributeConfig{
							{
								Key: "profile.required",
							},
						},
					},
				},
			},
		},
		{
			name: "multiple_attributes",
			cfg: &Config{
				Profiles: map[string]MetricInfo{
					"profile.count.by_attr": {
						Description: "Profile count by attributes",
						Attributes: []AttributeConfig{
							{
								Key: "profile.required",
							},
							{
								Key: "profile.optional",
							},
						},
					},
				},
			},
		},
		{
			name: "default_attribute_value",
			cfg: &Config{
				Profiles: map[string]MetricInfo{
					"profile.count.by_attr": {
						Description: "Profile count by attribute with default",
						Attributes: []AttributeConfig{
							{
								Key: "profile.required",
							},
							{
								Key:          "profile.optional",
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
				Profiles: map[string]MetricInfo{
					"profile.count.if.by_attr": {
						Description: "Profile count by attribute if ...",
						Conditions: []string{
							`resource.attributes["resource.optional"] != nil`,
						},
						Attributes: []AttributeConfig{
							{
								Key: "profile.required",
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
			factory := NewFactory().(xconnector.Factory)
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateProfilesToMetrics(context.Background(),
				connectortest.NewNopSettings(metadata.Type), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testProfiles, err := golden.ReadProfiles(filepath.Join("testdata", "profiles", "input.yaml"))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeProfiles(context.Background(), testProfiles))

			allMetrics := sink.AllMetrics()
			assert.Len(t, allMetrics, 1)

			// golden.WriteMetrics(t, filepath.Join("testdata", "profiles", tc.name+".yaml"), allMetrics[0])
			expected, err := golden.ReadMetrics(filepath.Join("testdata", "profiles", tc.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder()))
		})
	}
}

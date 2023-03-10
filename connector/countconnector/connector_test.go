// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package countconnector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

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
							`resource.attributes["resource-attr"] != "resource-attr-val-1"`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.count.if": {
						Description: "Span event count if ...",
						Conditions: []string{
							`resource.attributes["resource-attr"] == "resource-attr-val-1"`,
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
							`resource.attributes["resource-attr"] != "resource-attr-val-1"`,
							`name == "operationB"`,
						},
					},
				},
				SpanEvents: map[string]MetricInfo{
					"spanevent.count.if": {
						Description: "Span event count if ...",
						Conditions: []string{
							`resource.attributes["resource-attr"] != "resource-attr-val-1"`,
							`name == "event-with-attr"`,
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
							`resource.attributes["resource-attr"] != "resource-attr-val-1"`,
							`name == "operationB"`,
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
							`resource.attributes["resource-attr"] != "resource-attr-val-1"`,
							`name == "event-with-attr"`,
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
				connectortest.NewNopCreateSettings(), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testSpans, err := golden.ReadTraces(filepath.Join("testdata", "traces", "input.json"))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeTraces(context.Background(), testSpans))

			allMetrics := sink.AllMetrics()
			assert.Equal(t, 1, len(allMetrics))

			// golden.WriteMetrics(t, filepath.Join("testdata", "traces", tc.name+".json"), allMetrics[0])
			expected, err := golden.ReadMetrics(filepath.Join("testdata", "traces", tc.name+".json"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreMetricsOrder()))
		})
	}
}

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
							`resource.attributes["resource-attr-2"] != nil`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"datapoint.count.if": {
						Description: "Data point count if ...",
						Conditions: []string{
							`resource.attributes["resource-attr-2"] == nil`,
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
							`resource.attributes["resource-attr-2"] != nil`,
							`type == METRIC_DATA_TYPE_HISTOGRAM`,
						},
					},
				},
				DataPoints: map[string]MetricInfo{
					"datapoint.count.if": {
						Description: "Data point count if ...",
						Conditions: []string{
							`resource.attributes["resource-attr-2"] == nil`,
							`value_int == 123`,
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
							`resource.attributes["resource-attr-2"] != nil`,
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
							`resource.attributes["resource-attr-2"] == nil`,
							`value_int == 123`,
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
				connectortest.NewNopCreateSettings(), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testMetrics, err := golden.ReadMetrics(filepath.Join("testdata", "metrics", "input.json"))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeMetrics(context.Background(), testMetrics))

			allMetrics := sink.AllMetrics()
			assert.Equal(t, 1, len(allMetrics))

			// golden.WriteMetrics(t, filepath.Join("testdata", "metrics", tc.name+".json"), allMetrics[0])
			expected, err := golden.ReadMetrics(filepath.Join("testdata", "metrics", tc.name+".json"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreMetricsOrder()))
		})
	}
}

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
							`resource.attributes["resource-attr-2"] != nil`,
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
							`resource.attributes["resource-attr-2"] != nil`,
							`attributes["customer"] == "acme"`,
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
							`resource.attributes["resource-attr-2"] != nil`,
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
				connectortest.NewNopCreateSettings(), tc.cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testLogs, err := golden.ReadLogs(filepath.Join("testdata", "logs", "input.json"))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs))

			allMetrics := sink.AllMetrics()
			assert.Equal(t, 1, len(allMetrics))

			// golden.WriteMetrics(t, filepath.Join("testdata", "logs", tc.name+".json"), allMetrics[0])
			expected, err := golden.ReadMetrics(filepath.Join("testdata", "logs", tc.name+".json"))
			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0],
				pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreMetricsOrder()))
		})
	}
}

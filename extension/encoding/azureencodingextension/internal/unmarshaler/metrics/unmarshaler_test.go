// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const testFilesDirectory = "testdata"

var testBuildInfo = component.BuildInfo{
	Version: "test-version",
}

// Samples are taken from:
// https://learn.microsoft.com/en-us/azure/azure-monitor/platform/stream-monitoring-data-event-hubs#data-formats for Diagnostic Settings
// https://learn.microsoft.com/en-us/azure/azure-monitor/data-collection/data-collection-metrics?tabs=event-hubs#event-hubs-2 for DCRs

func TestResourceMetricsUnmarshaler_UnmarshalMetrics_Golden(t *testing.T) {
	t.Parallel()

	testFiles, err := filepath.Glob(filepath.Join(testFilesDirectory, "*.json"))
	require.NoError(t, err)

	for _, testFile := range testFiles {
		testName := strings.TrimSuffix(filepath.Base(testFile), ".json")

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			observedZapCore, observedLogs := observer.New(zap.WarnLevel)
			unmarshaler := NewAzureResourceMetricsUnmarshaler(
				testBuildInfo,
				zap.New(observedZapCore),
				MetricsConfig{},
			)

			data, err := os.ReadFile(testFile)
			require.NoError(t, err)

			metrics, err := unmarshaler.UnmarshalMetrics(data)
			require.NoError(t, err)

			require.Equal(t, 0, observedLogs.Len(), "Unexpected error/warn logs: %+v", observedLogs.All())

			expectedMetrics, err := golden.ReadMetrics(filepath.Join(testFilesDirectory, fmt.Sprintf("%s_expected.yaml", testName)))
			require.NoError(t, err)

			compareOptions := []pmetrictest.CompareMetricsOption{
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
			}
			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, metrics, compareOptions...))
		})
	}
}

func TestResourceMetricsUnmarshaler_AggregatePresence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		aggregations         []MetricAggregation
		aggregateFields      string
		expectedMetricValues map[string]float64
	}{
		{
			name:            "omitted and null aggregates",
			aggregateFields: `"count": 1, "total": 2, "minimum": null, "average": 2`,
			expectedMetricValues: map[string]float64{
				"requests_count":   1,
				"requests_total":   2,
				"requests_average": 2,
			},
		},
		{
			name:                 "explicit zero aggregates",
			aggregations:         []MetricAggregation{AggregationMinimum, AggregationMaximum},
			aggregateFields:      `"minimum": 0, "maximum": 0`,
			expectedMetricValues: map[string]float64{"requests_minimum": 0, "requests_maximum": 0},
		},
		{
			name:                 "configured missing aggregate",
			aggregations:         []MetricAggregation{AggregationMinimum, AggregationTotal},
			aggregateFields:      `"total": 0`,
			expectedMetricValues: map[string]float64{"requests_total": 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			unmarshaler := NewAzureResourceMetricsUnmarshaler(
				testBuildInfo,
				zap.NewNop(),
				MetricsConfig{Aggregations: tt.aggregations},
			)
			data := []byte(fmt.Sprintf(`{
				"records": [{
					%s,
					"resourceId": "/subscriptions/000/resourceGroups/rg/providers/Microsoft.Network/loadBalancers/lb",
					"time": "2026-08-25T00:00:00Z",
					"metricName": "Requests",
					"timeGrain": "PT1M"
				}]
			}`, tt.aggregateFields))

			got, err := unmarshaler.UnmarshalMetrics(data)
			require.NoError(t, err)
			require.Equal(t, 1, got.ResourceMetrics().Len())

			gotMetrics := got.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			require.Equal(t, len(tt.expectedMetricValues), gotMetrics.Len())
			for i := 0; i < gotMetrics.Len(); i++ {
				metric := gotMetrics.At(i)
				expectedValue, ok := tt.expectedMetricValues[metric.Name()]
				require.True(t, ok, "unexpected metric %q", metric.Name())
				require.Equal(t, expectedValue, metric.Gauge().DataPoints().At(0).DoubleValue())
			}
		})
	}
}

func TestResourceMetricsUnmarshaler_UnmarshalMetrics(t *testing.T) {
	t.Parallel()

	t.Run("Empty Records", func(t *testing.T) {
		t.Parallel()
		unmarshaler := NewAzureResourceMetricsUnmarshaler(
			component.BuildInfo{Version: "test-version"},
			zap.NewNop(),
			MetricsConfig{},
		)
		data := []byte(`{"records": []}`)
		metrics, err := unmarshaler.UnmarshalMetrics(data)
		require.NoError(t, err)
		require.Equal(t, 0, metrics.ResourceMetrics().Len())
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		t.Parallel()
		unmarshaler := NewAzureResourceMetricsUnmarshaler(
			component.BuildInfo{Version: "test-version"},
			zap.NewNop(),
			MetricsConfig{},
		)
		data := []byte(`{invalid-json}`)
		_, err := unmarshaler.UnmarshalMetrics(data)
		require.ErrorContains(t, err, "unable to detect JSON format from input:")
	})

	t.Run("Invalid Timestamp", func(t *testing.T) {
		t.Parallel()
		observedZapCore, observedLogs := observer.New(zap.WarnLevel)

		unmarshaler := NewAzureResourceMetricsUnmarshaler(
			component.BuildInfo{Version: "test-version"},
			zap.New(observedZapCore),
			MetricsConfig{},
		)
		data := []byte(`{
			"records": [{
				"count": 2,
				"total": 0.217,
				"minimum": 0.042,
				"maximum": 0.175,
				"average": 0.1085,
				"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.WEB/SITES/SCALEABLEWEBAPP1",
				"time": "invalid-timestamp",
				"metricName": "CpuTime",
				"timeGrain": "PT1M"
			}]
		}`)

		metrics, err := unmarshaler.UnmarshalMetrics(data)
		require.NoError(t, err)
		require.Equal(t, 1, observedLogs.FilterMessage("Unable to parse timestamp from Azure Metric").Len())
		require.Equal(t, 0, metrics.ResourceMetrics().Len())
	})

	t.Run("Invalid TimeGrain", func(t *testing.T) {
		t.Parallel()
		observedZapCore, observedLogs := observer.New(zap.WarnLevel)

		unmarshaler := NewAzureResourceMetricsUnmarshaler(
			component.BuildInfo{Version: "test-version"},
			zap.New(observedZapCore),
			MetricsConfig{},
		)
		data := []byte(`{
			"records": [{
				"count": 2,
				"total": 0.217,
				"minimum": 0.042,
				"maximum": 0.175,
				"average": 0.1085,
				"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.WEB/SITES/SCALEABLEWEBAPP1",
				"time": "2023-04-18T09:03:00.0000000Z",
				"metricName": "CpuTime",
				"timeGrain": "PTZ"
			}]
		}`)

		metrics, err := unmarshaler.UnmarshalMetrics(data)
		require.NoError(t, err)
		require.Equal(t, 1, observedLogs.FilterMessage("Unhandled Time Grain").Len())
		require.Equal(t, 0, metrics.ResourceMetrics().Len())
	})

	t.Run("Empty ResourceID", func(t *testing.T) {
		t.Parallel()
		observedZapCore, observedLogs := observer.New(zap.WarnLevel)

		unmarshaler := NewAzureResourceMetricsUnmarshaler(
			component.BuildInfo{Version: "test-version"},
			zap.New(observedZapCore),
			MetricsConfig{},
		)
		data := []byte(`{
			"records": [{
				"count": 2,
				"total": 0.217,
				"minimum": 0.042,
				"maximum": 0.175,
				"average": 0.1085,
				"time": "2023-04-18T09:03:00.0000000Z",
				"metricName": "CpuTime",
				"timeGrain": "PT1M"
			}]
		}`)

		metrics, err := unmarshaler.UnmarshalMetrics(data)
		require.NoError(t, err)
		require.Equal(t, 1, observedLogs.FilterMessage("No ResourceID set on Metrics record").Len())
		require.Equal(t, 1, metrics.ResourceMetrics().Len())
	})

	t.Run("Azure BlobStorage format", func(t *testing.T) {
		t.Parallel()
		unmarshaler := NewAzureResourceMetricsUnmarshaler(
			component.BuildInfo{Version: "test-version"},
			zap.NewNop(),
			MetricsConfig{},
		)
		data := []byte(`[{
			"count": 2,
			"total": 0.217,
			"minimum": 0.042,
			"maximum": 0.175,
			"average": 0.1085,
			"time": "2023-04-18T09:03:00.0000000Z",
			"metricName": "CpuTime",
			"timeGrain": "PT1M",
			"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.WEB/SITES/SCALEABLEWEBAPP1"
		}]`)

		metrics, err := unmarshaler.UnmarshalMetrics(data)
		require.NoError(t, err)
		require.Equal(t, 1, metrics.ResourceMetrics().Len())
	})
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestLogsToLogs2(t *testing.T) {
	testCases := []struct {
		name         string
		inputFile    string
		expectedFile string
		expectedLogs int
	}{
		{
			name:         "correct log metric",
			inputFile:    "input-log.yaml",
			expectedFile: "output-log.yaml",
			expectedLogs: 1,
		},
		{
			name:         "invalid log",
			inputFile:    "input-invalid-log.yaml",
			expectedLogs: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			sink := &consumertest.LogsSink{}
			conn, err := factory.CreateLogsToLogs(context.Background(),

				connectortest.NewNopSettings(), createDefaultConfig(), sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testLogs, err := golden.ReadLogs(filepath.Join("testdata", "logsToLogs", tc.inputFile))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs))

			allLogs := sink.AllLogs()
			assert.Len(t, allLogs, tc.expectedLogs)

			if tc.expectedLogs > 0 {
				// golden.WriteLogs(t, filepath.Join("testdata", "logsToLogs", tc.expectedFile), allLogs[0])
				expected, err := golden.ReadLogs(filepath.Join("testdata", "logsToLogs", tc.expectedFile))
				assert.NoError(t, err)
				assert.NoError(t, plogtest.CompareLogs(expected, allLogs[0]))
			}
		})
	}
}

func TestLogsToMetrics(t *testing.T) {
	testCases := []struct {
		name            string
		inputFile       string
		expectedFile    string
		expectedMetrics int
	}{
		{
			name:            "correct log metric",
			inputFile:       "input-metric.yaml",
			expectedFile:    "output-metric.yaml",
			expectedMetrics: 1,
		},
		{
			name:            "invalid metric",
			inputFile:       "input-invalid-metric.yaml",
			expectedMetrics: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			sink := &consumertest.MetricsSink{}
			conn, err := factory.CreateLogsToMetrics(context.Background(),

				connectortest.NewNopSettings(), createDefaultConfig(), sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testLogs, err := golden.ReadLogs(filepath.Join("testdata", "logsToMetrics", tc.inputFile))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs))

			allMetrics := sink.AllMetrics()
			assert.Len(t, allMetrics, tc.expectedMetrics)

			if tc.expectedMetrics > 0 {
				// golden.WriteMetrics(t, filepath.Join("testdata", "logsToMetrics", tc.expectedFile), allMetrics[0])
				expected, err := golden.ReadMetrics(filepath.Join("testdata", "logsToMetrics", tc.expectedFile))
				assert.NoError(t, err)
				assert.NoError(t, pmetrictest.CompareMetrics(expected, allMetrics[0]))
			}
		})
	}
}

func TestLogsToTraces(t *testing.T) {
	testCases := []struct {
		name           string
		inputFile      string
		expectedFile   string
		expectedTraces int
	}{
		{
			name:           "correct log trace",
			inputFile:      "input-trace.yaml",
			expectedFile:   "output-trace.yaml",
			expectedTraces: 1,
		},
		{
			name:           "invalid trace",
			inputFile:      "input-invalid-trace.yaml",
			expectedTraces: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			sink := &consumertest.TracesSink{}
			conn, err := factory.CreateLogsToTraces(context.Background(),

				connectortest.NewNopSettings(), createDefaultConfig(), sink)
			require.NoError(t, err)
			require.NotNil(t, conn)
			assert.False(t, conn.Capabilities().MutatesData)

			require.NoError(t, conn.Start(context.Background(), componenttest.NewNopHost()))
			defer func() {
				assert.NoError(t, conn.Shutdown(context.Background()))
			}()

			testLogs, err := golden.ReadLogs(filepath.Join("testdata", "logsToTraces", tc.inputFile))
			assert.NoError(t, err)
			assert.NoError(t, conn.ConsumeLogs(context.Background(), testLogs))

			allMetrics := sink.AllTraces()
			assert.Len(t, allMetrics, tc.expectedTraces)

			if tc.expectedTraces > 0 {
				// golden.WriteTraces(t, filepath.Join("testdata", "logsToTraces", tc.expectedFile), allMetrics[0])
				expected, err := golden.ReadTraces(filepath.Join("testdata", "logsToTraces", tc.expectedFile))
				assert.NoError(t, err)
				assert.NoError(t, ptracetest.CompareTraces(expected, allMetrics[0]))
			}
		})
	}
}

// This benchmark looks at how performance is affected when all three connectors are consuming logs (at the same time)
func BenchmarkConsumeLogs(b *testing.B) {
	inputlogs := "input-log.yaml"
	inputTraces := "input-trace.yaml"
	inputMetrics := "input-metric.yaml"

	factory := NewFactory()
	// initialize log -> log connector
	logsink := &consumertest.LogsSink{}
	logscon, _ := factory.CreateLogsToLogs(context.Background(),
		connectortest.NewNopSettings(), createDefaultConfig(), logsink)

	require.NoError(b, logscon.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(b, logscon.Shutdown(context.Background()))
	}()

	// initialize log -> traces connector
	tracesink := &consumertest.TracesSink{}
	traceconn, _ := factory.CreateLogsToTraces(context.Background(),
		connectortest.NewNopSettings(), createDefaultConfig(), tracesink)
	require.NoError(b, traceconn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(b, traceconn.Shutdown(context.Background()))
	}()

	// initialize log -> metric connector
	metricsink := &consumertest.MetricsSink{}
	metricconn, _ := factory.CreateLogsToMetrics(context.Background(),
		connectortest.NewNopSettings(), createDefaultConfig(), metricsink)
	require.NoError(b, metricconn.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(b, metricconn.Shutdown(context.Background()))
	}()

	testLogs, _ := golden.ReadLogs(filepath.Join("testdata", "logsToLogs", inputlogs))
	testTraces, _ := golden.ReadLogs(filepath.Join("testdata", "logsToTraces", inputTraces))
	testMetrics, _ := golden.ReadLogs(filepath.Join("testdata", "logsToMetrics", inputMetrics))

	for i := 0; i < b.N; i++ {
		assert.NoError(b, logscon.ConsumeLogs(context.Background(), testLogs))
		assert.NoError(b, traceconn.ConsumeLogs(context.Background(), testTraces))
		assert.NoError(b, metricconn.ConsumeLogs(context.Background(), testMetrics))
	}
}

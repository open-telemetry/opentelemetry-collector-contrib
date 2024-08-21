package otlpjsonconnector

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestLogsToTraces2(t *testing.T) {
	testCases := []struct {
		name         string
		inputFile    string
		expectedFile string
	}{
		{
			name:         "correct log trace",
			inputFile:    "input-trace.yaml",
			expectedFile: "output-trace.yaml",
		},
		{
			name:         "invalid trace",
			inputFile:    "input-invalid-trace.yaml",
			expectedFile: "output-invalid-trace.yaml",
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
			assert.Equal(t, 1, len(allMetrics))

			golden.WriteTraces(t, filepath.Join("testdata", "logsToTraces", tc.expectedFile), allMetrics[0])
			expected, err := golden.ReadTraces(filepath.Join("testdata", "logsToTraces", tc.expectedFile))
			assert.NoError(t, err)
			assert.NoError(t, ptracetest.CompareTraces(expected, allMetrics[0]))
		})
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var (
	mockedConsumedResourceWithType = func() pmetric.Metrics {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("opencensus.resourcetype", "host")
		rm.Resource().Attributes().PutStr("label-key", "label-value")
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("metric-name")
		m.SetDescription("metric-description")
		m.SetUnit("metric-unit")
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(0)
		return md
	}()

	mockedConsumedResourceEmpty = func() pmetric.Metrics {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("metric-name")
		m.SetDescription("metric-description")
		m.SetUnit("metric-unit")
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(0)
		return md
	}()
)

type resourceProcessorTestCase struct {
	name                    string
	resourceProcessorConfig string
	mockedConsumedMetrics   pmetric.Metrics
	expectedMetrics         pmetric.Metrics
}

func getResourceProcessorTestCases() []resourceProcessorTestCase {
	tests := []resourceProcessorTestCase{
		{
			name: "update_and_rename_existing_attributes",
			resourceProcessorConfig: `
  resource:
    attributes:
    - key: label-key
      value: new-label-value
      action: update
    - key: resource-type
      from_attribute: opencensus.resourcetype
      action: upsert
    - key: opencensus.resourcetype
      action: delete
`,
			mockedConsumedMetrics: mockedConsumedResourceWithType,
			expectedMetrics: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				rm := md.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("label-key", "new-label-value")
				rm.Resource().Attributes().PutStr("resource-type", "host")
				return md
			}(),
		},
		{
			name: "set_attribute_on_empty_resource",
			resourceProcessorConfig: `
  resource:
    attributes:
    - key: additional-label-key
      value: additional-label-value
      action: insert

`,
			mockedConsumedMetrics: mockedConsumedResourceEmpty,
			expectedMetrics: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				rm := md.ResourceMetrics().AppendEmpty()
				rm.Resource().Attributes().PutStr("additional-label-key", "additional-label-value")
				return md
			}(),
		},
	}

	return tests
}

func TestMetricResourceProcessor(t *testing.T) {
	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	tests := getResourceProcessorTestCases()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
			require.NoError(t, err)

			agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
			processors := []ProcessorNameAndConfigBody{
				{Name: "resource", Body: test.resourceProcessorConfig},
			}
			configStr := createConfigYaml(t, sender, receiver, resultDir, processors, nil)
			configCleanup, err := agentProc.PrepareConfig(t, configStr)
			require.NoError(t, err)
			defer configCleanup()

			options := testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10}
			dataProvider := testbed.NewPerfTestDataProvider(options)
			tc := testbed.NewTestCase(
				t,
				dataProvider,
				sender,
				receiver,
				agentProc,
				&testbed.PerfTestValidator{},
				performanceResultsSummary,
			)
			defer tc.Stop()

			tc.StartBackend()
			tc.StartAgent()
			defer tc.StopAgent()

			tc.EnableRecording()

			require.NoError(t, sender.Start())

			// Clear previously received metrics.
			tc.MockBackend.ClearReceivedItems()
			startCounter := tc.MockBackend.DataItemsReceived()

			sender, ok := tc.LoadGenerator.(*testbed.ProviderSender).Sender.(testbed.MetricDataSender)
			require.True(t, ok, "unsupported metric sender")

			require.NoError(t, sender.ConsumeMetrics(context.Background(), test.mockedConsumedMetrics))

			// We bypass the load generator in this test, but make sure to increment the
			// counter since it is used in final reports.
			tc.LoadGenerator.IncDataItemsSent()

			tc.WaitFor(func() bool { return tc.MockBackend.DataItemsReceived() == startCounter+1 },
				"datapoints received")

			// Assert Resources
			m := tc.MockBackend.ReceivedMetrics[0]
			rm := m.ResourceMetrics()
			require.Equal(t, 1, rm.Len())

			expectidMD := test.expectedMetrics
			require.Equal(t,
				expectidMD.ResourceMetrics().At(0).Resource().Attributes().AsRaw(),
				rm.At(0).Resource().Attributes().AsRaw(),
			)
		})
	}
}

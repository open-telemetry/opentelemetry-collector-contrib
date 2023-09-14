package tests

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"github.com/stretchr/testify/require"
)

func TestSporadic(t *testing.T) {
	factories, err := testbed.Components()
	require.NoError(t, err, "default components resulted in: %v", err)
	runner := testbed.NewInProcessCollector(factories)
	options := testbed.LoadOptions{DataItemsPerSecond: 10_000, ItemsPerBatch: 10}
	dataProvider := testbed.NewPerfTestDataProvider(options)
	sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t))
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		runner,
		&testbed.PerfTestValidator{},
		performanceResultsSummary,
		testbed.WithSkipResults(),
		testbed.WithResourceLimits(
			testbed.ResourceSpec{
				ExpectedMaxRAM:         1000,
				ResourceCheckPeriod:    time.Second,
				MaxConsecutiveFailures: 5,
			},
		),
	)
	defer tc.Stop()
	tc.StartBackend()
	tc.StartAgent()
	tc.StartLoad(options)
}

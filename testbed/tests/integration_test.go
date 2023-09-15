package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"
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
	_, err = runner.PrepareConfig(correctnesstests.CreateConfigYamlSporadic(sender, receiver, nil, "traces", 2))
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
	tc.Sleep(3 * time.Second)

	tc.StopLoad()

	tc.WaitFor(func() bool { return tc.LoadGenerator.DataItemsSent() > 0 }, "load generator started")
	tc.WaitFor(func() bool {
		return tc.LoadGenerator.DataItemsSent()-tc.LoadGenerator.PermanentErrors() == tc.MockBackend.DataItemsReceived()
	},
		"all data items received")
	fmt.Println("Non permanent errors: ", tc.LoadGenerator.NonPermanentErrors())
	fmt.Println("Permanent errors: ", tc.LoadGenerator.PermanentErrors())
	tc.StopAgent()
}

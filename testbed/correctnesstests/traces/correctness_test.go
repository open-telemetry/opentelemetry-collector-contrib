// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var correctnessResults testbed.TestResultsSummary = &testbed.CorrectnessResults{}

func TestMain(m *testing.M) {
	testbed.DoTestMain(m, correctnessResults)
}

func TestTracingGoldenData(t *testing.T) {
	tests, err := correctnesstests.LoadPictOutputPipelineDefs("testdata/generated_pict_pairs_traces_pipeline.txt")
	require.NoError(t, err)
	processors := []correctnesstests.ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
    send_batch_size: 1024
`,
		},
	}
	for _, test := range tests {
		test.TestName = fmt.Sprintf("%s-%s", test.Receiver, test.Exporter)
		test.DataSender = correctnesstests.ConstructTraceSender(t, test.Receiver)
		test.DataReceiver = correctnesstests.ConstructReceiver(t, test.Exporter)
		t.Run(test.TestName, func(t *testing.T) {
			testWithTracingGoldenDataset(t, test.DataSender, test.DataReceiver, test.ResourceSpec, processors)
		})
	}
}

func testWithTracingGoldenDataset(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	processors []correctnesstests.ProcessorNameAndConfigBody,
) {
	dataProvider := testbed.NewGoldenDataProvider(
		"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_traces.txt",
		"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_spans.txt",
		"")
	factories, err := testbed.Components()
	require.NoError(t, err, "default components resulted in: %v", err)
	runner := testbed.NewInProcessCollector(factories)
	validator := testbed.NewCorrectTestValidator(sender.ProtocolName(), receiver.ProtocolName(), dataProvider)
	config := correctnesstests.CreateConfigYaml(t, sender, receiver, nil, processors)
	log.Println(config)
	configCleanup, cfgErr := runner.PrepareConfig(t, config)
	require.NoError(t, cfgErr, "collector configuration resulted in: %v", cfgErr)
	defer configCleanup()
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		runner,
		validator,
		correctnessResults,
		testbed.WithResourceLimits(resourceSpec),
	)
	defer tc.Stop()

	tc.EnableRecording()
	tc.StartBackend()
	tc.StartAgent()

	tc.StartLoad(testbed.LoadOptions{
		DataItemsPerSecond: 1024,
		ItemsPerBatch:      1,
	})

	tc.Sleep(2 * time.Second)

	tc.StopLoad()

	tc.WaitForN(func() bool { return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived() },
		3*time.Second, "all data items received")

	tc.StopAgent()

	tc.ValidateData()
}

func TestSporadicGoldenDataset(t *testing.T) {
	testCases := []struct {
		decisionFunc func() error
	}{
		{
			decisionFunc: testbed.RandomNonPermanentError,
		},
		{
			decisionFunc: testbed.RandomPermanentError,
		},
	}
	for _, tt := range testCases {
		factories, err := testbed.Components()
		require.NoError(t, err, "default components resulted in: %v", err)
		runner := testbed.NewInProcessCollector(factories)
		options := testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10}
		dataProvider := testbed.NewGoldenDataProvider(
			"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_traces.txt",
			"../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_spans.txt",
			"")
		sender := testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
		receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))
		receiver.WithRetry(`
    retry_on_failure:
      enabled: false
`)
		receiver.WithQueue(`
    sending_queue:
      enabled: false
`)
		_, err = runner.PrepareConfig(t, correctnesstests.CreateConfigYaml(t, sender, receiver, nil, nil))
		require.NoError(t, err, "collector configuration resulted in: %v", err)
		validator := testbed.NewCorrectTestValidator(sender.ProtocolName(), receiver.ProtocolName(), dataProvider)
		tc := testbed.NewTestCase(
			t,
			dataProvider,
			sender,
			receiver,
			runner,
			validator,
			correctnessResults,
			testbed.WithSkipResults(),
			testbed.WithDecisionFunc(tt.decisionFunc),
		)
		defer tc.Stop()
		tc.StartBackend()
		tc.StartAgent()
		tc.StartLoad(options)
		tc.Sleep(3 * time.Second)

		tc.StopLoad()

		tc.WaitForN(func() bool {
			return tc.LoadGenerator.DataItemsSent()-tc.LoadGenerator.PermanentErrors() == tc.MockBackend.DataItemsReceived()
		}, 5*time.Second, "all data items received")
		tc.StopAgent()
		tc.ValidateData()
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connectors

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var correctnessResults testbed.TestResultsSummary = &testbed.CorrectnessResults{}

func TestMain(m *testing.M) {
	testbed.DoTestMain(m, correctnessResults)
}

func TestGoldenData(t *testing.T) {
	processors := []correctnesstests.ProcessorNameAndConfigBody{
		{
			Name: "batch",
			Body: `
  batch:
    send_batch_size: 1024
`,
		},
	}
	sampleTest := correctnesstests.PipelineDef{
		TestName:  "test routing",
		Receiver:  "otlp",
		Exporter:  "otlp",
		Connector: "routing",
	}

	sampleTest.DataSender = correctnesstests.ConstructTraceSender(t, sampleTest.Receiver)
	sampleTest.DataReceiver = correctnesstests.ConstructReceiver(t, sampleTest.Exporter)
	sampleTest.DataConnector = correctnesstests.ConstructConnector(t, sampleTest.Connector, "traces")
	t.Run(sampleTest.TestName, func(t *testing.T) {
		testWithGoldenDataset(t, sampleTest.DataSender, sampleTest.DataReceiver, sampleTest.ResourceSpec, sampleTest.DataConnector, processors)
	})
}

func testWithGoldenDataset(
	t *testing.T,
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resourceSpec testbed.ResourceSpec,
	connector testbed.DataConnector,
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
	config := correctnesstests.CreateConfigYaml(t, sender, receiver, connector, processors)
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
}

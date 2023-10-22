// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package correctnesstests // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datareceivers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

var correctnessResults testbed.TestResultsSummary = &testbed.CorrectnessResults{}

func retryableDecisionFunc() error {

	// Return in roughly 1 out of 4 times, so we can test the retrying with multiple failures
	if rand.Float32() < 0.7 {
		return status.New(codes.Unavailable, "non-permanent").Err()
	}
	return nil
}

func TestRetrying(t *testing.T) {
	tests := []struct {
		name     string
		sender   testbed.DataSender
		receiver testbed.DataReceiver
	}{
		{
			"Zipkin",
			datasenders.NewZipkinDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
			datareceivers.NewZipkinDataReceiver(testbed.GetAvailablePort(t)),
		},
		{
			"OTLP",
			testbed.NewOTLPTraceDataSender(testbed.DefaultHost, testbed.GetAvailablePort(t)),
			testbed.NewOTLPDataReceiver(testbed.GetAvailablePort(t)),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factories, err := testbed.Components()
			require.NoError(t, err, "default components resulted in: %v", err)
			runner := testbed.NewInProcessCollector(factories)
			options := testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10}
			dataProvider := testbed.NewGoldenDataProvider(
				"../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_traces.txt",
				"../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_spans.txt",
				"")
			test.receiver.WithRetry(`
    retry_on_failure:
      enabled: true
      initial_interval: 0.5s
`)
			test.receiver.WithQueue(`
    sending_queue:
      enabled: false
`)

			_, err = runner.PrepareConfig(CreateConfigYaml(test.sender, test.receiver, nil, "traces"))
			fmt.Println(CreateConfigYaml(test.sender, test.receiver, nil, "traces"))
			require.NoError(t, err, "collector configuration resulted in: %v", err)
			validator := testbed.NewCorrectTestValidator(test.sender.ProtocolName(), test.receiver.ProtocolName(), dataProvider)
			tc := testbed.NewTestCase(
				t,
				dataProvider,
				test.sender,
				test.receiver,
				runner,
				validator,
				correctnessResults,
				testbed.WithSkipResults(),
				testbed.WithDecisionFunc(retryableDecisionFunc),
			)
			defer tc.Stop()
			tc.StartBackend()
			tc.StartAgent()
			tc.StartLoad(options)

			tc.Sleep(5 * time.Second)

			tc.StopLoad()

			tc.WaitForN(func() bool {
				return tc.LoadGenerator.DataItemsSent() == tc.MockBackend.DataItemsReceived()
			}, 10*time.Second, "all data items received")
			tc.StopAgent()
			tc.ValidateData()
		})
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestLogExporter(t *testing.T) {
	for _, tc := range []struct {
		name string
		// restartCollector restarts the OTEL collector. Restarting
		// the collector allows durability testing of the ES exporter
		// based on the OTEL config used for testing.
		restartCollector bool
		mockESFailure    bool
	}{
		{name: "basic"},
		{name: "es_intermittent_failure", mockESFailure: true},
		/* Below tests should be enabled after https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30792 is fixed
		{name: "collector_restarts", restartCollector: true},
		{name: "collector_restart_with_es_intermittent_failure", mockESFailure: true, restartCollector: true},
		*/
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner(t, tc.restartCollector, tc.mockESFailure)
		})
	}
}

func runner(t *testing.T, restartCollector, mockESFailure bool) {
	t.Helper()

	sender := testbed.NewOTLPLogsDataSender(
		testbed.DefaultHost, testutil.GetAvailablePort(t),
	)
	receiver := newElasticsearchDataReceiver(t)
	provider := testbed.NewPerfTestDataProvider(testbed.LoadOptions{
		DataItemsPerSecond: 10_000,
		ItemsPerBatch:      10,
	})

	cfg := createConfigYaml(t, sender, receiver, nil, nil, "logs", getDebugFlag(t))
	t.Log("test otel collector configuration:", cfg)
	collector := newRecreatableOtelCol(t)
	cleanup, err := collector.PrepareConfig(cfg)
	require.NoError(t, err)
	defer cleanup()

	var esFailing atomic.Bool
	tc := testbed.NewTestCase(
		t,
		provider,
		sender,
		receiver,
		collector,
		testbed.NewCorrectTestValidator(sender.ProtocolName(), receiver.ProtocolName(), provider),
		&testbed.CorrectnessResults{},
		testbed.WithDecisionFunc(func() error {
			if esFailing.Load() {
				return errors.New("simulated ES failure")
			}
			return nil
		}),
	)
	defer tc.Stop()

	tc.EnableRecording()
	tc.StartBackend()
	tc.StartAgent()

	// Start sending load and send for some time before proceeding.
	tc.StartLoad(testbed.LoadOptions{DataItemsPerSecond: 1_000})
	tc.Sleep(2 * time.Second)

	// Fail ES if required and send load.
	if mockESFailure {
		esFailing.Store(true)
		tc.Sleep(2 * time.Second)
	}

	// Restart collector if required and send load.
	if restartCollector {
		require.NoError(t, collector.Restart(false, 2*time.Second))
		tc.Sleep(2 * time.Second)
	}

	// Recover ES if failing and send load.
	if esFailing.Swap(false) {
		tc.Sleep(2 * time.Second)
	}
	tc.StopLoad()

	tc.WaitFor(
		func() bool {
			return tc.MockBackend.DataItemsReceived() == tc.LoadGenerator.DataItemsSent()
		},
		"backend should receive all sent items",
	)
	tc.ValidateData()
}

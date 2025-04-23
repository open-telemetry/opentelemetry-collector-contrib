// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

func TestExporter(t *testing.T) {
	for _, eventType := range []string{"logs", "metrics", "traces"} {
		for _, tc := range []struct {
			name string

			// batcherEnabled enables/disables the batch sender. If this is
			// nil, then the exporter buffers data itself (legacy behavior),
			// whereas if it is non-nil then the exporter will not perform
			// any buffering itself.
			batcherEnabled *bool

			// restartCollector restarts the OTEL collector. Restarting
			// the collector allows durability testing of the ES exporter
			// based on the OTEL config used for testing.
			restartCollector bool
			mockESFailure    bool
		}{
			{name: "basic"},
			{name: "es_intermittent_failure", mockESFailure: true},

			{name: "batcher_enabled", batcherEnabled: ptrTo(true)},
			{name: "batcher_enabled_es_intermittent_failure", batcherEnabled: ptrTo(true), mockESFailure: true},
			{name: "batcher_disabled", batcherEnabled: ptrTo(false)},
			{name: "batcher_disabled_es_intermittent_failure", batcherEnabled: ptrTo(false), mockESFailure: true},

			/* TODO: Below tests should be enabled after https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30792 is fixed
			{name: "collector_restarts", restartCollector: true},
			{name: "collector_restart_with_es_intermittent_failure", mockESFailure: true, restartCollector: true},
			*/
		} {
			t.Run(fmt.Sprintf("%s/%s", eventType, tc.name), func(t *testing.T) {
				var opts []dataReceiverOption
				if tc.batcherEnabled != nil {
					opts = append(opts, withBatcherEnabled(*tc.batcherEnabled))
				}
				runner(t, eventType, tc.restartCollector, tc.mockESFailure, opts...)
			})
		}
	}
}

func runner(t *testing.T, eventType string, restartCollector, mockESFailure bool, opts ...dataReceiverOption) {
	t.Helper()

	var (
		sender testbed.DataSender
		host   = testbed.DefaultHost
		port   = testutil.GetAvailablePort(t)
	)
	switch eventType {
	case "logs":
		sender = testbed.NewOTLPLogsDataSender(host, port)
	case "metrics":
		sender = testbed.NewOTLPMetricDataSender(host, port)
	case "traces":
		sender = testbed.NewOTLPTraceDataSender(host, port)
	default:
		t.Fatalf("failed to create data sender for type: %s", eventType)
	}

	receiver := newElasticsearchDataReceiver(t, opts...)
	loadOpts := testbed.LoadOptions{
		DataItemsPerSecond: 1_000,
		ItemsPerBatch:      10,
	}
	provider := testbed.NewPerfTestDataProvider(loadOpts)

	cfg := createConfigYaml(t, sender, receiver, nil, nil, eventType, getDebugFlag(t))
	t.Log("test otel collector configuration:", cfg)
	collector := newRecreatableOtelCol(t)
	cleanup, err := collector.PrepareConfig(t, cfg)
	require.NoError(t, err)
	defer cleanup()

	var esFailing atomic.Bool
	tc := testbed.NewTestCase(
		t,
		provider,
		sender,
		receiver,
		collector,
		newCountValidator(t, provider),
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
	tc.StartLoad(loadOpts)
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

func ptrTo[T any](t T) *T {
	return &t
}

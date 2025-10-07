// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package integrationtest

import (
	"fmt"
	"net"
	"net/http"
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
			mockESErr        error
		}{
			{name: "basic"},
			{name: "es_intermittent_http_error", mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}},
			{name: "es_intermittent_doc_error", mockESErr: errElasticsearch{httpStatus: http.StatusOK, httpDocStatus: http.StatusTooManyRequests}},

			{name: "batcher_enabled", batcherEnabled: ptrTo(true)},
			{name: "batcher_enabled_es_intermittent_http_error", batcherEnabled: ptrTo(true), mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}},
			{name: "batcher_enabled_es_intermittent_doc_error", batcherEnabled: ptrTo(true), mockESErr: errElasticsearch{httpStatus: http.StatusOK, httpDocStatus: http.StatusTooManyRequests}},
			{name: "batcher_disabled", batcherEnabled: ptrTo(false)},
			{name: "batcher_disabled_es_intermittent_http_error", batcherEnabled: ptrTo(false), mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}},
			{name: "batcher_disabled_es_intermittent_doc_error", batcherEnabled: ptrTo(false), mockESErr: errElasticsearch{httpStatus: http.StatusOK, httpDocStatus: http.StatusTooManyRequests}},

			/* TODO: Below tests should be enabled after https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30792 is fixed
			{name: "collector_restarts", restartCollector: true},
			{name: "collector_restart_with_es_intermittent_failure", mockESErr: true, restartCollector: true},
			*/
		} {
			t.Run(fmt.Sprintf("%s/%s", eventType, tc.name), func(t *testing.T) {
				var opts []dataReceiverOption
				if tc.batcherEnabled != nil {
					opts = append(opts, withBatcherEnabled(*tc.batcherEnabled))
				}
				runner(t, eventType, tc.restartCollector, tc.mockESErr, opts...)
			})
		}
	}
}

func runner(t *testing.T, eventType string, restartCollector bool, mockESErr error, opts ...dataReceiverOption) {
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

	// The port used by the sender is not yet active and can be detected as a
	// available port by another call to testutil#GetAvailablePort in an attempt
	// to create a new datareceiver. To prevent the conflict occupy the port
	// temporarily.
	testListner, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	require.NoError(t, err, "port is expected to be free")

	receiver := newElasticsearchDataReceiver(t, opts...)
	loadOpts := testbed.LoadOptions{
		DataItemsPerSecond: 1_000,
		ItemsPerBatch:      10,
	}
	provider := testbed.NewPerfTestDataProvider(loadOpts)

	// Stop the listener so that collector can start correctly.
	require.NoError(t, testListner.Close())

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
				return mockESErr
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
	if mockESErr != nil {
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

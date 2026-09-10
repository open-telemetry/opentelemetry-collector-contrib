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

			enableBatching bool

			// restartCollector restarts the OTEL collector. Restarting
			// the collector allows durability testing of the ES exporter
			// based on the OTEL config used for testing.
			restartCollector bool
			mockESErr        error
		}{
			{name: "basic"},
			{name: "es_intermittent_http_error", mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}},
			{name: "es_intermittent_doc_error", mockESErr: errElasticsearch{httpStatus: http.StatusOK, httpDocStatus: http.StatusTooManyRequests}},

			{name: "enable sending_queue batching", enableBatching: true},
			{name: "batcher_enabled_es_intermittent_http_error", enableBatching: true, mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}},
			{name: "batcher_enabled_es_intermittent_doc_error", enableBatching: true, mockESErr: errElasticsearch{httpStatus: http.StatusOK, httpDocStatus: http.StatusTooManyRequests}},
			{name: "batcher_disabled", enableBatching: false},
			{name: "batcher_disabled_es_intermittent_http_error", enableBatching: false, mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}},
			{name: "batcher_disabled_es_intermittent_doc_error", enableBatching: false, mockESErr: errElasticsearch{httpStatus: http.StatusOK, httpDocStatus: http.StatusTooManyRequests}},

			/* TODO: Below tests should be enabled after https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30792 is fixed
			{name: "collector_restarts", restartCollector: true},
			{name: "collector_restart_with_es_intermittent_failure", mockESErr: true, restartCollector: true},
			*/
		} {
			t.Run(fmt.Sprintf("%s/%s", eventType, tc.name), func(t *testing.T) {
				runner(t, eventType, tc.restartCollector, tc.mockESErr, withBatching(tc.enableBatching))
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

type outageDataReceiver struct {
	*esDataReceiver
	storage string
}

func (r *outageDataReceiver) GenConfigYAMLStr() string {
	storage := ""
	if r.storage != "" {
		storage = fmt.Sprintf("      storage: %s\n", r.storage)
	}
	return fmt.Sprintf(`
  elasticsearch:
    endpoint: %q
    logs_index: %s
    retry:
      enabled: true
      max_retries: 200
      initial_interval: 500ms
      max_interval: 1s
    timeout: 5s
    sending_queue:
      enabled: true
%s      block_on_overflow: false
      num_consumers: 10
      queue_size: 10000
      sizer: requests
      wait_for_result: false
      batch:
        flush_timeout: 10m
        min_size: 5000
        max_size: 10000
        sizer: items
`, r.endpoint, TestLogsIndex, storage)
}

// TestExporterRetriesAfterElasticsearchRecovers verifies that data remains
// queued while Elasticsearch is unavailable and is exported when it recovers.
// The retry intervals are short enough for testing, while the exporter timeout
// is long enough to inspect retry behavior with a debugger.
func TestExporterRetriesAfterElasticsearchRecovers(t *testing.T) {
	for _, tc := range []struct {
		name       string
		persistent bool
	}{
		{name: "persistent_queue", persistent: true},
		{name: "in_memory_queue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testExporterRetriesAfterElasticsearchRecovers(t, tc.persistent)
		})
	}
}

func testExporterRetriesAfterElasticsearchRecovers(t *testing.T, persistent bool) {
	senderPort := testutil.GetAvailablePort(t)
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", senderPort))
	require.NoError(t, err, "port is expected to be free")

	sender := testbed.NewOTLPLogsDataSender(testbed.DefaultHost, senderPort)
	receiver := &outageDataReceiver{esDataReceiver: newElasticsearchDataReceiver(t)}
	loadOpts := testbed.LoadOptions{
		DataItemsPerSecond: 5_000,
		ItemsPerBatch:      5_000,
	}
	provider := testbed.NewPerfTestDataProvider(loadOpts)

	require.NoError(t, listener.Close())

	var extensions map[string]string
	if persistent {
		receiver.storage = "file_storage"
		extensions = map[string]string{
			"file_storage": fmt.Sprintf("file_storage:\n    directory: %q", t.TempDir()),
		}
	}
	cfg := createConfigYaml(t, sender, receiver, nil, extensions, "logs", getDebugFlag(t))
	t.Log("test otel collector configuration:", cfg)
	collector := newRecreatableOtelCol(t)
	cleanup, err := collector.PrepareConfig(t, cfg)
	require.NoError(t, err)
	defer cleanup()

	tc := testbed.NewTestCase(
		t,
		provider,
		sender,
		receiver,
		collector,
		newCountValidator(t, provider),
		&testbed.CorrectnessResults{},
		testbed.WithSkipResults(),
	)
	defer tc.Stop()

	// Start the collector without the mock Elasticsearch backend, send one
	// full batch, and allow the first export attempt to time out.
	tc.StartAgent()
	tc.StartLoad(loadOpts)
	require.Eventually(t, func() bool {
		return tc.LoadGenerator.DataItemsSent() == 5_000
	}, 3*time.Second, 10*time.Millisecond)
	tc.StopLoad()
	time.Sleep(5500 * time.Millisecond)

	// Once Elasticsearch recovers, the queue should retry the request and
	// deliver every log in the batch.
	tc.StartBackend()
	require.Eventually(t, func() bool {
		return tc.MockBackend.DataItemsReceived() == tc.LoadGenerator.DataItemsSent()
	}, 3*time.Second, 10*time.Millisecond)
}

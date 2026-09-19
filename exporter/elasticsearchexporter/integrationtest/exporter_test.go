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

			// persistentQueue backs the exporter's sending queue with the
			// file_storage extension, so every request round-trips through
			// the persistent queue's on-disk serialization before delivery.
			persistentQueue bool

			// allowMissingDocs tolerates lost documents: the run only requires
			// delivery to resume and the received count to stabilize. Used for
			// scenarios hitting
			// https://github.com/open-telemetry/opentelemetry-collector/issues/15677.
			allowMissingDocs bool

			// logsOnly limits a scenario to the logs event type. The queueing
			// behavior it exercises is signal-agnostic, and CI runs this
			// package's tests three times (-count=3) within one timeout, so
			// repeating such scenarios per signal would blow the budget.
			logsOnly bool
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

			{name: "persistent_queue", persistentQueue: true, logsOnly: true},
			{name: "persistent_queue_es_intermittent_http_error", persistentQueue: true, mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}, logsOnly: true},
			{name: "persistent_queue_collector_restart", persistentQueue: true, restartCollector: true, logsOnly: true},
			// Restarting DURING an active ES outage loses the documents that
			// are queued or in flight at shutdown, because the shutdown-time
			// queue drain completes them with an error and removes them from
			// the persistent queue instead of keeping them for redelivery; see
			// https://github.com/open-telemetry/opentelemetry-collector/issues/15677.
			{name: "persistent_queue_collector_restart_with_es_intermittent_failure", persistentQueue: true, restartCollector: true, allowMissingDocs: true, mockESErr: errElasticsearch{httpStatus: http.StatusServiceUnavailable}, logsOnly: true},
			{name: "collector_restarts", restartCollector: true, logsOnly: true},
			// The in-memory equivalent (collector_restart_with_es_intermittent_failure)
			// stays disabled: an in-memory queue inherently loses its contents
			// across a restart, so there is no delivery bound to assert.
		} {
			if tc.logsOnly && eventType != "logs" {
				continue
			}
			t.Run(fmt.Sprintf("%s/%s", eventType, tc.name), func(t *testing.T) {
				runner(t, eventType, tc.restartCollector, tc.persistentQueue, tc.allowMissingDocs, tc.mockESErr, withBatching(tc.enableBatching))
			})
		}
	}
}

func runner(t *testing.T, eventType string, restartCollector, persistentQueue, allowMissingDocs bool, mockESErr error, opts ...dataReceiverOption) {
	t.Helper()

	// A persistent sending queue stores every request through the file_storage
	// extension, exercising the exporter's on-disk queue serialization in a
	// full collector pipeline. Restart-survival is not asserted here; the
	// restart scenarios above are disabled pending
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30792.
	if allowMissingDocs {
		// Keep ES-level retries short so the shutdown-time queue drain fails
		// fast instead of blocking collector shutdown behind the HTTP client's
		// long retry loop.
		opts = append(opts, withMaxRetries(2))
	}
	var extensions map[string]string
	if persistentQueue {
		extensions = map[string]string{
			"file_storage": fmt.Sprintf("file_storage:\n    directory: '%s'", t.TempDir()),
		}
		opts = append(opts, withQueueStorage("file_storage"))
	}

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

	cfg := createConfigYaml(t, sender, receiver, nil, extensions, eventType, getDebugFlag(t))
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

	if allowMissingDocs {
		// Documents lost to the shutdown-drain behavior described in the
		// scenario table cannot arrive, so only require delivery to resume
		// and the received count to stabilize.
		// TODO: require full delivery once
		// https://github.com/open-telemetry/opentelemetry-collector/issues/15677
		// is fixed.
		var lastReceived uint64
		lastChange := time.Now()
		tc.WaitForN(
			func() bool {
				if received := tc.MockBackend.DataItemsReceived(); received != lastReceived {
					lastReceived = received
					lastChange = time.Now()
				}
				return lastReceived > 0 && time.Since(lastChange) > 3*time.Second
			},
			30*time.Second,
			"backend should keep receiving items and stabilize",
		)
		require.Positive(t, lastReceived)
		require.LessOrEqual(t, lastReceived, tc.LoadGenerator.DataItemsSent())
		return
	}

	tc.WaitFor(
		func() bool {
			return tc.MockBackend.DataItemsReceived() == tc.LoadGenerator.DataItemsSent()
		},
		"backend should receive all sent items",
	)
	tc.ValidateData()
}

// outageExporterTimeout is the exporter `timeout` used by
// TestExporterRetriesAfterElasticsearchRecovers. The simulated outage is kept
// going until the exporter has been retrying for longer than this.
const outageExporterTimeout = 5 * time.Second

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
    timeout: %s
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
`, r.endpoint, TestLogsIndex, outageExporterTimeout, storage)
}

// TestExporterRetriesAfterElasticsearchRecovers verifies that data remains
// queued while Elasticsearch is unavailable and is exported when it recovers.
//
// The outage is simulated by the mock Elasticsearch aborting every connection
// without a response. The exporter sees the same transport-level failure as
// with an unreachable endpoint, but deterministically on every platform and
// observably by the test: dialing a closed port can take seconds to fail on
// Windows, which made a closed-port version of this test flaky.
//
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

	// While esDown is set, the mock Elasticsearch aborts every bulk request.
	// Each aborted request is one export attempt; timestamping them lets the
	// test observe how long the exporter keeps retrying.
	var (
		esDown                        atomic.Bool
		firstAttemptAt, lastAttemptAt atomic.Int64 // Unix nanoseconds
	)
	esDown.Store(true)

	tc := testbed.NewTestCase(
		t,
		provider,
		sender,
		receiver,
		collector,
		newCountValidator(t, provider),
		&testbed.CorrectnessResults{},
		testbed.WithSkipResults(),
		testbed.WithDecisionFunc(func() error {
			if !esDown.Load() {
				return nil
			}
			now := time.Now().UnixNano()
			firstAttemptAt.CompareAndSwap(0, now)
			lastAttemptAt.Store(now)
			return errElasticsearch{abortConnection: true}
		}),
	)
	defer tc.Stop()

	// Start with Elasticsearch down and send one full batch.
	tc.StartBackend()
	tc.StartAgent()
	tc.StartLoad(loadOpts)
	require.Eventually(t, func() bool {
		return tc.LoadGenerator.DataItemsSent() == 5_000
	}, 3*time.Second, 10*time.Millisecond)
	tc.StopLoad()

	// Keep the outage going until the exporter has been retrying for longer
	// than `timeout`. Before #49834 all retries of a flush shared a single
	// `timeout` deadline, so retrying stopped here and the batch was dropped.
	require.Eventually(t, func() bool {
		first, last := firstAttemptAt.Load(), lastAttemptAt.Load()
		return first != 0 && time.Duration(last-first) > outageExporterTimeout
	}, 30*time.Second, 10*time.Millisecond, "exporter should keep retrying while Elasticsearch is down")

	// Once Elasticsearch recovers, the next retry, at most retry::max_interval
	// away, should deliver every log in the batch.
	esDown.Store(false)
	require.Eventually(t, func() bool {
		return tc.MockBackend.DataItemsReceived() == tc.LoadGenerator.DataItemsSent()
	}, 15*time.Second, 10*time.Millisecond)
}

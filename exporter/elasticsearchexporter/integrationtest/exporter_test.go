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
	for _, eventType := range []string{"logs", "traces"} {
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
			{name: "collector_restarts", restartCollector: true},
			// Test is failing due to timeout because in-flight requests are not aware of shutdown
			// and will keep retrying up to the configured retry limit
			// TODO: re-enable test after moving to use retry sender
			// as https://github.com/open-telemetry/opentelemetry-collector/issues/10166 is fixed
			// {name: "collector_restart_with_es_intermittent_failure", mockESFailure: true, restartCollector: true},
		} {
			t.Run(fmt.Sprintf("%s/%s", eventType, tc.name), func(t *testing.T) {
				runner(t, eventType, tc.restartCollector, tc.mockESFailure)
			})
		}
	}
}

func runner(t *testing.T, eventType string, restartCollector, mockESFailure bool) {
	t.Helper()

	var (
		sender testbed.DataSender
		host   = testbed.DefaultHost
		port   = testutil.GetAvailablePort(t)
	)
	switch eventType {
	case "logs":
		sender = testbed.NewOTLPLogsDataSender(host, port)
	case "traces":
		sender = testbed.NewOTLPTraceDataSender(host, port)
	default:
		t.Fatalf("failed to create data sender for type: %s", eventType)
	}

	receiver := newElasticsearchDataReceiver(t, true, nil)
	loadOpts := testbed.LoadOptions{
		DataItemsPerSecond: 1_000,
		ItemsPerBatch:      10,
	}
	provider := testbed.NewPerfTestDataProvider(loadOpts)

	tempDir := t.TempDir()
	extensions := map[string]string{
		"file_storage/elasticsearchexporter": fmt.Sprintf(`file_storage/elasticsearchexporter:
    directory: %s`, tempDir),
	}
	cfg := createConfigYaml(t, sender, receiver, nil, extensions, eventType, getDebugFlag(t))
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

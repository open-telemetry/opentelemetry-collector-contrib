// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// TestKubeletStatsReceiver measures Collector CPU and memory while the receiver
// repeatedly scrapes kubelet stats for increasing numbers of pods.
func TestKubeletStatsReceiver(t *testing.T) {
	tests := []struct {
		name     string
		podCount int
	}{
		{name: "Baseline/10Pods", podCount: 10},
		{name: "Scale/100Pods", podCount: 100},
		{name: "Scale/1000Pods", podCount: 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioKubeletStatsScrape(t, tt.podCount)
		})
	}
}

func scenarioKubeletStatsScrape(t *testing.T, podCount int) {
	statsSummary := buildKubeletStatsSummary(t, podCount)

	sender := &kubeletStatsStaticSender{
		DataSenderBase: testbed.DataSenderBase{
			Host: testbed.DefaultHost,
			Port: testutil.GetAvailablePort(t),
		},
		statsSummary: statsSummary,
	}
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	agentProc := testbed.NewChildProcessCollector(testbed.WithEnvVar("GOMAXPROCS", "2"))
	configCleanup, err := agentProc.PrepareConfig(t, createKubeletStatsConfigYAML(sender, receiver, resultDir))
	require.NoError(t, err)
	t.Cleanup(configCleanup)

	loadGen := &staticScrapeLoadGenerator{sender: sender}
	tc := testbed.NewLoadGeneratorTestCase(
		t,
		loadGen,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{IncludeLimitsInReport: true},
		performanceResultsSummary,
		testbed.WithResourceLimits(testbed.ResourceSpec{
			ExpectedMaxCPU: 200,
			ExpectedMaxRAM: 512,
		}),
	)
	t.Cleanup(tc.Stop)

	require.NoError(t, sender.Start())
	t.Cleanup(func() { _ = sender.Stop() })

	tc.StartBackend()
	tc.StartAgent()

	require.Eventually(t, func() bool { return tc.MockBackend.DataItemsReceived() > 0 },
		10*time.Second, 100*time.Millisecond, "collector never produced data from kubelet stats")

	startReceived := tc.MockBackend.DataItemsReceived()

	benchDuration := tc.Duration
	tc.Sleep(benchDuration)

	require.NoError(t, sender.Stop())
	totalReceived := waitForBackendDrain(t, tc.MockBackend, time.Second)

	received := totalReceived - startReceived
	throughput := float64(received) / benchDuration.Seconds()

	loadGen.SetDataItemsSent(totalReceived)

	rc := agentProc.GetTotalConsumption()
	t.Logf(
		"Scraped %d data points during %s (%.0f points/sec). CPU avg/max=%.1f%%/%.1f%%, RAM avg/max=%d/%d MiB",
		received, benchDuration, throughput,
		rc.CPUPercentAvg, rc.CPUPercentMax,
		rc.RAMMiBAvg, rc.RAMMiBMax,
	)
}

type kubeletStatsStaticSender struct {
	testbed.DataSenderBase
	server       *http.Server
	serverMu     sync.Mutex
	stopOnce     sync.Once
	stopErr      error
	startedAt    time.Time
	statsSummary []byte
}

func (s *kubeletStatsStaticSender) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats/summary", s.serveStatsSummary)

	server := &http.Server{
		Addr:              s.GetEndpoint().String(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}

	s.serverMu.Lock()
	s.server = server
	s.startedAt = time.Now()
	s.serverMu.Unlock()

	go func() {
		_ = server.Serve(listener)
	}()

	return nil
}

func (s *kubeletStatsStaticSender) serveStatsSummary(
	w http.ResponseWriter,
	_ *http.Request,
) {
	var summary map[string]any
	if err := json.Unmarshal(s.statsSummary, &summary); err != nil {
		http.Error(w, "invalid stats summary fixture", http.StatusInternalServerError)
		return
	}

	s.serverMu.Lock()
	startedAt := s.startedAt
	s.serverMu.Unlock()

	refreshCPUStats(summary, time.Now().UTC(), time.Since(startedAt))

	payload, err := json.Marshal(summary)
	if err != nil {
		http.Error(w, "could not encode stats summary", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(payload)
}

func refreshCPUStats(value any, now time.Time, elapsed time.Duration) {
	switch current := value.(type) {
	case map[string]any:
		if cpu, ok := current["cpu"].(map[string]any); ok {
			if _, ok := cpu["time"].(string); ok {
				cpu["time"] = now.Format(time.RFC3339Nano)
			}
			if usage, ok := cpu["usageCoreNanoSeconds"].(float64); ok && usage >= 0 {
				cpu["usageCoreNanoSeconds"] =
					uint64(usage) + uint64(elapsed.Nanoseconds()/100)
			}
		}

		for _, child := range current {
			refreshCPUStats(child, now, elapsed)
		}
	case []any:
		for _, child := range current {
			refreshCPUStats(child, now, elapsed)
		}
	}
}

func (s *kubeletStatsStaticSender) Stop() error {
	s.stopOnce.Do(func() {
		s.serverMu.Lock()
		server := s.server
		s.serverMu.Unlock()

		if server == nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		s.stopErr = server.Shutdown(ctx)
	})

	return s.stopErr
}

func (s *kubeletStatsStaticSender) GenConfigYAMLStr() string {
	return fmt.Sprintf(`
  kubeletstats:
    collection_interval: 1s
    endpoint: "%s"
    auth_type: none
    metric_groups: [container, pod, node]
`, s.GetEndpoint())
}

func (*kubeletStatsStaticSender) ProtocolName() string {
	return "kubeletstats"
}

func createKubeletStatsConfigYAML(
	sender testbed.DataSender,
	receiver testbed.DataReceiver,
	resultDir string,
) string {
	return fmt.Sprintf(`
receivers:%s
exporters:%s
extensions:
  pprof:
    save_to_file: %s/cpu.prof

service:
  extensions: [pprof]
  pipelines:
    metrics:
      receivers: [%s]
      exporters: [%s]
`,
		sender.GenConfigYAMLStr(),
		receiver.GenConfigYAMLStr(),
		resultDir,
		sender.ProtocolName(),
		receiver.ProtocolName(),
	)
}

func buildKubeletStatsSummary(t *testing.T, podCount int) []byte {
	t.Helper()

	fixturePath := filepath.Join(
		"..", "..", "receiver", "kubeletstatsreceiver",
		"testdata", "stats-summary.json",
	)
	fixture, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	var summary map[string]any
	require.NoError(t, json.Unmarshal(fixture, &summary))

	basePods, ok := summary["pods"].([]any)
	require.True(t, ok, "stats-summary fixture must contain a pods array")
	require.NotEmpty(t, basePods, "stats-summary fixture must contain at least one pod")

	podTemplate, ok := basePods[0].(map[string]any)
	require.True(t, ok, "stats-summary fixture pod must be an object")

	pods := make([]any, podCount)
	for i := range podCount {
		podJSON, marshalErr := json.Marshal(podTemplate)
		require.NoError(t, marshalErr)

		var pod map[string]any
		require.NoError(t, json.Unmarshal(podJSON, &pod))

		podRef, ok := pod["podRef"].(map[string]any)
		require.True(t, ok, "stats-summary fixture pod must contain podRef")

		podRef["name"] = fmt.Sprintf("benchmark-pod-%d", i)
		podRef["namespace"] = "benchmark"
		podRef["uid"] = fmt.Sprintf("benchmark-pod-uid-%d", i)
		pods[i] = pod
	}

	summary["pods"] = pods

	result, err := json.Marshal(summary)
	require.NoError(t, err)

	return result
}

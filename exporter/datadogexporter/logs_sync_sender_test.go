// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !solaris && !freebsd

package datadogexporter

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter"
	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

const (
	sentLogRecordsMetric       = "otelcol_exporter_sent_log_records"
	sendFailedLogRecordsMetric = "otelcol_exporter_send_failed_log_records"
)

// setSyncSenderGate sets the logsagentexporter UseSyncSender feature gate for the duration of the test.
func setSyncSenderGate(t *testing.T, enabled bool) {
	t.Helper()
	previous := logsagentexporter.IsSyncSenderEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(logsagentexporter.SyncSenderGateID, enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(logsagentexporter.SyncSenderGateID, previous))
	})
}

// fakeLogsIntake stands in for the Datadog logs intake. It answers every payload of log records with
// the status returned by statusFor and counts the log records it accepted.
type fakeLogsIntake struct {
	*httptest.Server
	statusFor func(payload int64) int
	payloads  atomic.Int64
	delivered atomic.Int64
}

func newFakeLogsIntake(t *testing.T, statusFor func(payload int64) int) *fakeLogsIntake {
	t.Helper()
	intake := &fakeLogsIntake{statusFor: statusFor}
	intake.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer gz.Close()
			body = gz
		}
		data, err := io.ReadAll(body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var records []json.RawMessage
		if json.Unmarshal(data, &records) != nil || len(records) == 0 {
			// The logs agent checks HTTP connectivity with an empty JSON object when it starts.
			w.WriteHeader(http.StatusOK)
			return
		}
		status := intake.statusFor(intake.payloads.Add(1))
		if status == http.StatusOK {
			intake.delivered.Add(int64(len(records)))
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(intake.Close)
	return intake
}

func alwaysStatus(status int) func(int64) int {
	return func(int64) int { return status }
}

func testLogs(n int) plog.Logs {
	ld := plog.NewLogs()
	records := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for i := range n {
		lr := records.AppendEmpty()
		lr.Body().SetStr(fmt.Sprintf("log record %d", i))
		lr.SetSeverityText("Info")
	}
	return ld
}

// buildLogsExporter creates the logs exporter of the Datadog exporter, pointed at intakeURL. Unless
// mutate adds a sending queue, ConsumeLogs returns the outcome of the delivery.
func buildLogsExporter(t *testing.T, intakeURL string, retry configretry.BackOffConfig, mutate ...func(*datadogconfig.Config)) (exporter.Logs, *componenttest.Telemetry) {
	t.Helper()
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() { _ = tt.Shutdown(t.Context()) })

	cfg := &datadogconfig.Config{
		TagsConfig: datadogconfig.TagsConfig{Hostname: "test-host"},
		Logs: datadogconfig.LogsConfig{
			TCPAddrConfig:    confignet.TCPAddrConfig{Endpoint: intakeURL},
			UseCompression:   true,
			CompressionLevel: 6,
			BatchWait:        1,
		},
		BackOffConfig: retry,
	}
	for _, m := range mutate {
		m(cfg)
	}
	params := exportertest.NewNopSettings(metadata.Type)
	params.TelemetrySettings = tt.NewTelemetrySettings()

	exp, err := NewFactory().CreateLogs(t.Context(), params, cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	if logsagentexporter.IsSyncSenderEnabled() {
		// There is nothing to wait for. The asynchronous logs agent is left running instead: when the
		// intake keeps failing, it takes its whole grace period to stop.
		t.Cleanup(func() { _ = exp.Shutdown(t.Context()) })
	}
	return exp, tt
}

// TestLogsExporter_AsyncPipeline_SwallowsIntakeErrors documents the behavior reported in
// open-telemetry/opentelemetry-collector-contrib#47386: without the sync sender, intake errors never
// reach exporterhelper, which counts the records as sent.
func TestLogsExporter_AsyncPipeline_SwallowsIntakeErrors(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusServiceUnavailable} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			setSyncSenderGate(t, false)
			intake := newFakeLogsIntake(t, alwaysStatus(status))
			exp, tt := buildLogsExporter(t, intake.URL, configretry.BackOffConfig{})

			require.NoError(t, exp.ConsumeLogs(t.Context(), testLogs(10)))
			require.Eventually(t, func() bool { return intake.payloads.Load() > 0 }, 10*time.Second, 10*time.Millisecond,
				"the logs agent never tried to deliver the payload")

			assert.Zero(t, intake.delivered.Load())
			assert.Equal(t, int64(10), otelCounterSum(t, tt, sentLogRecordsMetric))
			assert.Zero(t, otelCounterSum(t, tt, sendFailedLogRecordsMetric))
		})
	}
}

func TestLogsExporter_SyncSender_Delivers(t *testing.T) {
	setSyncSenderGate(t, true)
	intake := newFakeLogsIntake(t, alwaysStatus(http.StatusOK))
	exp, tt := buildLogsExporter(t, intake.URL, configretry.BackOffConfig{})

	require.NoError(t, exp.ConsumeLogs(t.Context(), testLogs(10)))

	assert.Equal(t, int64(10), intake.delivered.Load(), "records must reach the intake before ConsumeLogs returns")
	assert.Equal(t, int64(10), otelCounterSum(t, tt, sentLogRecordsMetric))
	assert.Zero(t, otelCounterSum(t, tt, sendFailedLogRecordsMetric))
}

func TestLogsExporter_SyncSender_PermanentError(t *testing.T) {
	setSyncSenderGate(t, true)
	intake := newFakeLogsIntake(t, alwaysStatus(http.StatusForbidden))
	// Retries stay enabled: a permanent error must be dropped without retrying.
	exp, tt := buildLogsExporter(t, intake.URL, shortRetry())

	err := exp.ConsumeLogs(t.Context(), testLogs(10))

	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err), "403 must be reported as a permanent error: %v", err)
	assert.Contains(t, err.Error(), "403")
	assert.Equal(t, int64(1), intake.payloads.Load(), "a permanent error must not be retried")
	assert.Equal(t, int64(10), otelCounterSum(t, tt, sendFailedLogRecordsMetric))
	assert.Zero(t, otelCounterSum(t, tt, sentLogRecordsMetric))
}

func TestLogsExporter_SyncSender_RetryableError(t *testing.T) {
	setSyncSenderGate(t, true)
	intake := newFakeLogsIntake(t, alwaysStatus(http.StatusServiceUnavailable))
	exp, tt := buildLogsExporter(t, intake.URL, configretry.BackOffConfig{})

	err := exp.ConsumeLogs(t.Context(), testLogs(10))

	require.Error(t, err)
	assert.False(t, consumererror.IsPermanent(err), "503 must stay retryable: %v", err)
	assert.Contains(t, err.Error(), "503")
	assert.Equal(t, int64(10), otelCounterSum(t, tt, sendFailedLogRecordsMetric))
	assert.Zero(t, otelCounterSum(t, tt, sentLogRecordsMetric))
}

func TestLogsExporter_SyncSender_RetriesTransientErrors(t *testing.T) {
	setSyncSenderGate(t, true)
	intake := newFakeLogsIntake(t, func(payload int64) int {
		if payload <= 2 {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	})
	exp, tt := buildLogsExporter(t, intake.URL, shortRetry())

	require.NoError(t, exp.ConsumeLogs(t.Context(), testLogs(10)))

	assert.Equal(t, int64(3), intake.payloads.Load())
	assert.Equal(t, int64(10), intake.delivered.Load())
	assert.Equal(t, int64(10), otelCounterSum(t, tt, sentLogRecordsMetric))
	assert.Zero(t, otelCounterSum(t, tt, sendFailedLogRecordsMetric))
}

// The asynchronous logs pipeline batched the records of all requests. With a sync sender, the sending
// queue takes that role.
func TestLogsExporter_SyncSender_BatchesQueuedRecords(t *testing.T) {
	setSyncSenderGate(t, true)
	intake := newFakeLogsIntake(t, alwaysStatus(http.StatusOK))
	exp, _ := buildLogsExporter(t, intake.URL, configretry.BackOffConfig{}, func(cfg *datadogconfig.Config) {
		cfg.QueueSettings = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	})

	for range 5 {
		require.NoError(t, exp.ConsumeLogs(t.Context(), testLogs(2)))
	}

	require.Eventually(t, func() bool { return intake.delivered.Load() == 10 }, 5*time.Second, 10*time.Millisecond,
		"the queued records never reached the intake")
	assert.Less(t, intake.payloads.Load(), int64(5), "the records of the queued requests must share payloads")
}

// fakeIntConfig fakes the logs agent configuration that sizes the settings of a sync sender.
type fakeIntConfig map[string]int

func (c fakeIntConfig) GetInt(key string) int { return c[key] }

func TestLogsSenderConcurrency(t *testing.T) {
	tests := []struct {
		name string
		cfg  fakeIntConfig
		want int
	}{
		{
			name: "defaults of the logs pipelines",
			cfg:  fakeIntConfig{"logs_config.pipelines": 4},
			want: 40,
		},
		{
			name: "configured concurrent sends",
			cfg:  fakeIntConfig{"logs_config.pipelines": 2, "logs_config.batch_max_concurrent_send": 5},
			want: 10,
		},
		{
			name: "no pipelines configured",
			cfg:  fakeIntConfig{},
			want: 10,
		},
		{
			name: "invalid concurrent sends",
			cfg:  fakeIntConfig{"logs_config.pipelines": 3, "logs_config.batch_max_concurrent_send": -1},
			want: 30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, logsSenderConcurrency(tt.cfg))
		})
	}
}

func TestLogsBatchSize(t *testing.T) {
	assert.Equal(t, 500, logsBatchSize(fakeIntConfig{"logs_config.batch_max_size": 500}))
	assert.Equal(t, 1000, logsBatchSize(fakeIntConfig{}), "an unset size falls back to the default of the logs pipelines")
	assert.Equal(t, 1000, logsBatchSize(fakeIntConfig{"logs_config.batch_max_size": -5}))
}

func TestLogsExporterHelperSettings(t *testing.T) {
	syncSettings := &logsSyncSettings{concurrency: 40, batchSize: 1000}
	newConfig := func(t *testing.T, sendingQueue map[string]any) *datadogconfig.Config {
		t.Helper()
		cfg := datadogconfig.CreateDefaultConfig().(*datadogconfig.Config)
		cfg.ClientConfig.Timeout = 7 * time.Second
		require.NoError(t, cfg.Unmarshal(confmap.NewFromStringMap(map[string]any{"sending_queue": sendingQueue})))
		return cfg
	}

	t.Run("asynchronous pipeline keeps the configured settings", func(t *testing.T) {
		cfg := newConfig(t, map[string]any{})

		timeout, queue := logsExporterHelperSettings(cfg, nil)

		assert.Zero(t, timeout.Timeout, "the HTTP client of the logs agent applies its own timeout")
		assert.Equal(t, cfg.QueueSettings, queue)
	})

	t.Run("sync sender is bounded by the client timeout and sizes the queue", func(t *testing.T) {
		cfg := newConfig(t, map[string]any{})

		timeout, queue := logsExporterHelperSettings(cfg, syncSettings)

		assert.Equal(t, 7*time.Second, timeout.Timeout)
		require.True(t, queue.HasValue())
		assert.Equal(t, 40, queue.Get().NumConsumers)
		require.True(t, queue.Get().Batch.HasValue())
		batch := queue.Get().Batch.Get()
		assert.Equal(t, exporterhelper.RequestSizerTypeItems, batch.Sizer)
		assert.Equal(t, int64(1000), batch.MinSize)
		assert.Equal(t, int64(1000), batch.MaxSize)
	})

	t.Run("sync sender keeps an explicit num_consumers", func(t *testing.T) {
		cfg := newConfig(t, map[string]any{"num_consumers": 3})

		_, queue := logsExporterHelperSettings(cfg, syncSettings)

		require.True(t, queue.HasValue())
		assert.Equal(t, 3, queue.Get().NumConsumers)
		assert.True(t, queue.Get().Batch.HasValue())
	})

	t.Run("sync sender leaves a disabled queue disabled", func(t *testing.T) {
		cfg := newConfig(t, map[string]any{})
		cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()

		_, queue := logsExporterHelperSettings(cfg, syncSettings)

		assert.False(t, queue.HasValue())
	})
}

func TestNewLogsAgentExporter_SyncSettings(t *testing.T) {
	intake := newFakeLogsIntake(t, alwaysStatus(http.StatusOK))
	newConfig := func() *datadogconfig.Config {
		return &datadogconfig.Config{
			Logs: datadogconfig.LogsConfig{
				TCPAddrConfig:    confignet.TCPAddrConfig{Endpoint: intake.URL},
				UseCompression:   true,
				CompressionLevel: 6,
				BatchWait:        1,
			},
		}
	}
	sourceProvider := &testutil.MockSourceProvider{Src: source.Source{Kind: source.HostnameKind, Identifier: "test-host", SourceIdentifier: source.SourceIdentifier{Primary: "test-host"}}}
	params := exportertest.NewNopSettings(metadata.Type)

	t.Run("sync sender", func(t *testing.T) {
		setSyncSenderGate(t, true)

		agent, exp, syncSettings, err := newLogsAgentExporter(t.Context(), params, newConfig(), sourceProvider, nil)

		require.NoError(t, err)
		assert.Nil(t, agent, "the asynchronous logs agent is not started")
		assert.NotNil(t, exp)
		require.NotNil(t, syncSettings)
		// The settings come from the configuration of the logs agent that the exporter builds.
		assert.Equal(t, min(4, runtime.GOMAXPROCS(0))*10, syncSettings.concurrency)
		assert.Equal(t, 1000, syncSettings.batchSize)
	})

	t.Run("asynchronous logs agent", func(t *testing.T) {
		setSyncSenderGate(t, false)

		agent, exp, syncSettings, err := newLogsAgentExporter(t.Context(), params, newConfig(), sourceProvider, nil)

		require.NoError(t, err)
		require.NotNil(t, agent)
		t.Cleanup(func() { _ = agent.Stop(t.Context()) })
		assert.NotNil(t, exp)
		assert.Nil(t, syncSettings)
	})
}

func TestLogsExporter_SyncSender_AppliesClientTimeout(t *testing.T) {
	setSyncSenderGate(t, true)
	release := make(chan struct{})
	slowIntake := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		<-release
	}))
	t.Cleanup(slowIntake.Close)
	t.Cleanup(func() { close(release) }) // runs before slowIntake.Close, which waits for the handler
	exp, _ := buildLogsExporter(t, slowIntake.URL, configretry.BackOffConfig{}, func(cfg *datadogconfig.Config) {
		cfg.ClientConfig.Timeout = 200 * time.Millisecond
	})

	start := time.Now()
	err := exp.ConsumeLogs(t.Context(), testLogs(1))

	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second, "ConsumeLogs must give up at the client timeout")
}

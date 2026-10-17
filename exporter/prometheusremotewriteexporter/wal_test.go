// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadatatest"
)

func doNothingExportSink(_ context.Context, reqL []*prompb.WriteRequest) error {
	_ = reqL
	return nil
}

func TestWALCreation_nilConfig(t *testing.T) {
	config := (*WALConfig)(nil)
	set := exportertest.NewNopSettings(metadata.Type)
	pwal, err := newWAL(config, set, doNothingExportSink)
	require.Nil(t, pwal)
	require.NoError(t, err)
}

func TestWALCreation_nonNilConfig(t *testing.T) {
	config := &WALConfig{Directory: t.TempDir()}
	set := exportertest.NewNopSettings(metadata.Type)
	pwal, err := newWAL(config, set, doNothingExportSink)
	require.NotNil(t, pwal)
	require.NoError(t, err)
	assert.NoError(t, pwal.stop())
}

func orderByLabelValueForEach(reqL []*prompb.WriteRequest) {
	for _, req := range reqL {
		orderByLabelValue(req)
	}
}

func orderByLabelValue(wreq *prompb.WriteRequest) {
	// Sort the timeSeries by their labels.
	type byLabelMessage struct {
		label  *prompb.Label
		sample *prompb.Sample
	}

	for i := range wreq.Timeseries {
		timeSeries := wreq.Timeseries[i]
		bMsgs := make([]*byLabelMessage, 0, len(wreq.Timeseries)*10)
		for i := range timeSeries.Labels {
			bMsgs = append(bMsgs, &byLabelMessage{
				label:  &timeSeries.Labels[i],
				sample: &timeSeries.Samples[i],
			})
		}
		sort.Slice(bMsgs, func(i, j int) bool {
			return bMsgs[i].label.Value < bMsgs[j].label.Value
		})

		for i := range bMsgs {
			timeSeries.Labels[i] = *bMsgs[i].label
			timeSeries.Samples[i] = *bMsgs[i].sample
		}
	}

	// Now finally sort stably by timeseries value for
	// which just .String() is good enough for comparison.
	sort.Slice(wreq.Timeseries, func(i, j int) bool {
		ti, tj := wreq.Timeseries[i], wreq.Timeseries[j]
		return ti.String() < tj.String()
	})
}

func TestWALStopManyTimes(t *testing.T) {
	tempDir := t.TempDir()
	config := &WALConfig{
		Directory:         tempDir,
		TruncateFrequency: 60 * time.Microsecond,
		BufferSize:        1,
	}
	set := exportertest.NewNopSettings(metadata.Type)
	pwal, err := newWAL(config, set, doNothingExportSink)
	require.NotNil(t, pwal)
	require.NoError(t, err)

	// Ensure that invoking .stop() multiple times doesn't cause a panic, but actually
	// First close should NOT return an error.
	require.NoError(t, pwal.stop())
	for range 4 {
		// Every invocation to .stop() should return an errAlreadyClosed.
		require.ErrorIs(t, pwal.stop(), errAlreadyClosed)
	}
}

func TestWAL_persist(t *testing.T) {
	// Unit tests that requests written to the WAL persist.
	config := &WALConfig{Directory: t.TempDir()}
	set := exportertest.NewNopSettings(metadata.Type)
	pwal, err := newWAL(config, set, doNothingExportSink)
	require.NotNil(t, pwal)
	require.NoError(t, err)

	// 1. Write out all the entries.
	reqL := []*prompb.WriteRequest{
		{
			Timeseries: []prompb.TimeSeries{
				{
					Labels:  []prompb.Label{{Name: "ts1l1", Value: "ts1k1"}},
					Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
				},
			},
		},
		{
			Timeseries: []prompb.TimeSeries{
				{
					Labels:  []prompb.Label{{Name: "ts2l1", Value: "ts2k1"}},
					Samples: []prompb.Sample{{Value: 2, Timestamp: 200}},
				},
				{
					Labels:  []prompb.Label{{Name: "ts1l1", Value: "ts1k1"}},
					Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
				},
			},
		},
	}

	ctx := t.Context()
	require.NoError(t, pwal.retrieveWALIndices())
	t.Cleanup(func() {
		assert.NoError(t, pwal.stop())
	})

	require.NoError(t, pwal.persistToWAL(ctx, reqL))
	require.Len(t, pwal.rNotify, 1)

	// 2. Read all the entries from the WAL itself, guided by the indices available,
	// and ensure that they are exactly in order as we'd expect them.
	wal := pwal.wal
	start, err := wal.FirstIndex()
	require.NoError(t, err)
	end, err := wal.LastIndex()
	require.NoError(t, err)

	var reqLFromWAL []*prompb.WriteRequest
	for i := start; i <= end; i++ {
		req, err := pwal.readPrompbFromWAL(ctx, i)
		require.NoError(t, err)
		reqLFromWAL = append(reqLFromWAL, req)
	}

	orderByLabelValueForEach(reqL)
	orderByLabelValueForEach(reqLFromWAL)
	require.Equal(t, reqLFromWAL[0], reqL[0])
	require.Equal(t, reqLFromWAL[1], reqL[1])
}

func TestExportWithWALEnabled(t *testing.T) {
	cfg := &Config{
		WAL: configoptional.Some(WALConfig{
			Directory:  t.TempDir(),
			BufferSize: 1,
		}),
		RemoteWriteProtoMsg: remoteapi.WriteV1MessageType,
	}
	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopSettings(metadata.Type)
	set.BuildInfo = buildInfo

	requestsReceived := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestsReceived.Add(1)
		assert.LessOrEqual(t, requestsReceived.Load(), int64(2), "Only two requests should be received")
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.NotNil(t, body)
		// Receives the http requests and unzip, unmarshalls, and extracts TimeSeries
		writeReq := &prompb.WriteRequest{}
		var unzipped []byte

		dest, err := snappy.Decode(unzipped, body)
		assert.NoError(t, err)

		ok := proto.Unmarshal(dest, writeReq)
		assert.NoError(t, ok)

		assert.Len(t, writeReq.Timeseries, 1)
		ts := writeReq.Timeseries[0]
		assert.Len(t, ts.Labels, 1)
		l := ts.Labels[0]
		assert.Equal(t, "__name__", l.Name)
		assert.Equal(t, "test_metric", l.Value)

		assert.Len(t, ts.Samples, 1)
		assert.Equal(t, 100*requestsReceived.Load(), ts.Samples[0].Timestamp)
	}))
	defer server.Close()

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.HTTP = clientConfig

	// Pickup any defaults applied during validation
	require.NoError(t, cfg.Validate())

	prwe, err := newPRWExporter(cfg, set)
	assert.NoError(t, err)
	assert.NotNil(t, prwe)
	err = prwe.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, prwe.client)

	metrics := map[string]*prompb.TimeSeries{
		"test_metric": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
		},
	}
	assert.NoError(t, prwe.handleExport(t.Context(), metrics, nil))

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, int64(1), requestsReceived.Load())
	}, 5*time.Second, 10*time.Millisecond, "First metric was not received")

	metrics["test_metric"].Samples[0].Timestamp = 200
	assert.NoError(t, prwe.handleExport(t.Context(), metrics, nil))

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Equal(t, int64(2), requestsReceived.Load())
	}, 5*time.Second, 10*time.Millisecond, "Second metric was not received")

	// While on Unix systems, t.TempDir() would easily close the WAL files,
	// on Windows, it doesn't. So we need to close it manually to avoid flaky tests.
	err = prwe.Shutdown(t.Context())
	assert.NoError(t, err)
}

func TestWALWrite_Telemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tel.Shutdown(context.Background())) //nolint:usetesting
	})
	set := metadatatest.NewSettings(tel)

	cfg := &Config{
		WAL: configoptional.Some(WALConfig{
			Directory: t.TempDir(),
		}),
		RemoteWriteProtoMsg: remoteapi.WriteV2MessageType,
	}

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Do nothing
	}))
	defer server.Close()

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.HTTP = clientConfig

	prw, err := newPRWExporter(cfg, set)
	require.NotNil(t, prw)
	require.NoError(t, err)

	err = prw.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, prw.Shutdown(context.Background())) //nolint:usetesting
	})
	wal := prw.wal

	// Create some test data
	metrics := map[string]*prompb.TimeSeries{
		"test_metric": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
		},
	}

	// Test successful WAL write
	err = prw.handleExport(t.Context(), metrics, nil)
	require.NoError(t, err)
	exporterAttr := attribute.NewSet(attribute.String("exporter", set.ID.String()))
	metadatatest.AssertEqualExporterPrometheusremotewriteWalWrites(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1, Attributes: exporterAttr}},
		metricdatatest.IgnoreTimestamp())

	// Test failed WAL write by causing an out-of-order write error
	currentIndex := wal.wWALIndex.Load()
	wal.wWALIndex.Store(currentIndex - 1)

	err = prw.handleExport(t.Context(), metrics, nil)
	require.Error(t, err)
	metadatatest.AssertEqualExporterPrometheusremotewriteWalWritesFailures(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1, Attributes: exporterAttr}},
		metricdatatest.IgnoreTimestamp())

	_, err = tel.GetMetric("otelcol_exporter_prometheusremotewrite_wal_write_latency")
	require.NoError(t, err)

	_, err = tel.GetMetric("otelcol_exporter_prometheusremotewrite_wal_bytes_written")
	require.NoError(t, err)
}

func TestWALRead_Telemetry(t *testing.T) {
	// Skip flaky test in CI, because it's flaky and hard to reliably test; still useful for local testing.
	t.Skip("Skipping in CI: test is flaky;still useful for local testing")
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tel.Shutdown(context.Background())) //nolint:usetesting
	})
	set := metadatatest.NewSettings(tel)

	// Create a temporary directory for the WAL
	tempDir := t.TempDir()
	cfg := &Config{
		WAL: configoptional.Some(WALConfig{
			BufferSize: 1,
			Directory:  tempDir,
		}),
		RemoteWriteProtoMsg: remoteapi.WriteV2MessageType,
	}

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Do nothing
	}))
	defer server.Close()

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.HTTP = clientConfig

	prw, err := newPRWExporter(cfg, set)
	require.NotNil(t, prw)
	require.NoError(t, err)

	err = prw.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, prw.Shutdown(context.Background())) //nolint:usetesting
	})

	// Verify initial WAL reads metric
	metadatatest.AssertEqualExporterPrometheusremotewriteWalReads(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp())
	wal := prw.wal

	// Create some test data
	metrics := map[string]*prompb.TimeSeries{
		"test_metric": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
		},
	}

	// Write a successful WAL write first
	err = prw.handleExport(t.Context(), metrics, nil)
	require.NoError(t, err)
	err = wal.wal.Close()
	require.NoError(t, err)

	// Write corrupted data
	corruptedData := []byte{0x80}
	firstWalFile := filepath.Join(wal.walPath, "00000000000000000001")
	err = os.WriteFile(firstWalFile, corruptedData, 0o600)
	require.NoError(t, err)
	// write the corrupted data and start reading from the index

	err = prw.Start(t.Context(), componenttest.NewNopHost())
	// Unable to start the WAL cause there is a corrupted entry
	require.Error(t, err)
	_, err = tel.GetMetric("otelcol_exporter_prometheusremotewrite_wal_reads_failures")

	// verify that the metric exists, so it's incremented
	require.NoError(t, err)

	_, err = tel.GetMetric("otelcol_exporter_prometheusremotewrite_wal_read_latency")
	require.NoError(t, err)

	_, err = tel.GetMetric("otelcol_exporter_prometheusremotewrite_wal_bytes_read")
	require.NoError(t, err)
}

func TestWALLag_Telemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tel.Shutdown(context.Background())) //nolint:usetesting
	})
	set := metadatatest.NewSettings(tel)

	cfg := &Config{
		WAL: configoptional.Some(WALConfig{
			Directory:          t.TempDir(),
			BufferSize:         1,
			LagRecordFrequency: 10 * time.Millisecond, // Very short interval for testing
		}),
		RemoteWriteProtoMsg: remoteapi.WriteV2MessageType,
	}

	// Create a server that will be slow to process requests (to create lag)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Do nothing
	}))

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.HTTP = clientConfig

	prw, err := newPRWExporter(cfg, set)
	require.NotNil(t, prw)
	require.NoError(t, err)

	err = prw.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, prw.Shutdown(context.Background())) //nolint:usetesting
		server.Close()
	})

	// Create test data to write to WAL
	metrics := map[string]*prompb.TimeSeries{
		"test_metric_1": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric_1"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
		},
	}

	// Write multiple metrics to create lag (wIndex will be ahead of rIndex)
	err = prw.handleExport(t.Context(), metrics, nil)
	require.NoError(t, err)

	// Wait for lag recording to happen (longer than lagRecordFrequency)
	time.Sleep(5 * cfg.WAL.Get().LagRecordFrequency)

	// The wal_lag metric must carry the exporter attribute set to the
	// component ID supplied by the test settings. We ideally would use
	// otelcol.component.id, but the rest of the PRW exporter self-observability
	// metrics currently use "exporter"; this can be switched for the whole
	// exporter at a future point.
	exporterAttr := attribute.NewSet(attribute.String("exporter", set.ID.String()))
	metadatatest.AssertEqualExporterPrometheusremotewriteWalLag(t, tel,
		[]metricdata.DataPoint[int64]{{Attributes: exporterAttr}},
		metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
}

// TestWAL_IdleFlush verifies that buffered WAL entries are flushed to the
// backend even when no further data arrives. With BufferSize larger than the
// number of written entries, the only path that can deliver the data is the
// idle read-timeout in readPrompbFromWAL firing and the truncation timer in
// continuallyPopWALThenExport then flushing the buffered request. This guards
// against the "buffered data stall on idle" regression.
func TestWAL_IdleFlush(t *testing.T) {
	requestsReceived := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		requestsReceived.Add(1)
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.NotNil(t, body)
	}))

	cfg := &Config{
		WAL: configoptional.Some(WALConfig{
			Directory: t.TempDir(),
			// BufferSize deliberately larger than the single entry we write,
			// so an export is NOT triggered by the buffer filling up. The only
			// way the data is delivered is via the idle read-timeout + truncation
			// timer flush path.
			BufferSize: 100,
			// Short truncate frequency so the idle flush happens quickly. The
			// idle read-wait timeout is truncate_frequency/2.
			TruncateFrequency: 200 * time.Millisecond,
		}),
		RemoteWriteProtoMsg: remoteapi.WriteV1MessageType,
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.HTTP = clientConfig
	require.NoError(t, cfg.Validate())

	set := exportertest.NewNopSettings(metadata.Type)
	prwe, err := newPRWExporter(cfg, set)
	require.NoError(t, err)
	require.NotNil(t, prwe)

	require.NoError(t, prwe.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, prwe.Shutdown(context.Background())) //nolint:usetesting
		server.Close()
	})

	metrics := map[string]*prompb.TimeSeries{
		"test_metric": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
		},
	}
	// Write a single entry, then stay idle (no further writes). The entry must
	// still be delivered via the idle-flush path within a few truncate cycles.
	require.NoError(t, prwe.handleExport(t.Context(), metrics, nil))

	assert.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.GreaterOrEqual(t, requestsReceived.Load(), int64(1))
	}, 5*time.Second, 10*time.Millisecond, "buffered WAL entry was not flushed while idle")
}

// decodeWriteRequest unpacks a snappy-compressed remote-write body.
func decodeWriteRequest(tb testing.TB, body []byte) *prompb.WriteRequest {
	tb.Helper()
	var unzipped []byte
	dest, err := snappy.Decode(unzipped, body)
	require.NoError(tb, err)
	req := &prompb.WriteRequest{}
	require.NoError(tb, proto.Unmarshal(dest, req))
	return req
}

// TestWALUnblocksAfterNonRetryableRejection is the regression test for the WAL
// head-of-line block: a record the endpoint rejects with a non-retryable status
// must be dropped so the records queued behind it can still be delivered.
//
// The endpoint here behaves like Mimir with a sample that fell outside its
// in-order acceptance window: the stale sample is rejected permanently, and any
// request carrying it is rejected whole. Before the fix the read loop rewound to
// that record forever, so every later record was resent alongside it and nothing
// was ever accepted again.
func TestWALUnblocksAfterNonRetryableRejection(t *testing.T) {
	const staleTimestamp, freshTimestamp = int64(100), int64(200)

	accepted := &sync.Map{}
	rejections := &atomic.Int64{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		req := decodeWriteRequest(t, body)

		var timestamps []int64
		for _, ts := range req.Timeseries {
			for _, s := range ts.Samples {
				timestamps = append(timestamps, s.Timestamp)
			}
		}
		if slices.Contains(timestamps, staleTimestamp) {
			rejections.Add(1)
			http.Error(w, "err-mimir-sample-timestamp-too-old", http.StatusBadRequest)
			return
		}
		for _, got := range timestamps {
			accepted.Store(got, struct{}{})
		}
	}))

	cfg := &Config{
		WAL: configoptional.Some(WALConfig{
			Directory:         t.TempDir(),
			BufferSize:        1,
			TruncateFrequency: 100 * time.Millisecond,
		}),
		RemoteWriteProtoMsg: remoteapi.WriteV1MessageType,
	}
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.HTTP = clientConfig
	require.NoError(t, cfg.Validate())

	prwe, err := newPRWExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, prwe.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, prwe.Shutdown(context.Background())) //nolint:usetesting
		server.Close()
	})

	metrics := map[string]*prompb.TimeSeries{
		"test_metric": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: staleTimestamp}},
		},
	}
	require.NoError(t, prwe.handleExport(t.Context(), metrics, nil))

	// Wait until the stale record has actually been rejected at least once.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Positive(c, rejections.Load())
	}, 5*time.Second, 10*time.Millisecond, "stale record was never rejected")

	// A fresh record is written behind the rejected one.
	metrics["test_metric"].Samples[0].Timestamp = freshTimestamp
	require.NoError(t, prwe.handleExport(t.Context(), metrics, nil))

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		_, ok := accepted.Load(freshTimestamp)
		assert.True(c, ok)
	}, 10*time.Second, 20*time.Millisecond,
		"record queued behind the rejected one never reached the endpoint: the WAL head is blocked")
}

// TestWALRetainsBatchOnRetryableError covers the other direction end to end: a
// 5xx outage that outlasts the retry budget must not drop the record. It has to
// survive in the WAL and be delivered once the endpoint recovers.
func TestWALRetainsBatchOnRetryableError(t *testing.T) {
	const timestamp = int64(100)

	down := &atomic.Bool{}
	down.Store(true)
	attempts := &atomic.Int64{}
	accepted := &atomic.Int64{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		if down.Load() {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		for _, ts := range decodeWriteRequest(t, body).Timeseries {
			for _, sample := range ts.Samples {
				if sample.Timestamp == timestamp {
					accepted.Add(1)
				}
			}
		}
	}))

	cfg := &Config{
		WAL: configoptional.Some(WALConfig{
			Directory:         t.TempDir(),
			BufferSize:        1,
			TruncateFrequency: 100 * time.Millisecond,
		}),
		RemoteWriteProtoMsg: remoteapi.WriteV1MessageType,
		// A finite, short retry budget, so execute() gives up quickly and
		// reports the 503 as a permanent error.
		BackOffConfig: configretry.BackOffConfig{
			Enabled:         true,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     10 * time.Millisecond,
			MaxElapsedTime:  50 * time.Millisecond,
			Multiplier:      1.5,
		},
	}
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.HTTP = clientConfig
	require.NoError(t, cfg.Validate())

	prwe, err := newPRWExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, prwe.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, prwe.Shutdown(context.Background())) //nolint:usetesting
		server.Close()
	})

	metrics := map[string]*prompb.TimeSeries{
		"test_metric": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: timestamp}},
		},
	}
	require.NoError(t, prwe.handleExport(t.Context(), metrics, nil))

	// The outage outlasts the retry budget several times over.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Positive(c, attempts.Load())
	}, 5*time.Second, 10*time.Millisecond, "record was never attempted")
	time.Sleep(time.Second)

	down.Store(false)

	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Positive(c, accepted.Load())
	}, 10*time.Second, 20*time.Millisecond,
		"record was discarded during a retryable outage instead of being replayed")
}

// TestExportThenFrontTruncateWAL_RetentionByErrorKind pins down why this fix
// classifies on the rejection itself rather than on consumererror.IsPermanent.
// IsPermanent is also true for a cancelled export and for a batch that mixes a
// rejected request with a retryable one, and in both cases the data was never
// accepted, so truncating it loses writes the WAL exists to protect.
func TestExportThenFrontTruncateWAL_RetentionByErrorKind(t *testing.T) {
	rejected := func() error {
		err := &nonRetryableStatusError{StatusCode: 400, err: errors.New("remote write request failed")}
		return consumererror.NewPermanent(err)
	}

	tests := []struct {
		name         string
		sinkErr      error
		wantTruncate bool
	}{
		{
			name:         "rejected payload is dropped",
			sinkErr:      rejected(),
			wantTruncate: true,
		},
		{
			name:         "cancelled export is retained",
			sinkErr:      consumererror.NewPermanent(context.Canceled),
			wantTruncate: false,
		},
		{
			name:         "batch mixing a rejection and a retryable failure is retained",
			sinkErr:      multierr.Append(rejected(), errors.New("remote write request failed")),
			wantTruncate: false,
		},
		{
			name:         "retryable failure is retained",
			sinkErr:      errors.New("remote write request failed"),
			wantTruncate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pwal, err := newWAL(&WALConfig{Directory: t.TempDir()},
				exportertest.NewNopSettings(metadata.Type),
				func(context.Context, []*prompb.WriteRequest) error { return tt.sinkErr })
			require.NoError(t, err)
			require.NoError(t, pwal.retrieveWALIndices())
			t.Cleanup(func() { assert.NoError(t, pwal.stop()) })

			reqL := []*prompb.WriteRequest{{Timeseries: []prompb.TimeSeries{{
				Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
				Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
			}}}}
			ctx := t.Context()
			require.NoError(t, pwal.persistToWAL(ctx, reqL))
			_, err = pwal.readPrompbFromWAL(ctx, pwal.rWALIndex.Load())
			require.NoError(t, err)

			exportErr := pwal.exportThenFrontTruncateWAL(ctx, reqL)

			pwal.mu.Lock()
			first, fErr := pwal.wal.FirstIndex()
			last, lErr := pwal.wal.LastIndex()
			pwal.mu.Unlock()
			require.NoError(t, fErr)
			require.NoError(t, lErr)

			if tt.wantTruncate {
				assert.NoError(t, exportErr, "a rejected batch is handled, not reported upwards")
				assert.Greater(t, first, last,
					"the rejected record must be truncated so it stops blocking the WAL head")
				return
			}
			assert.Error(t, exportErr, "a batch that was never accepted must be reported upwards")
			assert.Equal(t, uint64(1), first, "the record must be retained for a later retry")
		})
	}
}

func TestAllNonRetryable(t *testing.T) {
	nonRetryable := func() error {
		return &nonRetryableStatusError{StatusCode: 400, err: errors.New("remote write request failed")}
	}
	transient := errors.New("connection refused")

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "bare non-retryable", err: nonRetryable(), want: true},
		{name: "bare transient", err: transient, want: false},
		{
			name: "permanent-wrapped non-retryable",
			err:  consumererror.NewPermanent(nonRetryable()),
			want: true,
		},
		{
			name: "permanent-wrapped transient",
			err:  consumererror.NewPermanent(transient),
			want: false,
		},
		{
			name: "multierr of non-retryable only",
			err:  multierr.Append(nonRetryable(), nonRetryable()),
			want: true,
		},
		{
			name: "multierr mixing non-retryable and transient",
			err:  multierr.Append(nonRetryable(), consumererror.NewPermanent(transient)),
			want: false,
		},
		{
			name: "nested multierr of non-retryable only",
			err: multierr.Append(
				multierr.Append(nonRetryable(), nonRetryable()),
				nonRetryable(),
			),
			want: true,
		},
		{
			name: "nested multierr hiding a transient error",
			err: multierr.Append(
				multierr.Append(nonRetryable(), transient),
				nonRetryable(),
			),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, allNonRetryable(tt.err))
		})
	}
}

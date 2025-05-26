// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

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

	for _, timeSeries := range wreq.Timeseries {
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
	for i := 0; i < 4; i++ {
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

	ctx := context.Background()
	require.NoError(t, pwal.retrieveWALIndices())
	t.Cleanup(func() {
		assert.NoError(t, pwal.stop())
	})

	require.NoError(t, pwal.persistToWAL(reqL))

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
		WAL: &WALConfig{
			Directory: t.TempDir(),
		},
		TargetInfo:          &TargetInfo{}, // Declared just to avoid nil pointer dereference.
		RemoteWriteProtoMsg: config.RemoteWriteProtoMsgV1,
	}
	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopSettings(metadata.Type)
	set.BuildInfo = buildInfo

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
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
	}))
	defer server.Close()

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.ClientConfig = clientConfig

	prwe, err := newPRWExporter(cfg, set)
	assert.NoError(t, err)
	assert.NotNil(t, prwe)
	err = prwe.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, prwe.client)

	metrics := map[string]*prompb.TimeSeries{
		"test_metric": {
			Labels:  []prompb.Label{{Name: "__name__", Value: "test_metric"}},
			Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
		},
	}
	err = prwe.handleExport(context.Background(), metrics, nil)
	assert.NoError(t, err)

	// While on Unix systems, t.TempDir() would easily close the WAL files,
	// on Windows, it doesn't. So we need to close it manually to avoid flaky tests.
	err = prwe.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestWAL_Telemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tel.Shutdown(context.Background()))
	})
	set := metadatatest.NewSettings(tel)

	cfg := &Config{
		WAL: &WALConfig{
			Directory: t.TempDir(),
		},
		TargetInfo:          &TargetInfo{}, // Declared just to avoid nil pointer dereference.
		RemoteWriteProtoMsg: config.RemoteWriteProtoMsgV2,
	}

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// Do nothing
	}))
	defer server.Close()

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	cfg.ClientConfig = clientConfig

	prw, err := newPRWExporter(cfg, set)
	require.NotNil(t, prw)
	require.NoError(t, err)

	err = prw.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, prw.Shutdown(context.Background()))
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
	err = prw.handleExport(context.Background(), metrics, nil)
	require.NoError(t, err)
	metadatatest.AssertEqualExporterPrometheusremotewriteWalWrites(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp())

	// Test failed WAL write by causing an out-of-order write error
	currentIndex := wal.wWALIndex.Load()
	wal.wWALIndex.Store(currentIndex - 1)

	err = prw.handleExport(context.Background(), metrics, nil)
	require.Error(t, err)
	metadatatest.AssertEqualExporterPrometheusremotewriteWalWritesFailures(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp())
}

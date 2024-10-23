// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

// Test_PushMetrics checks the number of TimeSeries received by server and the number of metrics dropped is the same as
// expected
func Test_PushMetricsV2(t *testing.T) {

	// success cases
	intSumBatch := testdata.GenerateMetricsManyMetricsSameResource(10)

	sumBatch := getMetricsFromMetricList(validMetrics1[validSum], validMetrics2[validSum])

	intGaugeBatch := getMetricsFromMetricList(validMetrics1[validIntGauge], validMetrics2[validIntGauge])

	doubleGaugeBatch := getMetricsFromMetricList(validMetrics1[validDoubleGauge], validMetrics2[validDoubleGauge])

	checkFunc := func(t *testing.T, r *http.Request, expected int, isStaleMarker bool) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, len(body))
		dest, err := snappy.Decode(buf, body)
		assert.Equal(t, "0.1.0", r.Header.Get("x-prometheus-remote-write-version"))
		assert.Equal(t, "snappy", r.Header.Get("content-encoding"))
		assert.Equal(t, "opentelemetry-collector/1.0", r.Header.Get("User-Agent"))
		assert.NotNil(t, r.Header.Get("tenant-id"))
		require.NoError(t, err)
		wr := &prompb.WriteRequest{}
		ok := proto.Unmarshal(dest, wr)
		require.NoError(t, ok)
		assert.Len(t, wr.Timeseries, expected)
		if isStaleMarker {
			assert.True(t, value.IsStaleNaN(wr.Timeseries[0].Samples[0].Value))
		}
	}

	tests := []struct {
		name                       string
		metrics                    pmetric.Metrics
		reqTestFunc                func(t *testing.T, r *http.Request, expected int, isStaleMarker bool)
		expectedTimeSeries         int
		httpResponseCode           int
		returnErr                  bool
		isStaleMarker              bool
		skipForWAL                 bool
		expectedFailedTranslations int
	}{
		{
			name:               "intSum_case",
			metrics:            intSumBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 4,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "doubleSum_case",
			metrics:            sumBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 2,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "doubleGauge_case",
			metrics:            doubleGaugeBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 2,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "intGauge_case",
			metrics:            intGaugeBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 2,
			httpResponseCode:   http.StatusAccepted,
		},
	}

	for _, useWAL := range []bool{true, false} {
		name := "NoWAL"
		if useWAL {
			name = "WAL"
		}
		t.Run(name, func(t *testing.T) {
			if useWAL {
				t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9124")
			}
			for _, ttt := range tests {
				tt := ttt
				if useWAL && tt.skipForWAL {
					t.Skip("test not supported when using WAL")
				}
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if tt.reqTestFunc != nil {
							tt.reqTestFunc(t, r, tt.expectedTimeSeries, tt.isStaleMarker)
						}
						w.WriteHeader(tt.httpResponseCode)
					}))

					defer server.Close()

					// Adjusted retry settings for faster testing
					retrySettings := configretry.BackOffConfig{
						Enabled:         true,
						InitialInterval: 100 * time.Millisecond, // Shorter initial interval
						MaxInterval:     1 * time.Second,        // Shorter max interval
						MaxElapsedTime:  2 * time.Second,        // Shorter max elapsed time
					}
					clientConfig := confighttp.NewDefaultClientConfig()
					clientConfig.Endpoint = server.URL
					clientConfig.ReadBufferSize = 0
					clientConfig.WriteBufferSize = 512 * 1024
					cfg := &Config{
						Namespace:         "",
						ClientConfig:      clientConfig,
						MaxBatchSizeBytes: 3000000,
						RemoteWriteQueue:  RemoteWriteQueue{NumConsumers: 1},
						SendMetadata:      true,
						SendRW2:           true,
						TargetInfo: &TargetInfo{
							Enabled: true,
						},
						CreatedMetric: &CreatedMetric{
							Enabled: true,
						},
						BackOffConfig: retrySettings,
					}

					if useWAL {
						cfg.WAL = &WALConfig{
							Directory: t.TempDir(),
						}
					}

					assert.NotNil(t, cfg)
					buildInfo := component.BuildInfo{
						Description: "OpenTelemetry Collector",
						Version:     "1.0",
					}
					tel := setupTestTelemetry()
					set := tel.NewSettings()
					mp := set.LeveledMeterProvider(configtelemetry.LevelBasic)
					set.LeveledMeterProvider = func(level configtelemetry.Level) metric.MeterProvider {
						// detailed level enables otelhttp client instrumentation which we
						// dont want to test here
						if level == configtelemetry.LevelDetailed {
							return noop.MeterProvider{}
						}
						return mp
					}
					set.BuildInfo = buildInfo

					prwe, nErr := newPRWExporter(cfg, set)

					require.NoError(t, nErr)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					require.NoError(t, prwe.Start(ctx, componenttest.NewNopHost()))
					defer func() {
						require.NoError(t, prwe.Shutdown(ctx))
					}()
					err := prwe.PushMetrics(ctx, tt.metrics)
					if tt.returnErr {
						assert.Error(t, err)
						return
					}
					expectedMetrics := []metricdata.Metrics{}
					if tt.expectedFailedTranslations > 0 {
						expectedMetrics = append(expectedMetrics, metricdata.Metrics{
							Name:        "otelcol_exporter_prometheusremotewrite_failed_translations",
							Description: "Number of translation operations that failed to translate metrics from Otel to Prometheus",
							Unit:        "1",
							Data: metricdata.Sum[int64]{
								Temporality: metricdata.CumulativeTemporality,
								IsMonotonic: true,
								DataPoints: []metricdata.DataPoint[int64]{
									{
										Value:      int64(tt.expectedFailedTranslations),
										Attributes: attribute.NewSet(attribute.String("exporter", "prometheusremotewrite")),
									},
								},
							},
						})
					}

					expectedMetrics = append(expectedMetrics, metricdata.Metrics{
						Name:        "otelcol_exporter_prometheusremotewrite_translated_time_series",
						Description: "Number of Prometheus time series that were translated from OTel metrics",
						Unit:        "1",
						Data: metricdata.Sum[int64]{
							Temporality: metricdata.CumulativeTemporality,
							IsMonotonic: true,
							DataPoints: []metricdata.DataPoint[int64]{
								{
									Value:      int64(tt.expectedTimeSeries),
									Attributes: attribute.NewSet(attribute.String("exporter", "prometheusremotewrite")),
								},
							},
						},
					})
					tel.assertMetrics(t, expectedMetrics)
					assert.NoError(t, err)
				})
			}
		})
	}
}

/* func TestSendingFromMetricsV2(t *testing.T) {
	settings := Settings{
		Namespace:           "",
		ExternalLabels:      nil,
		DisableTargetInfo:   false,
		ExportCreatedMetric: false,
		AddMetricSuffixes:   false,
		SendMetadata:        false,
	}

	ts := uint64(time.Now().UnixNano())
	payload := createExportRequest(5, 0, 1, 3, 0, pcommon.Timestamp(ts))

	tsMap, symbolsTable, err := FromMetricsV2(payload.Metrics(), settings)
	require.NoError(t, err)

	tsArray := make([]writev2.TimeSeries, 0, len(tsMap))
	for _, ts := range tsMap {
		tsArray = append(tsArray, *ts)
	}

	writeReq := &writev2.Request{
		Symbols:    symbolsTable.Symbols(),
		Timeseries: tsArray,
	}

	data, errMarshal := proto.Marshal(writeReq)
	require.NoError(t, errMarshal)

	compressedData := snappy.Encode(nil, data)

	req, err := http.NewRequest("POST", "http://localhost:9091/api/v1/write", bytes.NewReader(compressedData))
	require.NoError(t, errMarshal)

	c := http.Client{}
	resp, err := c.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

} */

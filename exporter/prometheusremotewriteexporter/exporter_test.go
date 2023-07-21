// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"


	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

// Test_NewPRWExporter checks that a new exporter instance with non-nil fields is initialized
func Test_NewPRWExporter(t *testing.T) {
	cfg := &Config{
		TimeoutSettings:    exporterhelper.TimeoutSettings{},
		RetrySettings:      exporterhelper.RetrySettings{},
		Namespace:          "",
		ExternalLabels:     map[string]string{},
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: ""},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
		CreatedMetric: &CreatedMetric{
			Enabled: false,
		},
	}
	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopCreateSettings()
	set.BuildInfo = buildInfo

	tests := []struct {
		name                string
		config              *Config
		namespace           string
		endpoint            string
		concurrency         int
		externalLabels      map[string]string
		returnErrorOnCreate bool
		set                 exporter.CreateSettings
	}{
		{
			name:                "invalid_URL",
			config:              cfg,
			namespace:           "test",
			endpoint:            "invalid URL",
			concurrency:         5,
			externalLabels:      map[string]string{"Key1": "Val1"},
			returnErrorOnCreate: true,
			set:                 set,
		},
		{
			name:                "invalid_labels_case",
			config:              cfg,
			namespace:           "test",
			endpoint:            "http://some.url:9411/api/prom/push",
			concurrency:         5,
			externalLabels:      map[string]string{"Key1": ""},
			returnErrorOnCreate: true,
			set:                 set,
		},
		{
			name:           "success_case",
			config:         cfg,
			namespace:      "test",
			endpoint:       "http://some.url:9411/api/prom/push",
			concurrency:    5,
			externalLabels: map[string]string{"Key1": "Val1"},
			set:            set,
		},
		{
			name:           "success_case_no_labels",
			config:         cfg,
			namespace:      "test",
			endpoint:       "http://some.url:9411/api/prom/push",
			concurrency:    5,
			externalLabels: map[string]string{},
			set:            set,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.HTTPClientSettings.Endpoint = tt.endpoint
			cfg.ExternalLabels = tt.externalLabels
			cfg.Namespace = tt.namespace
			cfg.RemoteWriteQueue.NumConsumers = 1
			prwe, err := newPRWExporter(cfg, tt.set)

			if tt.returnErrorOnCreate {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			require.NotNil(t, prwe)
			assert.NotNil(t, prwe.exporterSettings)
			assert.NotNil(t, prwe.endpointURL)
			assert.NotNil(t, prwe.closeChan)
			assert.NotNil(t, prwe.wg)
			assert.NotNil(t, prwe.userAgentHeader)
			assert.NotNil(t, prwe.clientSettings)
		})
	}
}

// Test_Start checks if the client is properly created as expected.
func Test_Start(t *testing.T) {
	cfg := &Config{
		TimeoutSettings: exporterhelper.TimeoutSettings{},
		RetrySettings:   exporterhelper.RetrySettings{},
		Namespace:       "",
		ExternalLabels:  map[string]string{},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
		CreatedMetric: &CreatedMetric{
			Enabled: false,
		},
	}
	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopCreateSettings()
	set.BuildInfo = buildInfo
	tests := []struct {
		name                 string
		config               *Config
		namespace            string
		concurrency          int
		externalLabels       map[string]string
		returnErrorOnStartUp bool
		set                  exporter.CreateSettings
		endpoint             string
		clientSettings       confighttp.HTTPClientSettings
	}{
		{
			name:           "success_case",
			config:         cfg,
			namespace:      "test",
			concurrency:    5,
			externalLabels: map[string]string{"Key1": "Val1"},
			set:            set,
			clientSettings: confighttp.HTTPClientSettings{Endpoint: "https://some.url:9411/api/prom/push"},
		},
		{
			name:                 "invalid_tls",
			config:               cfg,
			namespace:            "test",
			concurrency:          5,
			externalLabels:       map[string]string{"Key1": "Val1"},
			set:                  set,
			returnErrorOnStartUp: true,
			clientSettings: confighttp.HTTPClientSettings{
				Endpoint: "https://some.url:9411/api/prom/push",
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile:   "non-existent file",
						CertFile: "",
						KeyFile:  "",
					},
					Insecure:   false,
					ServerName: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.ExternalLabels = tt.externalLabels
			cfg.Namespace = tt.namespace
			cfg.RemoteWriteQueue.NumConsumers = 1
			cfg.HTTPClientSettings = tt.clientSettings

			prwe, err := newPRWExporter(cfg, tt.set)
			assert.NoError(t, err)
			assert.NotNil(t, prwe)

			err = prwe.Start(context.Background(), componenttest.NewNopHost())
			if tt.returnErrorOnStartUp {
				assert.Error(t, err)
				return
			}
			assert.NotNil(t, prwe.client)
		})
	}
}

// Test_Shutdown checks after Shutdown is called, incoming calls to PushMetrics return error.
func Test_Shutdown(t *testing.T) {
	prwe := &prwExporter{
		wg:        new(sync.WaitGroup),
		closeChan: make(chan struct{}),
	}
	wg := new(sync.WaitGroup)
	err := prwe.Shutdown(context.Background())
	require.NoError(t, err)
	errChan := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errChan <- prwe.PushMetrics(context.Background(), pmetric.NewMetrics())
		}()
	}
	wg.Wait()
	close(errChan)
	for ok := range errChan {
		assert.Error(t, ok)
	}
}

// Test whether or not the Server receives the correct TimeSeries.
// Currently considering making this test an iterative for loop of multiple TimeSeries much akin to Test_PushMetrics
func Test_export(t *testing.T) {
	// First we will instantiate a dummy TimeSeries instance to pass into both the export call and compare the http request
	labels := getPromLabels(label11, value11, label12, value12, label21, value21, label22, value22)
	sample1 := getSample(floatVal1, msTime1)
	sample2 := getSample(floatVal2, msTime2)
	ts1 := getTimeSeries(labels, sample1, sample2)
	handleFunc := func(w http.ResponseWriter, r *http.Request, code int) {
		// The following is a handler function that reads the sent httpRequest, unmarshal, and checks if the WriteRequest
		// preserves the TimeSeries data correctly
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		require.NotNil(t, body)
		// Receives the http requests and unzip, unmarshalls, and extracts TimeSeries
		assert.Equal(t, "0.1.0", r.Header.Get("X-Prometheus-Remote-Write-Version"))
		assert.Equal(t, "snappy", r.Header.Get("Content-Encoding"))
		assert.Equal(t, "opentelemetry-collector/1.0", r.Header.Get("User-Agent"))
		writeReq := &prompb.WriteRequest{}
		var unzipped []byte

		dest, err := snappy.Decode(unzipped, body)
		require.NoError(t, err)

		ok := proto.Unmarshal(dest, writeReq)
		require.NoError(t, ok)

		assert.EqualValues(t, 1, len(writeReq.Timeseries))
		require.NotNil(t, writeReq.GetTimeseries())
		assert.Equal(t, *ts1, writeReq.GetTimeseries()[0])
		w.WriteHeader(code)
	}

	// Create in test table format to check if different HTTP response codes or server errors
	// are properly identified
	tests := []struct {
		name                string
		ts                  prompb.TimeSeries
		serverUp            bool
		httpResponseCode    int
		returnErrorOnCreate bool
	}{
		{"success_case",
			*ts1,
			true,
			http.StatusAccepted,
			false,
		},
		{
			"server_no_response_case",
			*ts1,
			false,
			http.StatusAccepted,
			false,
		}, {
			"error_status_code_case",
			*ts1,
			true,
			http.StatusForbidden,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if handleFunc != nil {
					handleFunc(w, r, tt.httpResponseCode)
				}
			}))
			defer server.Close()
			serverURL, uErr := url.Parse(server.URL)
			assert.NoError(t, uErr)
			if !tt.serverUp {
				server.Close()
			}
			err := runExportPipeline(ts1, serverURL)
			if tt.returnErrorOnCreate {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestNoMetricsNoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	serverURL, uErr := url.Parse(server.URL)
	assert.NoError(t, uErr)
	assert.NoError(t, runExportPipeline(nil, serverURL))
}

func runExportPipeline(ts *prompb.TimeSeries, endpoint *url.URL) error {
	// First we will construct a TimeSeries array from the testutils package
	testmap := make(map[string]*prompb.TimeSeries)
	if ts != nil {
		testmap["test"] = ts
	}

	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = endpoint.String()
	cfg.RemoteWriteQueue.NumConsumers = 1

	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopCreateSettings()
	set.BuildInfo = buildInfo
	// after this, instantiate a CortexExporter with the current HTTP client and endpoint set to passed in endpoint
	prwe, err := newPRWExporter(cfg, set)
	if err != nil {
		return err
	}

	if err = prwe.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		return err
	}

	return prwe.handleExport(context.Background(), testmap)
}

// Test_PushMetrics checks the number of TimeSeries received by server and the number of metrics dropped is the same as
// expected
func Test_PushMetrics(t *testing.T) {

	invalidTypeBatch := testdata.GenerateMetricsMetricTypeInvalid()

	// success cases
	intSumBatch := testdata.GenerateMetricsManyMetricsSameResource(10)

	sumBatch := getMetricsFromMetricList(validMetrics1[validSum], validMetrics2[validSum])

	intGaugeBatch := getMetricsFromMetricList(validMetrics1[validIntGauge], validMetrics2[validIntGauge])

	doubleGaugeBatch := getMetricsFromMetricList(validMetrics1[validDoubleGauge], validMetrics2[validDoubleGauge])

	expHistogramBatch := getMetricsFromMetricList(
		getExpHistogramMetric("exponential_hist", lbs1, time1, &floatVal1, uint64(2), 2, []uint64{1, 1}),
		getExpHistogramMetric("exponential_hist", lbs2, time2, &floatVal2, uint64(2), 0, []uint64{2, 2}),
	)
	emptyExponentialHistogramBatch := getMetricsFromMetricList(
		getExpHistogramMetric("empty_exponential_hist", lbs1, time1, &floatValZero, uint64(0), 0, []uint64{}),
		getExpHistogramMetric("empty_exponential_hist", lbs1, time1, &floatValZero, uint64(0), 1, []uint64{}),
		getExpHistogramMetric("empty_exponential_hist", lbs2, time2, &floatValZero, uint64(0), 0, []uint64{}),
		getExpHistogramMetric("empty_exponential_hist_two", lbs2, time2, &floatValZero, uint64(0), 0, []uint64{}),
	)
	exponentialNoSumHistogramBatch := getMetricsFromMetricList(
		getExpHistogramMetric("no_sum_exponential_hist", lbs1, time1, nil, uint64(2), 0, []uint64{1, 1}),
		getExpHistogramMetric("no_sum_exponential_hist", lbs1, time2, nil, uint64(2), 0, []uint64{2, 2}),
	)

	histogramBatch := getMetricsFromMetricList(validMetrics1[validHistogram], validMetrics2[validHistogram])
	emptyDataPointHistogramBatch := getMetricsFromMetricList(validMetrics1[validEmptyHistogram], validMetrics2[validEmptyHistogram])
	histogramNoSumBatch := getMetricsFromMetricList(validMetrics1[validHistogramNoSum], validMetrics2[validHistogramNoSum])

	summaryBatch := getMetricsFromMetricList(validMetrics1[validSummary], validMetrics2[validSummary])

	// len(BucketCount) > len(ExplicitBounds)
	unmatchedBoundBucketHistBatch := getMetricsFromMetricList(validMetrics2[unmatchedBoundBucketHist])

	// fail cases
	emptyDoubleGaugeBatch := getMetricsFromMetricList(invalidMetrics[emptyGauge])

	emptyCumulativeSumBatch := getMetricsFromMetricList(invalidMetrics[emptyCumulativeSum])

	emptyCumulativeHistogramBatch := getMetricsFromMetricList(invalidMetrics[emptyCumulativeHistogram])

	emptySummaryBatch := getMetricsFromMetricList(invalidMetrics[emptySummary])

	// staleNaN cases
	staleNaNHistogramBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNHistogram])
	staleNaNEmptyHistogramBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNEmptyHistogram])

	staleNaNSummaryBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNSummary])

	staleNaNIntGaugeBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNIntGauge])

	staleNaNDoubleGaugeBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNDoubleGauge])

	staleNaNIntSumBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNIntSum])

	staleNaNSumBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNSum])

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
		require.Nil(t, ok)
		assert.EqualValues(t, expected, len(wr.Timeseries))
		if isStaleMarker {
			assert.True(t, value.IsStaleNaN(wr.Timeseries[0].Samples[0].Value))
		}
	}

	tests := []struct {
		name               string
		metrics            pmetric.Metrics
		reqTestFunc        func(t *testing.T, r *http.Request, expected int, isStaleMarker bool)
		expectedTimeSeries int
		httpResponseCode   int
		returnErr          bool
		isStaleMarker      bool
		skipForWAL         bool
	}{
		{
			name:             "invalid_type_case",
			metrics:          invalidTypeBatch,
			httpResponseCode: http.StatusAccepted,
			returnErr:        true,
		},
		{
			name:               "intSum_case",
			metrics:            intSumBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 5,
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
		{
			name:               "exponential_histogram_case",
			metrics:            expHistogramBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 2,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "valid_empty_exponential_histogram_case",
			metrics:            emptyExponentialHistogramBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 3,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "exponential_histogram_no_sum_case",
			metrics:            exponentialNoSumHistogramBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 1,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "histogram_case",
			metrics:            histogramBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 12,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "valid_empty_histogram_case",
			metrics:            emptyDataPointHistogramBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 4,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "histogram_no_sum_case",
			metrics:            histogramNoSumBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 10,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "summary_case",
			metrics:            summaryBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 10,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "unmatchedBoundBucketHist_case",
			metrics:            unmatchedBoundBucketHistBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 5,
			httpResponseCode:   http.StatusAccepted,
		},
		{
			name:               "5xx_case",
			metrics:            unmatchedBoundBucketHistBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 5,
			httpResponseCode:   http.StatusServiceUnavailable,
			returnErr:          false,
			// When using the WAL, it returns success once the data is persisted to the WAL
			skipForWAL: true,
		},
		{
			name:             "emptyGauge_case",
			metrics:          emptyDoubleGaugeBatch,
			reqTestFunc:      checkFunc,
			httpResponseCode: http.StatusAccepted,
			returnErr:        true,
		},
		{
			name:             "emptyCumulativeSum_case",
			metrics:          emptyCumulativeSumBatch,
			reqTestFunc:      checkFunc,
			httpResponseCode: http.StatusAccepted,
			returnErr:        true,
		},
		{
			name:             "emptyCumulativeHistogram_case",
			metrics:          emptyCumulativeHistogramBatch,
			reqTestFunc:      checkFunc,
			httpResponseCode: http.StatusAccepted,
			returnErr:        true,
		},
		{
			name:             "emptySummary_case",
			metrics:          emptySummaryBatch,
			reqTestFunc:      checkFunc,
			httpResponseCode: http.StatusAccepted,
			returnErr:        true,
		},
		{
			name:               "staleNaNIntGauge_case",
			metrics:            staleNaNIntGaugeBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 1,
			httpResponseCode:   http.StatusAccepted,
			isStaleMarker:      true,
		},
		{
			name:               "staleNaNDoubleGauge_case",
			metrics:            staleNaNDoubleGaugeBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 1,
			httpResponseCode:   http.StatusAccepted,
			isStaleMarker:      true,
		},
		{
			name:               "staleNaNIntSum_case",
			metrics:            staleNaNIntSumBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 1,
			httpResponseCode:   http.StatusAccepted,
			isStaleMarker:      true,
		},
		{
			name:               "staleNaNSum_case",
			metrics:            staleNaNSumBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 1,
			httpResponseCode:   http.StatusAccepted,
			isStaleMarker:      true,
		},
		{
			name:               "staleNaNHistogram_case",
			metrics:            staleNaNHistogramBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 6,
			httpResponseCode:   http.StatusAccepted,
			isStaleMarker:      true,
		},
		{
			name:               "staleNaNEmptyHistogram_case",
			metrics:            staleNaNEmptyHistogramBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 3,
			httpResponseCode:   http.StatusAccepted,
			isStaleMarker:      true,
		},
		{
			name:               "staleNaNSummary_case",
			metrics:            staleNaNSummaryBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 5,
			httpResponseCode:   http.StatusAccepted,
			isStaleMarker:      true,
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

					cfg := &Config{
						Namespace: "",
						HTTPClientSettings: confighttp.HTTPClientSettings{
							Endpoint: server.URL,
							// We almost read 0 bytes, so no need to tune ReadBufferSize.
							ReadBufferSize:  0,
							WriteBufferSize: 512 * 1024,
						},
						RemoteWriteQueue: RemoteWriteQueue{NumConsumers: 1},
						TargetInfo: &TargetInfo{
							Enabled: true,
						},
						CreatedMetric: &CreatedMetric{
							Enabled: true,
						},
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
					set := exportertest.NewNopCreateSettings()
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
					assert.NoError(t, err)
				})
			}
		})
	}
}

func Test_validateAndSanitizeExternalLabels(t *testing.T) {
	tests := []struct {
		name                string
		inputLabels         map[string]string
		expectedLabels      map[string]string
		returnErrorOnCreate bool
	}{
		{"success_case_no_labels",
			map[string]string{},
			map[string]string{},
			false,
		},
		{"success_case_with_labels",
			map[string]string{"key1": "val1"},
			map[string]string{"key1": "val1"},
			false,
		},
		{"success_case_2_with_labels",
			map[string]string{"__key1__": "val1"},
			map[string]string{"__key1__": "val1"},
			false,
		},
		{"success_case_with_sanitized_labels",
			map[string]string{"__key1.key__": "val1"},
			map[string]string{"__key1_key__": "val1"},
			false,
		},
		{"labels_that_start_with_digit",
			map[string]string{"6key_": "val1"},
			map[string]string{"key_6key_": "val1"},
			false,
		},
		{"fail_case_empty_label",
			map[string]string{"": "val1"},
			map[string]string{},
			true,
		},
	}
	testsWithoutSanitizelabel := []struct {
		name                string
		inputLabels         map[string]string
		expectedLabels      map[string]string
		returnErrorOnCreate bool
	}{
		{"success_case_no_labels",
			map[string]string{},
			map[string]string{},
			false,
		},
		{"success_case_with_labels",
			map[string]string{"key1": "val1"},
			map[string]string{"key1": "val1"},
			false,
		},
		{"success_case_2_with_labels",
			map[string]string{"__key1__": "val1"},
			map[string]string{"__key1__": "val1"},
			false,
		},
		{"success_case_with_sanitized_labels",
			map[string]string{"__key1.key__": "val1"},
			map[string]string{"__key1_key__": "val1"},
			false,
		},
		{"labels_that_start_with_digit",
			map[string]string{"6key_": "val1"},
			map[string]string{"key_6key_": "val1"},
			false,
		},
		{"fail_case_empty_label",
			map[string]string{"": "val1"},
			map[string]string{},
			true,
		},
	}
	// run tests
	for _, tt := range tests {
		cfg := createDefaultConfig().(*Config)
		cfg.ExternalLabels = tt.inputLabels
		t.Run(tt.name, func(t *testing.T) {
			newLabels, err := validateAndSanitizeExternalLabels(cfg)
			if tt.returnErrorOnCreate {
				assert.Error(t, err)
				return
			}
			assert.EqualValues(t, tt.expectedLabels, newLabels)
			assert.NoError(t, err)
		})
	}

	for _, tt := range testsWithoutSanitizelabel {
		cfg := createDefaultConfig().(*Config)
		// disable sanitizeLabel flag
		cfg.ExternalLabels = tt.inputLabels
		t.Run(tt.name, func(t *testing.T) {
			newLabels, err := validateAndSanitizeExternalLabels(cfg)
			if tt.returnErrorOnCreate {
				assert.Error(t, err)
				return
			}
			assert.EqualValues(t, tt.expectedLabels, newLabels)
			assert.NoError(t, err)
		})
	}
}

// Ensures that when we attach the Write-Ahead-Log(WAL) to the exporter,
// that it successfully writes the serialized prompb.WriteRequests to the WAL,
// and that we can retrieve those exact requests back from the WAL, when the
// exporter starts up once again, that it picks up where it left off.
func TestWALOnExporterRoundTrip(t *testing.T) {
	t.Skip("skipping test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10142")
	if testing.Short() {
		t.Skip("This test could run for long")
	}

	// 1. Create a mock Prometheus Remote Write Exporter that'll just
	// receive the bytes uploaded to it by our exporter.
	uploadedBytesCh := make(chan []byte, 1)
	exiting := make(chan bool)
	prweServer := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		uploaded, err2 := io.ReadAll(req.Body)
		assert.NoError(t, err2, "Error while reading from HTTP upload")
		select {
		case uploadedBytesCh <- uploaded:
		case <-exiting:
			return
		}
	}))
	defer prweServer.Close()

	// 2. Create the WAL configuration, create the
	// exporter and export some time series!
	tempDir := t.TempDir()
	cfg := &Config{
		Namespace: "test_ns",
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: prweServer.URL,
		},
		RemoteWriteQueue: RemoteWriteQueue{NumConsumers: 1},
		WAL: &WALConfig{
			Directory:  tempDir,
			BufferSize: 1,
		},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
		CreatedMetric: &CreatedMetric{
			Enabled: false,
		},
	}

	set := exportertest.NewNopCreateSettings()
	set.BuildInfo = component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}

	prwe, perr := newPRWExporter(cfg, set)
	assert.NoError(t, perr)

	nopHost := componenttest.NewNopHost()
	ctx := context.Background()
	require.NoError(t, prwe.Start(ctx, nopHost))
	t.Cleanup(func() {
		// This should have been shut down during the test
		// If it does not error then something went wrong.
		assert.Error(t, prwe.Shutdown(ctx))
		close(exiting)
	})
	require.NotNil(t, prwe.wal)

	ts1 := &prompb.TimeSeries{
		Labels:  []prompb.Label{{Name: "ts1l1", Value: "ts1k1"}},
		Samples: []prompb.Sample{{Value: 1, Timestamp: 100}},
	}
	ts2 := &prompb.TimeSeries{
		Labels:  []prompb.Label{{Name: "ts2l1", Value: "ts2k1"}},
		Samples: []prompb.Sample{{Value: 2, Timestamp: 200}},
	}
	tsMap := map[string]*prompb.TimeSeries{
		"timeseries1": ts1,
		"timeseries2": ts2,
	}
	errs := prwe.handleExport(ctx, tsMap)
	assert.NoError(t, errs)
	// Shutdown after we've written to the WAL. This ensures that our
	// exported data in-flight will flushed flushed to the WAL before exiting.
	require.NoError(t, prwe.Shutdown(ctx))

	// 3. Let's now read back all of the WAL records and ensure
	// that all the prompb.WriteRequest values exist as we sent them.
	wal, _, werr := cfg.WAL.createWAL()
	assert.NoError(t, werr)
	assert.NotNil(t, wal)
	t.Cleanup(func() {
		assert.NoError(t, wal.Close())
	})

	// Read all the indices.
	firstIndex, ierr := wal.FirstIndex()
	assert.NoError(t, ierr)
	lastIndex, ierr := wal.LastIndex()
	assert.NoError(t, ierr)

	var reqs []*prompb.WriteRequest
	for i := firstIndex; i <= lastIndex; i++ {
		protoBlob, err := wal.Read(i)
		assert.NoError(t, err)
		assert.NotNil(t, protoBlob)
		req := new(prompb.WriteRequest)
		err = proto.Unmarshal(protoBlob, req)
		assert.NoError(t, err)
		reqs = append(reqs, req)
	}
	assert.Equal(t, 1, len(reqs))
	// We MUST have 2 time series as were passed into tsMap.
	gotFromWAL := reqs[0]
	assert.Equal(t, 2, len(gotFromWAL.Timeseries))
	want := &prompb.WriteRequest{
		Timeseries: orderBySampleTimestamp([]prompb.TimeSeries{
			*ts1, *ts2,
		}),
	}

	// Even after sorting timeseries, we need to sort them
	// also by Label to ensure deterministic ordering.
	orderByLabelValue(gotFromWAL)
	gotFromWAL.Timeseries = orderBySampleTimestamp(gotFromWAL.Timeseries)
	orderByLabelValue(want)

	assert.Equal(t, want, gotFromWAL)

	// 4. Finally, ensure that the bytes that were uploaded to the
	// Prometheus Remote Write endpoint are exactly as were saved in the WAL.
	// Read from that same WAL, export to the RWExporter server.
	prwe2, err := newPRWExporter(cfg, set)
	assert.NoError(t, err)
	require.NoError(t, prwe2.Start(ctx, nopHost))
	t.Cleanup(func() {
		assert.NoError(t, prwe2.Shutdown(ctx))
	})
	require.NotNil(t, prwe2.wal)

	snappyEncodedBytes := <-uploadedBytesCh
	decodeBuffer := make([]byte, len(snappyEncodedBytes))
	uploadedBytes, derr := snappy.Decode(decodeBuffer, snappyEncodedBytes)
	require.NoError(t, derr)
	gotFromUpload := new(prompb.WriteRequest)
	uerr := proto.Unmarshal(uploadedBytes, gotFromUpload)
	assert.NoError(t, uerr)
	gotFromUpload.Timeseries = orderBySampleTimestamp(gotFromUpload.Timeseries)
	// Even after sorting timeseries, we need to sort them
	// also by Label to ensure deterministic ordering.
	orderByLabelValue(gotFromUpload)

	// 4.1. Ensure that all the various combinations match up.
	// To ensure a deterministic ordering, sort the TimeSeries by Label Name.
	assert.Equal(t, want, gotFromUpload)
	assert.Equal(t, gotFromWAL, gotFromUpload)
}

func TestRetryOn5xx(t *testing.T) {
	// Create a mock HTTP server with a counter to simulate a 5xx error on the first attempt and a 2xx success on the second attempt
	attempts := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts == 0 {
			attempts++
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer mockServer.Close()

	endpointURL, err := url.Parse(mockServer.URL)
	assert.NoError(t, err)

	// Create the prwExporter
	exporter := &prwExporter{
		endpointURL: endpointURL,
		client:      http.DefaultClient,
	}

	ctx := context.Background()

	// Execute the write request and verify that the exporter returns a non-permanent error on the first attempt.
	err = exporter.execute(ctx, &prompb.WriteRequest{})
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "remote write returned HTTP status 500"))
	assert.False(t, consumererror.IsPermanent(err))

	// Execute the write request again and verify that the exporter does not return an error on the second attempt
	err = exporter.execute(ctx, &prompb.WriteRequest{})
	assert.NoError(t, err)
}

func TestPartitionTimeSeries(t *testing.T) {
	// Test input data
	tsMap := map[string]*prompb.TimeSeries{
		"cpu_usage_total": {
			Labels: []prompb.Label{
				{Name: "instance", Value: "node1"},
				{Name: "job", Value: "webserver"},
			},
		},
		"memory_usage_total": {
			Labels: []prompb.Label{
				{Name: "instance", Value: "node2"},
				{Name: "job", Value: "webserver"},
			},
		},
	}

	concurrencyLimit := 2

	partitions := partitionTimeSeries(tsMap, concurrencyLimit)

	// Check the number of partitions
	if len(partitions) != concurrencyLimit {
		t.Errorf("Expected %d partitions, got %d", concurrencyLimit, len(partitions))
	}

	// Check the number of time series in each partition
	totalTimeSeries := 0
	for _, partition := range partitions {
		totalTimeSeries += len(partition)
	}
	if totalTimeSeries != len(tsMap) {
		t.Errorf("Expected %d time series in partitions, got %d", len(tsMap), totalTimeSeries)
	}
}

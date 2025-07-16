// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadatatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

// Test_NewPRWExporter checks that a new exporter instance with non-nil fields is initialized
func Test_NewPRWExporter(t *testing.T) {
	cfg := &Config{
		TimeoutSettings: exporterhelper.TimeoutConfig{},
		BackOffConfig:   configretry.BackOffConfig{},
		Namespace:       "",
		ExternalLabels:  map[string]string{},
		ClientConfig:    confighttp.NewDefaultClientConfig(),
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
	}
	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopSettings(metadata.Type)
	set.BuildInfo = buildInfo

	tests := []struct {
		name                string
		config              *Config
		namespace           string
		endpoint            string
		concurrency         int
		externalLabels      map[string]string
		returnErrorOnCreate bool
		set                 exporter.Settings
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
			cfg.ClientConfig.Endpoint = tt.endpoint
			cfg.ExternalLabels = tt.externalLabels
			cfg.Namespace = tt.namespace
			cfg.RemoteWriteQueue.NumConsumers = 1
			cfg.RemoteWriteProtoMsg = config.RemoteWriteProtoMsgV1
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
		TimeoutSettings:   exporterhelper.TimeoutConfig{},
		BackOffConfig:     configretry.BackOffConfig{},
		MaxBatchSizeBytes: 3000000,
		Namespace:         "",
		ExternalLabels:    map[string]string{},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
	}
	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopSettings(metadata.Type)
	set.BuildInfo = buildInfo

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "https://some.url:9411/api/prom/push"
	clientConfigTLS := confighttp.NewDefaultClientConfig()
	clientConfigTLS.Endpoint = "https://some.url:9411/api/prom/push"
	clientConfigTLS.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "nonexistent file",
			CertFile: "",
			KeyFile:  "",
		},
		Insecure:   false,
		ServerName: "",
	}

	tests := []struct {
		name                 string
		config               *Config
		namespace            string
		concurrency          int
		externalLabels       map[string]string
		returnErrorOnStartUp bool
		set                  exporter.Settings
		endpoint             string
		clientSettings       confighttp.ClientConfig
	}{
		{
			name:           "success_case",
			config:         cfg,
			namespace:      "test",
			concurrency:    5,
			externalLabels: map[string]string{"Key1": "Val1"},
			set:            set,
			clientSettings: clientConfig,
		},
		{
			name:                 "invalid_tls",
			config:               cfg,
			namespace:            "test",
			concurrency:          5,
			externalLabels:       map[string]string{"Key1": "Val1"},
			set:                  set,
			returnErrorOnStartUp: true,
			clientSettings:       clientConfigTLS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.ExternalLabels = tt.externalLabels
			cfg.Namespace = tt.namespace
			cfg.RemoteWriteQueue.NumConsumers = 1
			cfg.ClientConfig = tt.clientSettings
			cfg.RemoteWriteProtoMsg = config.RemoteWriteProtoMsgV1

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
		require.NoError(t, err)
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

		assert.Len(t, writeReq.Timeseries, 1)
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
		{
			"success_case",
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
			true,
		},
		{
			"error_status_code_case",
			*ts1,
			true,
			http.StatusForbidden,
			true,
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	cfg.ClientConfig.Endpoint = endpoint.String()
	cfg.RemoteWriteQueue.NumConsumers = 1
	cfg.BackOffConfig = configretry.BackOffConfig{
		Enabled:         true,
		InitialInterval: 100 * time.Millisecond, // Shorter initial interval
		MaxInterval:     1 * time.Second,        // Shorter max interval
		MaxElapsedTime:  2 * time.Second,        // Shorter max elapsed time
	}

	buildInfo := component.BuildInfo{
		Description: "OpenTelemetry Collector",
		Version:     "1.0",
	}
	set := exportertest.NewNopSettings(metadata.Type)
	set.BuildInfo = buildInfo
	// after this, instantiate a CortexExporter with the current HTTP client and endpoint set to passed in endpoint
	prwe, err := newPRWExporter(cfg, set)
	if err != nil {
		return err
	}

	if err = prwe.Start(context.Background(), componenttest.NewNopHost()); err != nil {
		return err
	}

	return prwe.handleExport(context.Background(), testmap, nil)
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

	// partial success (or partial failure) cases

	partialSuccess1 := getMetricsFromMetricList(validMetrics1[validSum], validMetrics2[validSum],
		validMetrics1[validIntGauge], validMetrics2[validIntGauge], invalidMetrics[emptyGauge])

	// staleNaN cases
	staleNaNHistogramBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNHistogram])
	staleNaNEmptyHistogramBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNEmptyHistogram])

	staleNaNSummaryBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNSummary])

	staleNaNIntGaugeBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNIntGauge])

	staleNaNDoubleGaugeBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNDoubleGauge])

	staleNaNIntSumBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNIntSum])

	staleNaNSumBatch := getMetricsFromMetricList(staleNaNMetrics[staleNaNSum])

	checkFunc := func(t *testing.T, r *http.Request, expected int, isStaleMarker bool, enableSendingRW2 bool) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		buf := make([]byte, len(body))
		dest, err := snappy.Decode(buf, body)
		if enableSendingRW2 {
			assert.Equal(t, "2.0.0", r.Header.Get("x-prometheus-remote-write-version"))
			assert.Equal(t, "application/x-protobuf;proto=io.prometheus.write.v2.Request", r.Header.Get("Content-Type"))
		} else {
			assert.Equal(t, "0.1.0", r.Header.Get("x-prometheus-remote-write-version"))
		}
		assert.Equal(t, "snappy", r.Header.Get("content-encoding"))
		assert.Equal(t, "opentelemetry-collector/1.0", r.Header.Get("User-Agent"))
		assert.NotNil(t, r.Header.Get("tenant-id"))
		require.NoError(t, err)
		if enableSendingRW2 {
			wr := &writev2.Request{}
			ok := proto.Unmarshal(dest, wr)
			require.NoError(t, ok)
			assert.Len(t, wr.Timeseries, expected)
			// TODO check labels
		} else {
			wr := &prompb.WriteRequest{}
			ok := proto.Unmarshal(dest, wr)
			require.NoError(t, ok)
			assert.Len(t, wr.Timeseries, expected)
			if isStaleMarker {
				assert.True(t, value.IsStaleNaN(wr.Timeseries[0].Samples[0].Value))
			}
		}
	}

	tests := []struct {
		name                       string
		metrics                    pmetric.Metrics
		reqTestFunc                func(t *testing.T, r *http.Request, expected int, isStaleMarker bool, enableSendingRW2 bool)
		expectedTimeSeries         int
		httpResponseCode           int
		returnErr                  bool
		isStaleMarker              bool
		skipForWAL                 bool
		expectedFailedTranslations int
		enableSendingRW2           bool
	}{
		{
			name:                       "invalid_type_case",
			metrics:                    invalidTypeBatch,
			httpResponseCode:           http.StatusAccepted,
			reqTestFunc:                checkFunc,
			expectedTimeSeries:         0,
			expectedFailedTranslations: 1,
		},
		{
			name:               "intSum_case",
			metrics:            intSumBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 2,
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
			name:               "doubleGauge_case_rw2",
			metrics:            doubleGaugeBatch,
			reqTestFunc:        checkFunc,
			expectedTimeSeries: 2,
			httpResponseCode:   http.StatusAccepted,
			enableSendingRW2:   true,
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
			returnErr:          true,
			// When using the WAL, it returns success once the data is persisted to the WAL
			skipForWAL: true,
		},
		{
			name:                       "emptyGauge_case",
			metrics:                    emptyDoubleGaugeBatch,
			reqTestFunc:                checkFunc,
			httpResponseCode:           http.StatusAccepted,
			expectedFailedTranslations: 1,
		},
		{
			name:                       "emptyCumulativeSum_case",
			metrics:                    emptyCumulativeSumBatch,
			reqTestFunc:                checkFunc,
			httpResponseCode:           http.StatusAccepted,
			expectedFailedTranslations: 1,
		},
		{
			name:                       "emptyCumulativeHistogram_case",
			metrics:                    emptyCumulativeHistogramBatch,
			reqTestFunc:                checkFunc,
			httpResponseCode:           http.StatusAccepted,
			expectedFailedTranslations: 1,
		},
		{
			name:                       "emptySummary_case",
			metrics:                    emptySummaryBatch,
			reqTestFunc:                checkFunc,
			httpResponseCode:           http.StatusAccepted,
			expectedFailedTranslations: 1,
		},
		{
			name:                       "partialSuccess_case",
			metrics:                    partialSuccess1,
			reqTestFunc:                checkFunc,
			httpResponseCode:           http.StatusAccepted,
			expectedTimeSeries:         4,
			expectedFailedTranslations: 1,
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
			for _, tt := range tests {
				// skip WAL for rw2 cases as rw2 doesn't currently support WAL
				if useWAL && (tt.skipForWAL || tt.enableSendingRW2) {
					t.Skip("test not supported when using WAL")
				}
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if tt.reqTestFunc != nil {
							tt.reqTestFunc(t, r, tt.expectedTimeSeries, tt.isStaleMarker, tt.enableSendingRW2)
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
						TargetInfo: &TargetInfo{
							Enabled: true,
						},
						BackOffConfig: retrySettings,
					}

					if tt.enableSendingRW2 {
						cfg.RemoteWriteProtoMsg = config.RemoteWriteProtoMsgV2
						oldValue := enableSendingRW2FeatureGate.IsEnabled()
						testutil.SetFeatureGateForTest(t, enableSendingRW2FeatureGate, tt.enableSendingRW2)
						defer testutil.SetFeatureGateForTest(t, enableSendingRW2FeatureGate, oldValue)
					} else {
						cfg.RemoteWriteProtoMsg = config.RemoteWriteProtoMsgV1
					}

					if useWAL {
						cfg.WAL = &WALConfig{
							Directory: t.TempDir(),
						}
					}

					assert.NotNil(t, cfg)
					tel := componenttest.NewTelemetry(
						componenttest.WithMetricOptions(sdkmetric.WithView(
							// Drop otelhttp metrics
							sdkmetric.NewView(
								sdkmetric.Instrument{
									Scope: instrumentation.Scope{Name: otelhttp.ScopeName},
								},
								sdkmetric.Stream{
									Aggregation: sdkmetric.AggregationDrop{},
								},
							))),
					)
					t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) })
					set := metadatatest.NewSettings(tel)
					set.BuildInfo = component.BuildInfo{
						Description: "OpenTelemetry Collector",
						Version:     "1.0",
					}

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
					if tt.expectedFailedTranslations > 0 {
						metadatatest.AssertEqualExporterPrometheusremotewriteFailedTranslations(t, tel, []metricdata.DataPoint[int64]{
							{
								Value:      int64(tt.expectedFailedTranslations),
								Attributes: attribute.NewSet(attribute.String("exporter", "prometheusremotewrite"), attribute.String("endpoint", clientConfig.Endpoint)),
							},
						}, metricdatatest.IgnoreTimestamp())
					}

					metadatatest.AssertEqualExporterPrometheusremotewriteTranslatedTimeSeries(t, tel, []metricdata.DataPoint[int64]{
						{
							Value:      int64(tt.expectedTimeSeries),
							Attributes: attribute.NewSet(attribute.String("exporter", "prometheusremotewrite"), attribute.String("endpoint", clientConfig.Endpoint)),
						},
					}, metricdatatest.IgnoreTimestamp())
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
		{
			"success_case_no_labels",
			map[string]string{},
			map[string]string{},
			false,
		},
		{
			"success_case_with_labels",
			map[string]string{"key1": "val1"},
			map[string]string{"key1": "val1"},
			false,
		},
		{
			"success_case_2_with_labels",
			map[string]string{"__key1__": "val1"},
			map[string]string{"__key1__": "val1"},
			false,
		},
		{
			"success_case_with_sanitized_labels",
			map[string]string{"__key1.key__": "val1"},
			map[string]string{"__key1_key__": "val1"},
			false,
		},
		{
			"labels_that_start_with_digit",
			map[string]string{"6key_": "val1"},
			map[string]string{"key_6key_": "val1"},
			false,
		},
		{
			"fail_case_empty_label",
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
		{
			"success_case_no_labels",
			map[string]string{},
			map[string]string{},
			false,
		},
		{
			"success_case_with_labels",
			map[string]string{"key1": "val1"},
			map[string]string{"key1": "val1"},
			false,
		},
		{
			"success_case_2_with_labels",
			map[string]string{"__key1__": "val1"},
			map[string]string{"__key1__": "val1"},
			false,
		},
		{
			"success_case_with_sanitized_labels",
			map[string]string{"__key1.key__": "val1"},
			map[string]string{"__key1_key__": "val1"},
			false,
		},
		{
			"labels_that_start_with_digit",
			map[string]string{"6key_": "val1"},
			map[string]string{"key_6key_": "val1"},
			false,
		},
		{
			"fail_case_empty_label",
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
			assert.Equal(t, tt.expectedLabels, newLabels)
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
			assert.Equal(t, tt.expectedLabels, newLabels)
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
	prweServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
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
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = prweServer.URL
	cfg := &Config{
		Namespace:        "test_ns",
		ClientConfig:     clientConfig,
		RemoteWriteQueue: RemoteWriteQueue{NumConsumers: 1},
		WAL: &WALConfig{
			Directory:  tempDir,
			BufferSize: 1,
		},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
	}

	set := exportertest.NewNopSettings(metadata.Type)
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
	errs := prwe.handleExport(ctx, tsMap, nil)
	assert.NoError(t, errs)
	// Shutdown after we've written to the WAL. This ensures that our
	// exported data in-flight will be flushed to the WAL before exiting.
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
	assert.Len(t, reqs, 1)
	// We MUST have 2 time series as were passed into tsMap.
	gotFromWAL := reqs[0]
	assert.Len(t, gotFromWAL.Timeseries, 2)
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

func canceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func assertPermanentConsumerError(t assert.TestingT, err error, _ ...any) bool {
	return assert.True(t, consumererror.IsPermanent(err), "error should be consumererror.Permanent")
}

func TestRetries(t *testing.T) {
	tts := []struct {
		name             string
		serverErrorCount int // number of times server should return error
		expectedAttempts int
		httpStatus       int
		RetryOnHTTP429   bool
		assertError      assert.ErrorAssertionFunc
		assertErrorType  assert.ErrorAssertionFunc
		ctx              context.Context
	}{
		{
			"test 5xx should retry",
			3,
			4,
			http.StatusInternalServerError,
			false,
			assert.NoError,
			assert.NoError,
			context.Background(),
		},
		{
			"test 429 should retry",
			3,
			4,
			http.StatusTooManyRequests,
			true,
			assert.NoError,
			assert.NoError,
			context.Background(),
		},
		{
			"test 429 should not retry",
			4,
			1,
			http.StatusTooManyRequests,
			false,
			assert.Error,
			assertPermanentConsumerError,
			context.Background(),
		},
		{
			"test 4xx should not retry",
			4,
			1,
			http.StatusBadRequest,
			false,
			assert.Error,
			assertPermanentConsumerError,
			context.Background(),
		},
		{
			"test timeout context should not execute",
			4,
			0,
			http.StatusInternalServerError,
			false,
			assert.Error,
			assertPermanentConsumerError,
			canceledContext(),
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			totalAttempts := 0
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if totalAttempts < tt.serverErrorCount {
					http.Error(w, http.StatusText(tt.httpStatus), tt.httpStatus)
				} else {
					w.WriteHeader(http.StatusOK)
				}
				totalAttempts++
			},
			))
			defer mockServer.Close()

			endpointURL, err := url.Parse(mockServer.URL)
			require.NoError(t, err)

			// Create the telemetry
			testTel := componenttest.NewTelemetry()
			telemetry, err := newPRWTelemetry(exporter.Settings{TelemetrySettings: testTel.NewTelemetrySettings()}, endpointURL)
			require.NoError(t, err)

			// Create the prwExporter
			exporter := &prwExporter{
				endpointURL:    endpointURL,
				client:         http.DefaultClient,
				retryOnHTTP429: tt.RetryOnHTTP429,
				retrySettings: configretry.BackOffConfig{
					Enabled: true,
				},
				telemetry: telemetry,
			}
			buf := bufferPool.Get().(*buffer)
			buf.protobuf.Reset()
			defer bufferPool.Put(buf)

			errMarshal := buf.protobuf.Marshal(&prompb.WriteRequest{})
			if errMarshal != nil {
				require.NoError(t, errMarshal)
				return
			}

			err = exporter.execute(tt.ctx, buf)
			tt.assertError(t, err)
			tt.assertErrorType(t, err)
			assert.Equal(t, tt.expectedAttempts, totalAttempts)
		})
	}
}

func BenchmarkExecute(b *testing.B) {
	for _, numSample := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("numSample=%d", numSample), func(b *testing.B) {
			benchmarkExecute(b, numSample)
		})
	}
}

func benchmarkExecute(b *testing.B, numSample int) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()
	endpointURL, err := url.Parse(mockServer.URL)
	require.NoError(b, err)

	// Create the prwExporter
	exporter := &prwExporter{
		endpointURL: endpointURL,
		client:      http.DefaultClient,
	}

	generateSamples := func(n int) []prompb.Sample {
		samples := make([]prompb.Sample, 0, n)
		for i := 0; i < n; i++ {
			samples = append(samples, prompb.Sample{
				Timestamp: int64(i),
				Value:     float64(i),
			})
		}
		return samples
	}

	generateHistograms := func(n int) []prompb.Histogram {
		histograms := make([]prompb.Histogram, 0, n)
		for i := 0; i < n; i++ {
			histograms = append(histograms, prompb.Histogram{
				Timestamp:      int64(i),
				Count:          &prompb.Histogram_CountInt{CountInt: uint64(i)},
				PositiveCounts: []float64{float64(i)},
			})
		}
		return histograms
	}

	reqs := make([]*prompb.WriteRequest, 0, b.N)
	const labelValue = "abcdefg'hijlmn234!@#$%^&*()_+~`\"{}[],./<>?hello0123hiOlá你好Dzieńdobry9Zd8ra765v4stvuyte"
	for n := 0; n < b.N; n++ {
		num := strings.Repeat(strconv.Itoa(n), 16)
		req := &prompb.WriteRequest{
			Metadata: []prompb.MetricMetadata{
				{
					Type: prompb.MetricMetadata_COUNTER,
					Unit: "seconds",
					Help: "This is a counter",
				},
				{
					Type: prompb.MetricMetadata_HISTOGRAM,
					Unit: "seconds",
					Help: "This is a histogram",
				},
			},
			Timeseries: []prompb.TimeSeries{
				{
					Samples: generateSamples(numSample),
					Labels: []prompb.Label{
						{Name: "__name__", Value: "test_metric"},
						{Name: "test_label_name_" + num, Value: labelValue + num},
					},
				},
				{
					Histograms: generateHistograms(numSample),
					Labels: []prompb.Label{
						{Name: "__name__", Value: "test_histogram"},
						{Name: "test_label_name_" + num, Value: labelValue + num},
					},
				},
			},
		}
		reqs = append(reqs, req)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for _, req := range reqs {
		buf := bufferPool.Get().(*buffer)
		buf.protobuf.Reset()
		defer bufferPool.Put(buf)

		errMarshal := buf.protobuf.Marshal(req)
		if errMarshal != nil {
			require.NoError(b, errMarshal)
			return
		}
		err := exporter.execute(ctx, buf)
		require.NoError(b, err)
	}
}

func BenchmarkPushMetrics(b *testing.B) {
	for _, numMetrics := range []int{10, 100, 1000, 10000} {
		b.Run(fmt.Sprintf("numMetrics=%d", numMetrics), func(b *testing.B) {
			benchmarkPushMetrics(b, numMetrics, 1)
		})
	}
}

func BenchmarkPushMetricsVaryingMetrics(b *testing.B) {
	benchmarkPushMetrics(b, -1, 1)
}

// benchmarkPushMetrics benchmarks the PushMetrics method with a given number of metrics.
// If numMetrics is -1, it will benchmark with varying number of metrics, from 10 up to 10000.
func benchmarkPushMetrics(b *testing.B, numMetrics, numConsumers int) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()
	endpointURL, err := url.Parse(mockServer.URL)
	require.NoError(b, err)

	set := exportertest.NewNopSettings(metadata.Type)
	// Adjusted retry settings for faster testing
	retrySettings := configretry.BackOffConfig{
		Enabled:         true,
		InitialInterval: 100 * time.Millisecond, // Shorter initial interval
		MaxInterval:     1 * time.Second,        // Shorter max interval
		MaxElapsedTime:  2 * time.Second,        // Shorter max elapsed time
	}
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = endpointURL.String()
	clientConfig.ReadBufferSize = 0
	clientConfig.WriteBufferSize = 512 * 1024
	cfg := &Config{
		Namespace:         "",
		ClientConfig:      clientConfig,
		MaxBatchSizeBytes: 3000,
		RemoteWriteQueue:  RemoteWriteQueue{NumConsumers: numConsumers},
		BackOffConfig:     retrySettings,
		TargetInfo:        &TargetInfo{Enabled: true},
	}
	exporter, err := newPRWExporter(cfg, set)
	require.NoError(b, err)

	var metrics []pmetric.Metrics
	for n := 0; n < b.N; n++ {
		actualNumMetrics := numMetrics
		if numMetrics == -1 {
			actualNumMetrics = int(math.Pow(10, float64(n%4+1)))
		}
		m := testdata.GenerateMetricsManyMetricsSameResource(actualNumMetrics)
		for i := 0; i < m.MetricCount(); i++ {
			dp := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i).Sum().DataPoints().AppendEmpty()
			dp.SetIntValue(int64(i))
			// We add a random key to the attributes to ensure that we create a new time series during translation for each metric.
			dp.Attributes().PutInt("random_key", int64(i))
		}
		metrics = append(metrics, m)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(b, exporter.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		require.NoError(b, exporter.Shutdown(ctx))
	}()
	b.ReportAllocs()
	b.ResetTimer()
	for _, m := range metrics {
		err := exporter.PushMetrics(ctx, m)
		require.NoError(b, err)
	}
}

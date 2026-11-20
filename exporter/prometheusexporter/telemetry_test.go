// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadatatest"
)

var expectedScrapeDurationBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10, 30}

func newTestTelemetryExporter(t *testing.T, cfg *Config) (*prometheusExporter, *componenttest.Telemetry) {
	t.Helper()
	testTel := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, testTel.Shutdown(context.WithoutCancel(t.Context())))
	})
	set := metadatatest.NewSettings(testTel)
	if cfg.ServerConfig.NetAddr.Endpoint == "" {
		cfg.ServerConfig.NetAddr.Endpoint = "127.0.0.1:0"
	}
	if cfg.MetricExpiration == 0 {
		cfg.MetricExpiration = time.Hour
	}
	exp, err := newPrometheusExporter(cfg, set)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, exp.Shutdown(context.WithoutCancel(t.Context())))
	})
	return exp, testTel
}

// assertScrapeDuration validates metric metadata via metadatatest and explicitly
// checks histogram Count, Sum, Bounds, and BucketCounts (which IgnoreValue skips).
func assertScrapeDuration(t *testing.T, testTel *componenttest.Telemetry, wantCounts map[string]uint64) {
	t.Helper()
	wantShape := make([]metricdata.HistogramDataPoint[float64], 0, len(wantCounts))
	for outcome := range wantCounts {
		wantShape = append(wantShape, metricdata.HistogramDataPoint[float64]{
			Attributes: attribute.NewSet(attribute.String("outcome", outcome)),
		})
	}
	metadatatest.AssertEqualExporterPrometheusScrapeDuration(t, testTel, wantShape, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())

	got, err := testTel.GetMetric("otelcol.exporter.prometheus.scrape.duration")
	require.NoError(t, err)
	hist, ok := got.Data.(metricdata.Histogram[float64])
	require.True(t, ok, "expected Histogram[float64], got %T", got.Data)
	require.Len(t, hist.DataPoints, len(wantCounts))

	seen := make(map[string]bool, len(wantCounts))
	for _, dp := range hist.DataPoints {
		val, exists := dp.Attributes.Value("outcome")
		require.True(t, exists, "missing outcome attribute")
		outcome := val.AsString()
		wantCount, ok := wantCounts[outcome]
		require.True(t, ok, "unexpected outcome %q", outcome)
		seen[outcome] = true

		assert.Equal(t, wantCount, dp.Count, "scrape duration count mismatch for outcome=%s", outcome)
		assert.GreaterOrEqual(t, dp.Sum, float64(0), "scrape duration sum must be non-negative")
		assert.Equal(t, expectedScrapeDurationBounds, dp.Bounds)
		var bucketTotal uint64
		for _, c := range dp.BucketCounts {
			bucketTotal += c
		}
		assert.Equal(t, wantCount, bucketTotal, "bucket counts must sum to histogram count")
	}
	assert.Len(t, seen, len(wantCounts))
}

func TestTelemetryActiveSeries(t *testing.T) {
	exp, testTel := newTestTelemetryExporter(t, &Config{})

	// Initially 0 active series.
	metadatatest.AssertEqualExporterPrometheusActiveSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 0},
	}, metricdatatest.IgnoreTimestamp())

	now := time.Now()
	md := pmetric.NewMetrics()
	ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

	// Series 1: gauge
	g := ms.AppendEmpty()
	g.SetName("test_gauge")
	gdp := g.SetEmptyGauge().DataPoints().AppendEmpty()
	gdp.SetIntValue(10)
	gdp.Attributes().PutStr("k", "v1")
	gdp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// Series 2: sum
	s := ms.AppendEmpty()
	s.SetName("test_sum")
	sum := s.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sdp := sum.DataPoints().AppendEmpty()
	sdp.SetIntValue(20)
	sdp.Attributes().PutStr("k", "v2")
	sdp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// Series 3: histogram
	h := ms.AppendEmpty()
	h.SetName("test_hist")
	hist := h.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	hdp := hist.DataPoints().AppendEmpty()
	hdp.SetCount(1)
	hdp.SetSum(5)
	hdp.BucketCounts().FromRaw([]uint64{1, 0})
	hdp.ExplicitBounds().FromRaw([]float64{10})
	hdp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	require.NoError(t, exp.ConsumeMetrics(t.Context(), md))
	metadatatest.AssertEqualExporterPrometheusActiveSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 3},
	}, metricdatatest.IgnoreTimestamp())

	// Updating an existing series with a newer timestamp must not double-count.
	updateMD := pmetric.NewMetrics()
	ug := updateMD.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	ug.SetName("test_gauge")
	ugdp := ug.SetEmptyGauge().DataPoints().AppendEmpty()
	ugdp.SetIntValue(15)
	ugdp.Attributes().PutStr("k", "v1")
	ugdp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Second)))
	require.NoError(t, exp.ConsumeMetrics(t.Context(), updateMD))

	metadatatest.AssertEqualExporterPrometheusActiveSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 3},
	}, metricdatatest.IgnoreTimestamp())

	// Staleness marker deletes 1 series; repeating it must not decrement below the true count.
	staleMD := pmetric.NewMetrics()
	sg := staleMD.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	sg.SetName("test_gauge")
	sgdp := sg.SetEmptyGauge().DataPoints().AppendEmpty()
	sgdp.Attributes().PutStr("k", "v1")
	sgdp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	sgdp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(2 * time.Second)))

	require.NoError(t, exp.ConsumeMetrics(t.Context(), staleMD))
	require.NoError(t, exp.ConsumeMetrics(t.Context(), staleMD))
	metadatatest.AssertEqualExporterPrometheusActiveSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 2},
	}, metricdatatest.IgnoreTimestamp())

	// Expiration via scrape evicts the remaining series.
	acc := exp.collector.accumulator.(*lastValueAccumulator)
	acc.metricExpiration = time.Millisecond
	time.Sleep(5 * time.Millisecond)

	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)

	metadatatest.AssertEqualExporterPrometheusActiveSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 0},
	}, metricdatatest.IgnoreTimestamp())

	// Expiration via background cleanupExpired also decrements active series to 0.
	acc.metricExpiration = time.Hour
	require.NoError(t, exp.ConsumeMetrics(t.Context(), md))
	metadatatest.AssertEqualExporterPrometheusActiveSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 3},
	}, metricdatatest.IgnoreTimestamp())

	acc.metricExpiration = time.Millisecond
	time.Sleep(5 * time.Millisecond)
	acc.cleanupExpired()
	metadatatest.AssertEqualExporterPrometheusActiveSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 0},
	}, metricdatatest.IgnoreTimestamp())
}

func TestAccumulatorActiveSeriesConcurrent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		expiration time.Duration
	}{
		{name: "without_expiration", expiration: time.Hour},
		{name: "with_expiration", expiration: time.Nanosecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acc := newAccumulator(zap.NewNop(), tc.expiration).(*lastValueAccumulator)
			now := time.Now()

			var wg sync.WaitGroup
			for worker := range 8 {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					for i := range 50 {
						rm := pmetric.NewResourceMetrics()
						m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
						m.SetName(fmt.Sprintf("m_%d", i%10))
						dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
						dp.Attributes().PutStr("worker", fmt.Sprintf("%d", id%4))
						dp.SetIntValue(int64(i))
						dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Add(time.Duration(i) * time.Millisecond)))
						if i%7 == 0 {
							dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
						}
						acc.Accumulate(rm)
						if i%5 == 0 {
							acc.Collect()
							acc.cleanupExpired()
						}
						assert.GreaterOrEqual(t, acc.activeSeries(), int64(0))
					}
				}(worker)
			}
			wg.Wait()

			var actual int64
			acc.registeredMetrics.Range(func(_, _ any) bool {
				actual++
				return true
			})
			assert.Equal(t, actual, acc.activeSeries())
		})
	}
}

func TestTelemetryScrapeExportedSeriesAndDuration(t *testing.T) {
	exp, testTel := newTestTelemetryExporter(t, &Config{})

	// Scrape before any metrics arrive: 0 exported series, 1 successful scrape.
	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 0},
	}, metricdatatest.IgnoreTimestamp())
	assertScrapeDuration(t, testTel, map[string]uint64{"success": 1})

	// Ingest 1 resource (produces 1 target_info) + 2 metric series = 3 exported series.
	require.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(0, "", "svc", "inst-1")))

	res = httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 3},
	}, metricdatatest.IgnoreTimestamp())
	assertScrapeDuration(t, testTel, map[string]uint64{"success": 2})
}

func TestTelemetryScrapeExportedSeries_NameFilter(t *testing.T) {
	exp, testTel := newTestTelemetryExporter(t, &Config{})

	// Ingest 1 target_info + 2 counter series ("this_one_there_where_bytes_total").
	require.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(0, "", "svc", "inst-1")))

	// Filter to only target_info -> 1 series exported.
	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics?name[]=target_info", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "target_info")
	assert.NotContains(t, res.Body.String(), "this_one_there_where_bytes_total")

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 1},
	}, metricdatatest.IgnoreTimestamp())

	// Filter to a non-existent metric -> 0 series exported (cumulative stays 1).
	res = httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics?name[]=does_not_exist", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)
	assert.Empty(t, res.Body.String())

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 1},
	}, metricdatatest.IgnoreTimestamp())

	// Filter matching both families -> 3 series exported (cumulative becomes 4).
	res = httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics?name[]=target_info&name[]=this_one_there_where_bytes_total", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 4},
	}, metricdatatest.IgnoreTimestamp())
	assertScrapeDuration(t, testTel, map[string]uint64{"success": 3})
}

type failingGatherer struct {
	mfs []*dto.MetricFamily
	err error
}

func (f *failingGatherer) Gather() ([]*dto.MetricFamily, error) {
	return f.mfs, f.err
}

type failingResponseWriter struct {
	header http.Header
	code   int
}

func (w *failingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingResponseWriter) WriteHeader(code int) {
	w.code = code
}

func (*failingResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("broken pipe")
}

func TestTelemetryScrapeFailureDuration(t *testing.T) {
	t.Run("gather error with zero metrics returns 500 and records failure without exported series", func(t *testing.T) {
		testTel := componenttest.NewTelemetry()
		t.Cleanup(func() {
			require.NoError(t, testTel.Shutdown(context.WithoutCancel(t.Context())))
		})
		tb, err := metadata.NewTelemetryBuilder(testTel.NewTelemetrySettings())
		require.NoError(t, err)
		t.Cleanup(tb.Shutdown)

		h := newScrapeHandler(
			&failingGatherer{err: errors.New("gather failed")},
			promhttp.HandlerOpts{
				ErrorHandling: promhttp.ContinueOnError,
				ErrorLog:      newPromLogger(zap.NewNop()),
			},
			tb,
		)

		res := httptest.NewRecorder()
		h.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
		require.Equal(t, http.StatusInternalServerError, res.Code)

		metadatatest.AssertEqualExporterPrometheusScrapeDroppedSeries(t, testTel, []metricdata.DataPoint[int64]{
			{
				Attributes: attribute.NewSet(attribute.String("stage", "gather")),
				Value:      1,
			},
		}, metricdatatest.IgnoreTimestamp())
		assertScrapeDuration(t, testTel, map[string]uint64{"failure": 1})
		_, err = testTel.GetMetric("otelcol.exporter.prometheus.scrape.exported_series")
		require.Error(t, err, "exported_series should not be recorded on failed scrape")
	})

	t.Run("response write error records failure outcome without exported series", func(t *testing.T) {
		exp, testTel := newTestTelemetryExporter(t, &Config{})
		require.NoError(t, exp.ConsumeMetrics(t.Context(), metricBuilder(0, "", "svc", "inst-1")))

		fw := &failingResponseWriter{}
		exp.handler.ServeHTTP(fw, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))

		assertScrapeDuration(t, testTel, map[string]uint64{"failure": 1})
		_, err := testTel.GetMetric("otelcol.exporter.prometheus.scrape.exported_series")
		require.Error(t, err, "exported_series should not be recorded when response write fails")
	})
}

func TestTelemetryScrapeDroppedSeries_Conversion(t *testing.T) {
	exp, testTel := newTestTelemetryExporter(t, &Config{})

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	rm.Resource().Attributes().PutStr("service.instance.id", "inst")
	// Invalid UTF-8 resource attribute causes target_info NewConstMetric to fail.
	rm.Resource().Attributes().PutStr("bad_res_attr", "invalid-\xff-value")

	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	now := time.Now()

	// 1. Valid gauge -> should be exported.
	valid := ms.AppendEmpty()
	valid.SetName("valid_gauge")
	vdp := valid.SetEmptyGauge().DataPoints().AppendEmpty()
	vdp.SetIntValue(1)
	vdp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// 2. Exponential histogram without sum -> dropped in convertExponentialHistogram.
	noSum := ms.AppendEmpty()
	noSum.SetName("exp_hist_no_sum")
	eh1 := noSum.SetEmptyExponentialHistogram()
	eh1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	eh1dp := eh1.DataPoints().AppendEmpty()
	eh1dp.SetScale(0)
	eh1dp.SetCount(1)
	eh1dp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// 3. Exponential histogram with invalid scale < -4 -> dropped in convertExponentialHistogram.
	badScale := ms.AppendEmpty()
	badScale.SetName("exp_hist_bad_scale")
	eh2 := badScale.SetEmptyExponentialHistogram()
	eh2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	eh2dp := eh2.DataPoints().AppendEmpty()
	eh2dp.SetScale(-5)
	eh2dp.SetSum(1)
	eh2dp.SetCount(1)
	eh2dp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// 4. Gauge with invalid UTF-8 label value -> dropped in NewConstMetric.
	badUTF8 := ms.AppendEmpty()
	badUTF8.SetName("bad_utf8_gauge")
	budp := badUTF8.SetEmptyGauge().DataPoints().AppendEmpty()
	budp.SetIntValue(2)
	budp.Attributes().PutStr("tenant", "invalid-\xff-value")
	budp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	require.NoError(t, exp.ConsumeMetrics(t.Context(), md))

	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "valid_gauge")

	// 1 target_info drop + 3 metric conversion drops = 4 conversion drops; 1 exported series.
	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 1},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualExporterPrometheusScrapeDroppedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("stage", "conversion")),
			Value:      4,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestTelemetryTargetInfoMultiErrDrop(t *testing.T) {
	exp, testTel := newTestTelemetryExporter(t, &Config{})

	now := time.Now()
	md := pmetric.NewMetrics()

	// Resource 1: valid resource.
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", "svc-1")
	rm1.Resource().Attributes().PutStr("service.instance.id", "inst-1")
	m1 := rm1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m1.SetName("g1")
	dp1 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
	dp1.SetIntValue(1)
	dp1.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// Resource 2: empty attribute key triggers labelNamer.Build error -> multiErrs abort drops all 2 target_info series.
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", "svc-2")
	rm2.Resource().Attributes().PutStr("service.instance.id", "inst-2")
	rm2.Resource().Attributes().PutStr("", "empty-key-value")
	m2 := rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m2.SetName("g2")
	dp2 := m2.SetEmptyGauge().DataPoints().AppendEmpty()
	dp2.SetIntValue(2)
	dp2.SetTimestamp(pcommon.NewTimestampFromTime(now))

	require.NoError(t, exp.ConsumeMetrics(t.Context(), md))

	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 2},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualExporterPrometheusScrapeDroppedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("stage", "conversion")),
			Value:      2,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestTelemetryScrapeDroppedSeries_Gather(t *testing.T) {
	// WithoutScopeInfo=true causes identical metric+labels across distinct scopes
	// to collide during registry.Gather() under promhttp.ContinueOnError.
	exp, testTel := newTestTelemetryExporter(t, &Config{WithoutScopeInfo: true})

	now := time.Now()
	appendScopeSeries := func(scopeName string, val int64) pmetric.Metrics {
		md := pmetric.NewMetrics()
		sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
		sm.Scope().SetName(scopeName)
		m := sm.Metrics().AppendEmpty()
		m.SetName("colliding_gauge")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(val)
		dp.Attributes().PutStr("tenant", "acme")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
		return md
	}

	// 2 distinct scopes with identical Prometheus label set -> 1 unwrapped gather error.
	require.NoError(t, exp.ConsumeMetrics(t.Context(), appendScopeSeries("scope-a", 1)))
	require.NoError(t, exp.ConsumeMetrics(t.Context(), appendScopeSeries("scope-b", 2)))

	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "colliding_gauge")

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 1},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualExporterPrometheusScrapeDroppedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("stage", "gather")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())

	// Add a 3rd colliding scope -> Registry.Gather() returns prometheus.MultiError with len 2.
	require.NoError(t, exp.ConsumeMetrics(t.Context(), appendScopeSeries("scope-c", 3)))

	res = httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)

	metadatatest.AssertEqualExporterPrometheusScrapeExportedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{Value: 2},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualExporterPrometheusScrapeDroppedSeries(t, testTel, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("stage", "gather")),
			Value:      3, // 1 from first scrape + 2 from MultiError on second scrape
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestCountGatherErrors(t *testing.T) {
	assert.Equal(t, int64(0), countGatherErrors(nil))
	assert.Equal(t, int64(1), countGatherErrors(errors.New("single error")))
	assert.Equal(t, int64(0), countGatherErrors(prometheus.MultiError{}))
	assert.Equal(t, int64(3), countGatherErrors(prometheus.MultiError{
		errors.New("e1"),
		errors.New("e2"),
		errors.New("e3"),
	}))
	wrapped := fmt.Errorf("wrap: %w", prometheus.MultiError{errors.New("e1"), errors.New("e2")})
	assert.Equal(t, int64(2), countGatherErrors(wrapped))
}

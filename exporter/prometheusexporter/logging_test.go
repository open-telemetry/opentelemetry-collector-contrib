// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
)

func assertNoErrorOrWarnLogs(t *testing.T, logs *observer.ObservedLogs) {
	t.Helper()
	entries := logs.All()
	for i := range entries {
		entry := &entries[i]
		assert.Less(t, entry.Level, zapcore.WarnLevel, "unexpected %s log: %s (%v)", entry.Level, entry.Message, entry.ContextMap())
	}
}

func assertLogsDoNotContain(t *testing.T, logs *observer.ObservedLogs, forbidden string) {
	t.Helper()
	entries := logs.All()
	for i := range entries {
		entry := &entries[i]
		assert.NotContains(t, entry.Message, forbidden)
		for k, v := range entry.ContextMap() {
			assert.NotContains(t, k, forbidden)
			assert.NotContains(t, fmt.Sprint(v), forbidden)
		}
	}
}

func newTestLoggingExporter(t *testing.T, cfg *Config, logger *zap.Logger) *prometheusExporter {
	t.Helper()
	set := exportertest.NewNopSettings(metadata.Type)
	if logger != nil {
		set.Logger = logger
	}
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
	return exp
}

func TestSanitizeConversionError(t *testing.T) {
	assert.Equal(t, "invalid UTF-8 label value", sanitizeConversionError(errors.New(`label value "secret-\xff" is not valid UTF-8`)))
	assert.Equal(t, "inconsistent label cardinality", sanitizeConversionError(errors.New(`inconsistent label cardinality: expected 1 label values but got 2 in []string{"secret-a", "secret-b"}`)))
	assert.Equal(t, "instrument type conflict", sanitizeConversionError(errors.New("instrument type conflict")))
}

func TestCollectorConversionLogging(t *testing.T) {
	observedCore, observedLogs := observer.New(zap.DebugLevel)
	exp := newTestLoggingExporter(t, &Config{}, zap.New(observedCore))

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "svc")
	rm.Resource().Attributes().PutStr("service.instance.id", "inst")
	// Invalid UTF-8 resource attribute causes target_info NewConstMetric to fail.
	rm.Resource().Attributes().PutStr("bad_res_attr", "secret-target-invalid-\xff-value")

	ms := rm.ScopeMetrics().AppendEmpty().Metrics()
	now := time.Now()

	// 1. Valid gauge -> should be exported.
	valid := ms.AppendEmpty()
	valid.SetName("valid_gauge")
	vdp := valid.SetEmptyGauge().DataPoints().AppendEmpty()
	vdp.SetIntValue(1)
	vdp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// 2. Exponential histogram without sum -> fails conversion.
	noSum := ms.AppendEmpty()
	noSum.SetName("exp_hist_no_sum")
	eh1 := noSum.SetEmptyExponentialHistogram()
	eh1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	eh1dp := eh1.DataPoints().AppendEmpty()
	eh1dp.SetScale(0)
	eh1dp.SetCount(1)
	eh1dp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	// 3. Gauge with invalid UTF-8 label value -> fails NewConstMetric.
	badUTF8 := ms.AppendEmpty()
	badUTF8.SetName("bad_utf8_gauge")
	budp := badUTF8.SetEmptyGauge().DataPoints().AppendEmpty()
	budp.SetIntValue(2)
	budp.Attributes().PutStr("tenant", "secret-metric-invalid-\xff-value")
	budp.SetTimestamp(pcommon.NewTimestampFromTime(now))

	require.NoError(t, exp.ConsumeMetrics(t.Context(), md))

	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "valid_gauge")

	assertNoErrorOrWarnLogs(t, observedLogs)
	assertLogsDoNotContain(t, observedLogs, "secret-target-invalid")
	assertLogsDoNotContain(t, observedLogs, "secret-metric-invalid")

	failedLogs := observedLogs.FilterMessage("failed to convert metric")
	assert.Equal(t, 3, failedLogs.Len())
}

func TestScrapeGatherErrorLoggingSanitization(t *testing.T) {
	observedCore, observedLogs := observer.New(zap.DebugLevel)
	// WithoutScopeInfo=true causes identical metric+labels across distinct scopes
	// to collide during registry.Gather() under promhttp.ContinueOnError.
	exp := newTestLoggingExporter(t, &Config{WithoutScopeInfo: true}, zap.New(observedCore))

	const secretLabelValue = "acme-secret-tenant-98765"
	now := time.Now()
	appendScopeSeries := func(scopeName string, val int64) pmetric.Metrics {
		md := pmetric.NewMetrics()
		sm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
		sm.Scope().SetName(scopeName)
		m := sm.Metrics().AppendEmpty()
		m.SetName("colliding_gauge")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(val)
		dp.Attributes().PutStr("tenant", secretLabelValue)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(now))
		return md
	}

	require.NoError(t, exp.ConsumeMetrics(t.Context(), appendScopeSeries("scope-a", 1)))
	require.NoError(t, exp.ConsumeMetrics(t.Context(), appendScopeSeries("scope-b", 2)))

	res := httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "colliding_gauge")

	assertNoErrorOrWarnLogs(t, observedLogs)
	assertLogsDoNotContain(t, observedLogs, secretLabelValue)

	gatherLogs := observedLogs.FilterMessage("error gathering Prometheus metrics")
	require.Equal(t, 1, gatherLogs.Len())
	assert.Equal(t, int64(1), gatherLogs.All()[0].ContextMap()["error_count"])
	assert.Equal(t, []any{"duplicate_metric"}, gatherLogs.All()[0].ContextMap()["reasons"])

	// Add a 3rd colliding scope -> Registry.Gather() returns prometheus.MultiError with len 2.
	require.NoError(t, exp.ConsumeMetrics(t.Context(), appendScopeSeries("scope-c", 3)))

	res = httptest.NewRecorder()
	exp.handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, res.Code)

	assertNoErrorOrWarnLogs(t, observedLogs)
	assertLogsDoNotContain(t, observedLogs, secretLabelValue)

	gatherLogs = observedLogs.FilterMessage("error gathering Prometheus metrics")
	require.Equal(t, 2, gatherLogs.Len())
	assert.Equal(t, int64(2), gatherLogs.All()[1].ContextMap()["error_count"])
	assert.Equal(t, []any{"duplicate_metric"}, gatherLogs.All()[1].ContextMap()["reasons"])
}

func TestPromLoggerSanitizationAndLevels(t *testing.T) {
	t.Run("no logs when debug disabled", func(t *testing.T) {
		core, logs := observer.New(zap.InfoLevel)
		pl := newPromLogger(zap.New(core))
		pl.Println("error gathering metrics:", errors.New(`collected metric "foo" { label:{name:"secret" value:"top-secret"} }`))
		pl.Println("error encoding and sending metric family:", errors.New("encode secret"))
		assert.Equal(t, 0, logs.Len())
	})

	t.Run("sanitizes gather and encode errors at debug level", func(t *testing.T) {
		core, logs := observer.New(zap.DebugLevel)
		pl := newPromLogger(zap.New(core))

		pl.Println()
		pl.Println(12345)
		pl.Println("error gathering metrics:", prometheus.MultiError{
			errors.New(`collected metric "m" { label:{name:"pii" value:"secret-1"} } was collected before with the same name and label values`),
			errors.New(`collected metric "m" { label:{name:"pii" value:"secret-2"} } has help "a" but should have "b"`),
		})
		pl.Println("error gathering metrics:", errors.New(`collected metric "m" { label:{name:"pii" value:"secret-3"} }`))
		pl.Println("error encoding and sending metric family:", errors.New(`expected counter in metric m label:{name:"pii" value:"secret-4"}`))
		pl.Println("error encoding and sending metric family:", fmt.Errorf("write failed: %w", syscall.EPIPE))
		pl.Println("error getting writer", errors.New("invalid compression level"))
		pl.Println("error getting writer")
		pl.Println("unknown handler cause", "secret-5")

		assertNoErrorOrWarnLogs(t, logs)
		for _, forbidden := range []string{"secret-1", "secret-2", "secret-3", "secret-4", "secret-5"} {
			assertLogsDoNotContain(t, logs, forbidden)
		}

		gatherLogs := logs.FilterMessage("error gathering Prometheus metrics").All()
		require.Len(t, gatherLogs, 2)
		assert.Equal(t, int64(2), gatherLogs[0].ContextMap()["error_count"])
		assert.Equal(t, []any{"duplicate_metric", "help_conflict"}, gatherLogs[0].ContextMap()["reasons"])
		assert.Equal(t, int64(1), gatherLogs[1].ContextMap()["error_count"])
		assert.Equal(t, []any{"unknown"}, gatherLogs[1].ContextMap()["reasons"])

		encodeLogs := logs.FilterMessage("error encoding and sending Prometheus metric family").All()
		require.Len(t, encodeLogs, 2)
		assert.Equal(t, "encode_error", encodeLogs[0].ContextMap()["reason"])
		assert.Equal(t, "client_disconnected", encodeLogs[1].ContextMap()["reason"])

		writerLogs := logs.FilterMessage("error creating Prometheus response writer").All()
		require.Len(t, writerLogs, 2)
		assert.Equal(t, "invalid compression level", writerLogs[0].ContextMap()["error"])

		handlerLogs := logs.FilterMessage("error in Prometheus HTTP handler").All()
		require.Len(t, handlerLogs, 2)
		assert.Equal(t, "unknown handler cause", handlerLogs[1].ContextMap()["cause"])
	})
}

func TestClassifyGatherAndEncodeErrors(t *testing.T) {
	assert.Equal(t, "unknown", classifyGatherError(nil))
	assert.Equal(t, "duplicate_metric", classifyGatherError(errors.New(`collected metric "m" { } was collected before with the same name and label values`)))
	assert.Equal(t, "suffix_collision", classifyGatherError(errors.New(`collected metric named "a_count" collides with previously collected histogram named "a"`)))
	assert.Equal(t, "help_conflict", classifyGatherError(errors.New(`collected metric a has help "x" but should have "y"`)))
	assert.Equal(t, "type_conflict", classifyGatherError(errors.New(`collected metric a should be a Counter`)))
	assert.Equal(t, "type_conflict", classifyGatherError(errors.New(`collected metric "a" { } is not a GAUGE`)))
	assert.Equal(t, "duplicate_label", classifyGatherError(errors.New(`collected metric "a" { } has two or more labels with the same name: foo`)))
	assert.Equal(t, "invalid_label", classifyGatherError(errors.New(`collected metric "a" { } has a label with an invalid name: 123`)))
	assert.Equal(t, "invalid_label", classifyGatherError(errors.New(`"123" is not a valid label name for metric "a"`)))
	assert.Equal(t, "invalid_metric_name", classifyGatherError(errors.New(`"123" is not a valid metric name`)))
	assert.Equal(t, "collector_panic", classifyGatherError(errors.New(`prometheus collector panic recovered: boom`)))
	assert.Equal(t, "unknown", classifyGatherError(errors.New(`something unexpected with secret`)))

	count, reasons := classifyGatherErrors(nil)
	assert.Equal(t, int64(0), count)
	assert.Nil(t, reasons)

	count, reasons = classifyGatherErrors(prometheus.MultiError{})
	assert.Equal(t, int64(0), count)
	assert.Empty(t, reasons)

	wrapped := fmt.Errorf("wrap: %w", prometheus.MultiError{
		errors.New(`collected metric "m" { } was collected before with the same name and label values`),
		errors.New(`collected metric a has help "x" but should have "y"`),
	})
	count, reasons = classifyGatherErrors(wrapped)
	assert.Equal(t, int64(2), count)
	assert.Equal(t, []string{"duplicate_metric", "help_conflict"}, reasons)

	assert.Equal(t, "context_canceled", classifyEncodeError(context.Canceled))
	assert.Equal(t, "deadline_exceeded", classifyEncodeError(context.DeadlineExceeded))
	assert.Equal(t, "client_disconnected", classifyEncodeError(syscall.EPIPE))
	assert.Equal(t, "client_disconnected", classifyEncodeError(syscall.ECONNRESET))
	assert.Equal(t, "client_disconnected", classifyEncodeError(io.ErrClosedPipe))
	assert.Equal(t, "encode_error", classifyEncodeError(errors.New("raw encode failure")))
}

func TestAccumulatorLoggingCompliance(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	acc := newAccumulator(zap.New(core), time.Hour).(*lastValueAccumulator)
	now := time.Now()

	// 1. Unknown metric type -> Debug log, no Error/Warn.
	rmUnknown := pmetric.NewResourceMetrics()
	mUnknown := rmUnknown.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mUnknown.SetName("unknown_metric")
	assert.Equal(t, 0, acc.Accumulate(rmUnknown))
	assert.Equal(t, 1, logs.FilterMessage("failed to translate metric").Len())

	// 2. Unspecified temporality Sum -> Debug log, no Error/Warn.
	rmUnspecSum := pmetric.NewResourceMetrics()
	mUnspecSum := rmUnspecSum.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	mUnspecSum.SetName("unspec_sum")
	mUnspecSum.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityUnspecified)
	mUnspecSum.Sum().DataPoints().AppendEmpty().SetIntValue(1)
	assert.Equal(t, 0, acc.Accumulate(rmUnspecSum))
	assert.Equal(t, 1, logs.FilterMessage("dropping sum metric with unspecified aggregation temporality").Len())

	// 3. Misaligned delta Histogram (dropped and reset) + unspecified temporality Histogram -> Debug only.
	appendHist := func(temp pmetric.AggregationTemporality, startTs, ts time.Time) pmetric.ResourceMetrics {
		rm := pmetric.NewResourceMetrics()
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("delta_hist")
		h := m.SetEmptyHistogram()
		h.SetAggregationTemporality(temp)
		dp := h.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTs))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetCount(1)
		dp.SetSum(1)
		dp.BucketCounts().FromRaw([]uint64{1, 0})
		dp.ExplicitBounds().FromRaw([]float64{1})
		return rm
	}
	assert.Equal(t, 1, acc.Accumulate(appendHist(pmetric.AggregationTemporalityDelta, now.Add(-10*time.Second), now.Add(-5*time.Second))))
	// Misaligned drop (start <= prev timestamp).
	assert.Equal(t, 0, acc.Accumulate(appendHist(pmetric.AggregationTemporalityDelta, now.Add(-8*time.Second), now.Add(-3*time.Second))))
	// Misaligned reset (start > prev timestamp).
	assert.Equal(t, 1, acc.Accumulate(appendHist(pmetric.AggregationTemporalityDelta, now.Add(-2*time.Second), now.Add(-1*time.Second))))
	// Unspecified temporality drop on existing series.
	assert.Equal(t, 0, acc.Accumulate(appendHist(pmetric.AggregationTemporalityUnspecified, now, now.Add(time.Second))))
	assert.Equal(t, 1, logs.FilterMessage("dropping histogram datapoint with unsupported aggregation temporality").Len())

	// 4. Misaligned delta ExponentialHistogram (dropped and reset) + unspecified temporality -> Debug only.
	appendExpHist := func(temp pmetric.AggregationTemporality, startTs, ts time.Time) pmetric.ResourceMetrics {
		rm := pmetric.NewResourceMetrics()
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("delta_exp_hist")
		h := m.SetEmptyExponentialHistogram()
		h.SetAggregationTemporality(temp)
		dp := h.DataPoints().AppendEmpty()
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTs))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetScale(0)
		dp.SetCount(1)
		dp.SetSum(1)
		return rm
	}
	assert.Equal(t, 1, acc.Accumulate(appendExpHist(pmetric.AggregationTemporalityDelta, now.Add(-10*time.Second), now.Add(-5*time.Second))))
	// Misaligned drop.
	assert.Equal(t, 0, acc.Accumulate(appendExpHist(pmetric.AggregationTemporalityDelta, now.Add(-8*time.Second), now.Add(-3*time.Second))))
	// Misaligned reset.
	assert.Equal(t, 1, acc.Accumulate(appendExpHist(pmetric.AggregationTemporalityDelta, now.Add(-2*time.Second), now.Add(-1*time.Second))))
	// Unspecified temporality drop on existing series.
	assert.Equal(t, 0, acc.Accumulate(appendExpHist(pmetric.AggregationTemporalityUnspecified, now, now.Add(time.Second))))
	assert.Equal(t, 1, logs.FilterMessage("dropping exponential histogram datapoint with unsupported aggregation temporality").Len())

	assert.Equal(t, 4, logs.FilterMessage("Misaligned starting timestamps").Len())
	assert.Equal(t, 2, logs.FilterMessage("Dropped misaligned histogram datapoint").Len())

	// 5. Metric expiration logs structured metric_name field.
	acc.metricExpiration = time.Millisecond
	time.Sleep(5 * time.Millisecond)
	acc.cleanupExpired()
	expiredLogs := logs.FilterMessage("metric expired").All()
	require.Len(t, expiredLogs, 2)
	for _, entry := range expiredLogs {
		assert.Contains(t, entry.ContextMap(), "metric_name")
	}

	assertNoErrorOrWarnLogs(t, logs)
}

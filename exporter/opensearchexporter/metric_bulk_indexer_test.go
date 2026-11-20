// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestMetricJoinedError(t *testing.T) {
	tests := []struct {
		name     string
		errs     []error
		hasError bool
	}{
		{"no errors", nil, false},
		{"single error", []error{errors.New("test")}, true},
		{"multiple errors", []error{errors.New("err1"), errors.New("err2")}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mbi := &metricBulkIndexer{errs: tt.errs}
			err := mbi.joinedError()
			if (err != nil) != tt.hasError {
				t.Errorf("joinedError() = %v, expected error: %v", err, tt.hasError)
			}
		})
	}
}

func TestMetricProcessItemFailure(t *testing.T) {
	transportErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	tests := []struct {
		name          string
		status        int
		itemErr       error
		wantPermanent bool
	}{
		{"retry status", 500, nil, false},
		{"permanent status", 400, nil, true},
		{"no status, encoding error", 0, errors.New("json: unsupported value"), true},
		{"no status, transport error", 0, transportErr, false},
		{"no status, flush-wrapped deadline", 0, fmt.Errorf("flush: %w", context.DeadlineExceeded), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mbi := &metricBulkIndexer{}
			resp := opensearchapi.BulkRespItem{Status: tt.status}
			mbi.processItemFailure(resp, tt.itemErr, pmetric.NewMetrics())
			err := mbi.joinedError()
			require.Error(t, err)
			require.Equal(t, tt.wantPermanent, consumererror.IsPermanent(err))
		})
	}
}

func TestMetricOnIndexerErrorIsRetryable(t *testing.T) {
	mbi := &metricBulkIndexer{}
	// A transport failure surfaces through the bulk indexer's OnError callback.
	mbi.onIndexerError(t.Context(), &net.OpError{Op: "dial", Err: errors.New("connection refused")})
	err := mbi.joinedError()
	require.Error(t, err)
	require.False(t, consumererror.IsPermanent(err),
		"indexer-level transport error must be retryable, not permanent (otherwise retry_on_failure silently drops the batch)")
}

func TestMetricIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, true},
		{"net.OpError", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
		{
			"url.Error wrapping net.OpError",
			&url.Error{Op: "Post", URL: "http://localhost", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}},
			true,
		},
		{"flush-wrapped deadline", fmt.Errorf("flush: %w", context.DeadlineExceeded), true},
		{"encoding error", errors.New("json: unsupported value"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isRetryableError(tt.err))
		})
	}
}

func TestNewMetricBulkIndexerWithPipeline(t *testing.T) {
	tests := []struct {
		name     string
		pipeline string
	}{
		{"empty pipeline", ""},
		{"with pipeline", "my-pipeline"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mbi := newMetricBulkIndexer("create", nil, tt.pipeline)
			if mbi.pipeline != tt.pipeline {
				t.Errorf("expected pipeline %q, got %q", tt.pipeline, mbi.pipeline)
			}
			if mbi.bulkAction != "create" {
				t.Errorf("expected bulkAction 'create', got %s", mbi.bulkAction)
			}
		})
	}
}

func TestMetricNewBulkIndexerItem(t *testing.T) {
	mbi := &metricBulkIndexer{bulkAction: "index"}
	payload := []byte(`{"test": "data"}`)
	indexName := "test-index"
	item := mbi.newBulkIndexerItem(payload, indexName)

	if item.Action != "index" {
		t.Errorf("expected action 'index', got %s", item.Action)
	}
	if item.Index != indexName {
		t.Errorf("expected index %s, got %s", indexName, item.Index)
	}
	if item.Body == nil {
		t.Error("expected body to be set")
	}
}

func TestMakeMetric(t *testing.T) {
	resource := pcommon.NewResource()
	resource.Attributes().PutStr("service.name", "test-service")
	scope := pcommon.NewInstrumentationScope()
	scope.SetName("test-scope")

	metric := pmetric.NewMetric()
	metric.SetName("test-metric")
	metric.SetUnit("1")
	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(true)
	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetIntValue(7)

	metrics := makeMetric(resource, "resource-schema", scope, "scope-schema", metric, dp)

	if metrics.ResourceMetrics().Len() != 1 {
		t.Fatal("expected 1 resource metric")
	}
	rm := metrics.ResourceMetrics().At(0)
	if rm.SchemaUrl() != "resource-schema" {
		t.Errorf("expected schema 'resource-schema', got %s", rm.SchemaUrl())
	}
	if rm.ScopeMetrics().Len() != 1 {
		t.Fatal("expected 1 scope metric")
	}
	sm := rm.ScopeMetrics().At(0)
	if sm.SchemaUrl() != "scope-schema" {
		t.Errorf("expected schema 'scope-schema', got %s", sm.SchemaUrl())
	}
	if sm.Metrics().Len() != 1 {
		t.Fatal("expected 1 metric")
	}
	m := sm.Metrics().At(0)
	if m.Name() != "test-metric" {
		t.Errorf("expected name 'test-metric', got %s", m.Name())
	}
	if m.Type() != pmetric.MetricTypeSum {
		t.Fatalf("expected sum metric, got %s", m.Type())
	}
	if !m.Sum().IsMonotonic() {
		t.Error("expected monotonic sum")
	}
	if m.Sum().AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
		t.Error("expected cumulative temporality")
	}
	if m.Sum().DataPoints().Len() != 1 {
		t.Fatal("expected 1 data point")
	}
	if m.Sum().DataPoints().At(0).IntValue() != 7 {
		t.Errorf("expected value 7, got %d", m.Sum().DataPoints().At(0).IntValue())
	}
}

func TestMetricDataPoints(t *testing.T) {
	metric := pmetric.NewMetric()
	gauge := metric.SetEmptyGauge()
	gauge.DataPoints().AppendEmpty().SetIntValue(1)
	gauge.DataPoints().AppendEmpty().SetIntValue(2)

	dps := metricDataPoints(metric)
	if len(dps) != 2 {
		t.Fatalf("expected 2 data points, got %d", len(dps))
	}

	metric = pmetric.NewMetric()
	metric.SetEmptyHistogram().DataPoints().AppendEmpty()
	if len(metricDataPoints(metric)) != 1 {
		t.Fatal("expected 1 histogram data point")
	}

	metric = pmetric.NewMetric()
	metric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	if len(metricDataPoints(metric)) != 1 {
		t.Fatal("expected 1 exponential histogram data point")
	}

	metric = pmetric.NewMetric()
	metric.SetEmptySummary().DataPoints().AppendEmpty()
	if len(metricDataPoints(metric)) != 1 {
		t.Fatal("expected 1 summary data point")
	}

	// A metric with no type set has no data points.
	metric = pmetric.NewMetric()
	if len(metricDataPoints(metric)) != 0 {
		t.Fatal("expected no data points for empty metric")
	}
}

func newTestMetrics(count int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("test-scope")
	m := sm.Metrics().AppendEmpty()
	m.SetName("test.gauge")
	gauge := m.SetEmptyGauge()
	now := pcommon.NewTimestampFromTime(time.Now())
	for i := range count {
		dp := gauge.DataPoints().AppendEmpty()
		dp.SetTimestamp(now)
		dp.SetIntValue(int64((i + 1) * 10))
	}
	return md
}

func collectGaugeValues(md pmetric.Metrics) []int64 {
	var values []int64
	for i := range md.ResourceMetrics().Len() {
		values = appendScopeMetricGaugeValues(values, md.ResourceMetrics().At(i).ScopeMetrics())
	}
	return values
}

func appendScopeMetricGaugeValues(values []int64, sms pmetric.ScopeMetricsSlice) []int64 {
	for j := range sms.Len() {
		ms := sms.At(j).Metrics()
		for k := range ms.Len() {
			values = appendGaugeDataPointValues(values, ms.At(k).Gauge().DataPoints())
		}
	}
	return values
}

func appendGaugeDataPointValues(values []int64, dps pmetric.NumberDataPointSlice) []int64 {
	for l := range dps.Len() {
		values = append(values, dps.At(l).IntValue())
	}
	return values
}

func assertRetryMetricValues(t *testing.T, err error, wantValues []int64) {
	t.Helper()
	if len(wantValues) == 0 {
		return
	}
	var retryErr consumererror.Metrics
	require.ErrorAs(t, err, &retryErr)
	require.Equal(t, len(wantValues), retryErr.Data().DataPointCount())
	require.Equal(t, wantValues, collectGaugeValues(retryErr.Data()))
}

func TestMetricBulkIndexerPartialBulkFailureRetry(t *testing.T) {
	tests := []struct {
		name          string
		statuses      []int
		wantPermanent bool
		wantValues    []int64
	}{
		{
			name:       "multiple retryable item failures",
			statuses:   []int{http.StatusCreated, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			wantValues: []int64{20, 30},
		},
		{
			name:       "mixed permanent and retryable item failures",
			statuses:   []int{http.StatusBadRequest, http.StatusTooManyRequests, http.StatusServiceUnavailable},
			wantValues: []int64{20, 30},
		},
		{
			name:          "only permanent item failures",
			statuses:      []int{http.StatusCreated, http.StatusBadRequest},
			wantPermanent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := newMockBulkServer(t, "ss4o_metrics-default-namespace", tt.statuses...)
			defer ts.Close()

			client, err := newOpenSearchClient(ts.URL, ts.Client(), zap.NewNop())
			require.NoError(t, err)

			cfg := newDefaultConfig().(*Config)
			model := &encodeModel{sso: true, dataset: cfg.Dataset, namespace: cfg.Namespace}
			ir := newIndexResolver("ss4o_metrics", cfg.Dataset, cfg.Namespace)

			mbi := newMetricBulkIndexer(cfg.BulkAction, model, cfg.Pipeline)
			require.NoError(t, mbi.start(client))

			mbi.submit(t.Context(), newTestMetrics(len(tt.statuses)), ir, cfg, time.Now())
			mbi.close(t.Context())

			joined := mbi.joinedError()
			require.Error(t, joined)
			require.Equal(t, tt.wantPermanent, consumererror.IsPermanent(joined))
			assertRetryMetricValues(t, joined, tt.wantValues)
		})
	}
}

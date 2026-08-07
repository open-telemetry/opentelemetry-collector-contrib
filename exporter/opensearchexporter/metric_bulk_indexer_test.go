// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	tests := []struct {
		name         string
		status       int
		initialErrs  int
		expectedErrs int
	}{
		{"retry status", 500, 0, 1},
		{"permanent status", 400, 0, 1},
		{"no status", 0, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mbi := &metricBulkIndexer{errs: make([]error, tt.initialErrs)}
			resp := opensearchapi.BulkRespItem{Status: tt.status}
			metrics := pmetric.NewMetrics()
			mbi.processItemFailure(resp, nil, metrics)
			if len(mbi.errs) != tt.expectedErrs {
				t.Errorf("expected %d errors, got %d", tt.expectedErrs, len(mbi.errs))
			}
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

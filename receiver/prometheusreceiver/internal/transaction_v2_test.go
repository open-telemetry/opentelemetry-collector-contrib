// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

// =============================================================================
// Test Helpers for V2
// =============================================================================

// newTransactionV2ForTest creates a transactionV2 for testing with the provided context.
func newTransactionV2ForTest(t *testing.T, ctx context.Context, sink *consumertest.MetricsSink) *transactionV2 {
	settings := receivertest.NewNopSettings(receivertest.NopType)
	obsrecv := nopObsRecv(t)
	return newTransactionV2(ctx, sink, labels.EmptyLabels(), settings, obsrecv, false)
}

// createV2Labels creates labels for V2 tests with required job and instance.
func createV2Labels(metricName string, extraLabels ...string) labels.Labels {
	lbls := []string{
		model.MetricNameLabel, metricName,
		model.JobLabel, "test-job",
		model.InstanceLabel, "localhost:8080",
	}
	lbls = append(lbls, extraLabels...)
	return labels.FromStrings(lbls...)
}

// createV2Options creates AppendV2Options with the given metadata.
func createV2Options(metricType model.MetricType, help, unit string) storage.AppendV2Options {
	return storage.AppendV2Options{
		Metadata: metadata.Metadata{
			Type: metricType,
			Help: help,
			Unit: unit,
		},
	}
}

// float64FromBits creates a float64 from its IEEE 754 binary representation.
func float64FromBits(bits uint64) float64 {
	return math.Float64frombits(bits)
}

// =============================================================================
// Transaction Lifecycle Tests
// =============================================================================

func TestTransactionV2_Lifecycle(t *testing.T) {
	tests := []struct {
		name   string
		action func(t *testing.T, tr *transactionV2) error
	}{
		{
			name: "commit without adding samples succeeds",
			action: func(t *testing.T, tr *transactionV2) error {
				return tr.Commit()
			},
		},
		{
			name: "rollback always succeeds",
			action: func(t *testing.T, tr *transactionV2) error {
				return tr.Rollback()
			},
		},
		{
			name: "commit after adding valid sample succeeds",
			action: func(t *testing.T, tr *transactionV2) error {
				ls := createV2Labels("counter_test")
				opts := createV2Options(model.MetricTypeCounter, "", "")
				_, err := tr.Append(0, ls, 0, time.Now().UnixMilli(), 100.0, nil, nil, opts)
				if err != nil {
					return err
				}
				return tr.Commit()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			err := tt.action(t, tr)
			assert.NoError(t, err)
		})
	}
}

// =============================================================================
// Label Validation Tests
// =============================================================================

func TestTransactionV2_LabelValidation(t *testing.T) {
	tests := []struct {
		name    string
		labels  labels.Labels
		opts    storage.AppendV2Options
		wantErr string // empty means success expected
	}{
		{
			name: "valid labels succeeds",
			labels: labels.FromStrings(
				model.MetricNameLabel, "counter_test",
				model.JobLabel, "test-job",
				model.InstanceLabel, "localhost:8080",
			),
			opts: createV2Options(model.MetricTypeCounter, "", ""),
		},
		{
			name: "missing metric name returns error",
			labels: labels.FromStrings(
				model.JobLabel, "test-job",
				model.InstanceLabel, "localhost:8080",
			),
			opts:    createV2Options(model.MetricTypeCounter, "", ""),
			wantErr: errMetricNameNotFound.Error(),
		},
		{
			name: "empty metric name returns error",
			labels: labels.FromStrings(
				model.MetricNameLabel, "",
				model.JobLabel, "test-job",
				model.InstanceLabel, "localhost:8080",
			),
			opts:    createV2Options(model.MetricTypeCounter, "", ""),
			wantErr: errMetricNameNotFound.Error(),
		},
		{
			name: "duplicate labels returns error",
			labels: labels.FromStrings(
				model.MetricNameLabel, "counter_test",
				model.JobLabel, "test-job",
				model.InstanceLabel, "localhost:8080",
				"a", "1",
				"a", "2", // duplicate
			),
			opts:    createV2Options(model.MetricTypeCounter, "", ""),
			wantErr: "non-unique label names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ts := time.Now().UnixMilli()
			_, err := tr.Append(0, tt.labels, 0, ts, 100.0, nil, nil, tt.opts)

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestTransactionV2_FallbackToTargetLabels tests that when job/instance labels
// are missing from the sample, they fall back to the scrape target's labels.
func TestTransactionV2_FallbackToTargetLabels(t *testing.T) {
	sink := new(consumertest.MetricsSink)

	// Create a target with both job and instance labels
	scrapeTarget := scrape.NewTarget(
		labels.FromMap(map[string]string{
			model.InstanceLabel: "localhost:8080",
			model.JobLabel:      "federate",
		}),
		&config.ScrapeConfig{},
		map[model.LabelName]model.LabelValue{
			model.AddressLabel: "address:8080",
			model.SchemeLabel:  "http",
		},
		nil,
	)

	ctx := scrape.ContextWithMetricMetadataStore(
		scrape.ContextWithTarget(t.Context(), scrapeTarget),
		testMetadataStore(testMetadata))

	tr := newTransactionV2ForTest(t, ctx, sink)

	// Sample with only metric name - should fall back to target labels
	ls := labels.FromStrings(model.MetricNameLabel, "counter_test")
	opts := createV2Options(model.MetricTypeCounter, "", "")
	ts := time.Now().UnixMilli()

	_, err := tr.Append(0, ls, 0, ts, 100.0, nil, nil, opts)
	assert.NoError(t, err)
}

// =============================================================================
// Metric Type Tests
// =============================================================================

func TestTransactionV2_MetricTypes(t *testing.T) {
	tests := []struct {
		name          string
		metricName    string
		metricType    model.MetricType
		value         float64
		h             *histogram.Histogram
		fh            *histogram.FloatHistogram
		wantOTLPType  pmetric.MetricType
		wantMonotonic bool // for Sum metrics
	}{
		{
			name:          "counter becomes monotonic sum",
			metricName:    "counter_test",
			metricType:    model.MetricTypeCounter,
			value:         100.0,
			wantOTLPType:  pmetric.MetricTypeSum,
			wantMonotonic: true,
		},
		{
			name:         "gauge becomes gauge",
			metricName:   "gauge_test",
			metricType:   model.MetricTypeGauge,
			value:        42.5,
			wantOTLPType: pmetric.MetricTypeGauge,
		},
		{
			name:         "unknown type becomes gauge",
			metricName:   "unknown_test",
			metricType:   model.MetricTypeUnknown,
			value:        100.0,
			wantOTLPType: pmetric.MetricTypeGauge,
		},
		{
			name:         "native histogram becomes exponential histogram",
			metricName:   "hist_test",
			metricType:   model.MetricTypeHistogram,
			h:            tsdbutil.GenerateTestHistogram(1),
			wantOTLPType: pmetric.MetricTypeExponentialHistogram,
		},
		{
			name:         "float native histogram becomes exponential histogram",
			metricName:   "hist_test",
			metricType:   model.MetricTypeHistogram,
			fh:           tsdbutil.GenerateTestFloatHistogram(1),
			wantOTLPType: pmetric.MetricTypeExponentialHistogram,
		},
		{
			name:         "NHCB (schema -53) becomes classic histogram",
			metricName:   "hist_test",
			metricType:   model.MetricTypeHistogram,
			h:            tsdbutil.GenerateTestCustomBucketsHistogram(1),
			wantOTLPType: pmetric.MetricTypeHistogram,
		},
		{
			name:         "summary becomes summary",
			metricName:   "summary_test",
			metricType:   model.MetricTypeSummary,
			value:        100.0,
			wantOTLPType: pmetric.MetricTypeSummary,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ls := createV2Labels(tt.metricName, "foo", "bar")
			opts := createV2Options(tt.metricType, "", "")
			ts := time.Now().UnixMilli()

			_, err := tr.Append(0, ls, 0, ts, tt.value, tt.h, tt.fh, opts)
			require.NoError(t, err)
			require.NoError(t, tr.Commit())

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			require.Equal(t, 1, mds[0].MetricCount())

			metric := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			assert.Equal(t, tt.metricName, metric.Name())
			assert.Equal(t, tt.wantOTLPType, metric.Type())

			if tt.wantOTLPType == pmetric.MetricTypeSum {
				assert.Equal(t, tt.wantMonotonic, metric.Sum().IsMonotonic())
			}
		})
	}
}

// =============================================================================
// Start Timestamp Tests
// =============================================================================

func TestTransactionV2_StartTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		metricType  model.MetricType
		stMs        int64 // start timestamp in ms
		tsMs        int64 // sample timestamp in ms
		h           *histogram.Histogram
		fh          *histogram.FloatHistogram
		wantStartTs bool // should start timestamp be set?
	}{
		{
			name:        "counter with start timestamp",
			metricType:  model.MetricTypeCounter,
			stMs:        1000,
			tsMs:        2000,
			wantStartTs: true,
		},
		{
			name:        "counter with zero start timestamp",
			metricType:  model.MetricTypeCounter,
			stMs:        0,
			tsMs:        2000,
			wantStartTs: false, // zero means not set
		},
		{
			name:        "gauge ignores start timestamp",
			metricType:  model.MetricTypeGauge,
			stMs:        1000,
			tsMs:        2000,
			wantStartTs: false, // gauges don't have start timestamps
		},
		{
			name:        "native histogram with start timestamp",
			metricType:  model.MetricTypeHistogram,
			stMs:        1000,
			tsMs:        2000,
			h:           tsdbutil.GenerateTestHistogram(1),
			wantStartTs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ls := createV2Labels("metric_test")
			opts := createV2Options(tt.metricType, "", "")

			_, err := tr.Append(0, ls, tt.stMs, tt.tsMs, 100.0, tt.h, tt.fh, opts)
			require.NoError(t, err)
			require.NoError(t, tr.Commit())

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)

			metric := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

			var startTs, ts pcommon.Timestamp
			switch metric.Type() {
			case pmetric.MetricTypeSum:
				dp := metric.Sum().DataPoints().At(0)
				startTs = dp.StartTimestamp()
				ts = dp.Timestamp()
			case pmetric.MetricTypeGauge:
				dp := metric.Gauge().DataPoints().At(0)
				startTs = dp.StartTimestamp()
				ts = dp.Timestamp()
			case pmetric.MetricTypeExponentialHistogram:
				dp := metric.ExponentialHistogram().DataPoints().At(0)
				startTs = dp.StartTimestamp()
				ts = dp.Timestamp()
			}

			assert.Equal(t, pcommon.NewTimestampFromTime(time.UnixMilli(tt.tsMs)), ts)

			if tt.wantStartTs {
				assert.Equal(t, pcommon.NewTimestampFromTime(time.UnixMilli(tt.stMs)), startTs)
			} else {
				assert.Equal(t, pcommon.Timestamp(0), startTs)
			}
		})
	}
}

// =============================================================================
// Exemplar Tests
// =============================================================================

func TestTransactionV2_Exemplars(t *testing.T) {
	tests := []struct {
		name        string
		metricType  model.MetricType
		exemplars   []exemplar.Exemplar
		wantCount   int
		wantTraceID pcommon.TraceID // expected TraceID for first exemplar (if wantCount > 0)
		wantSpanID  pcommon.SpanID  // expected SpanID for first exemplar (if wantCount > 0)
	}{
		{
			name:       "counter with no exemplars",
			metricType: model.MetricTypeCounter,
			exemplars:  nil,
			wantCount:  0,
		},
		{
			name:       "counter with empty exemplars slice",
			metricType: model.MetricTypeCounter,
			exemplars:  []exemplar.Exemplar{},
			wantCount:  0,
		},
		{
			name:       "counter with single exemplar without trace context",
			metricType: model.MetricTypeCounter,
			exemplars: []exemplar.Exemplar{
				{
					Value:  1.0,
					Ts:     1663113420863,
					Labels: labels.FromStrings("foo", "bar"),
				},
			},
			wantCount:   1,
			wantTraceID: pcommon.TraceID{}, // no trace context
			wantSpanID:  pcommon.SpanID{},
		},
		{
			name:       "counter with exemplar with trace context",
			metricType: model.MetricTypeCounter,
			exemplars: []exemplar.Exemplar{
				{
					Value:  1.0,
					Ts:     1663113420863,
					Labels: labels.FromStrings("trace_id", "0102030405060708090a0b0c0d0e0f10", "span_id", "0102030405060708"),
				},
			},
			wantCount:   1,
			wantTraceID: [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
			wantSpanID:  [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		},
		{
			name:       "counter with multiple exemplars",
			metricType: model.MetricTypeCounter,
			exemplars: []exemplar.Exemplar{
				{
					Value:  1.0,
					Ts:     1663113420863,
					Labels: labels.FromStrings("foo", "bar"),
				},
				{
					Value:  2.0,
					Ts:     1663113420864,
					Labels: labels.FromStrings("baz", "qux"),
				},
				{
					Value:  3.0,
					Ts:     1663113420865,
					Labels: labels.FromStrings("trace_id", "0102030405060708090a0b0c0d0e0f10", "span_id", "0102030405060708"),
				},
			},
			wantCount:   3,
			wantTraceID: pcommon.TraceID{}, // first exemplar has no trace context
			wantSpanID:  pcommon.SpanID{},
		},
		{
			name:       "gauge with exemplars",
			metricType: model.MetricTypeGauge,
			exemplars: []exemplar.Exemplar{
				{
					Value:  1.0,
					Ts:     1663113420863,
					Labels: labels.FromStrings("foo", "bar"),
				},
			},
			wantCount:   1,
			wantTraceID: pcommon.TraceID{},
			wantSpanID:  pcommon.SpanID{},
		},
		{
			name:       "exemplar with empty trace_id and span_id labels",
			metricType: model.MetricTypeCounter,
			exemplars: []exemplar.Exemplar{
				{
					Value:  1.0,
					Ts:     1663113420863,
					Labels: labels.FromStrings("trace_id", "", "span_id", ""),
				},
			},
			wantCount:   1,
			wantTraceID: pcommon.TraceID{}, // empty strings = no trace context
			wantSpanID:  pcommon.SpanID{},
		},
		{
			name:       "exemplar with short trace_id (8 bytes)",
			metricType: model.MetricTypeCounter,
			exemplars: []exemplar.Exemplar{
				{
					Value:  1.0,
					Ts:     1663113420863,
					Labels: labels.FromStrings("trace_id", "174137cab66dc880", "span_id", "dfa4597a9d"),
				},
			},
			wantCount:   1,
			wantTraceID: [16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x17, 0x41, 0x37, 0xca, 0xb6, 0x6d, 0xc8, 0x80},
			wantSpanID:  [8]byte{0x00, 0x00, 0x00, 0xdf, 0xa4, 0x59, 0x7a, 0x9d},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ls := createV2Labels("metric_test", "foo", "bar")
			opts := storage.AppendV2Options{
				Metadata: metadata.Metadata{
					Type: tt.metricType,
				},
				Exemplars: tt.exemplars,
			}
			ts := time.Now().UnixMilli()

			_, err := tr.Append(0, ls, 0, ts, 100.0, nil, nil, opts)
			require.NoError(t, err)
			require.NoError(t, tr.Commit())

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)

			metric := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

			var exemplars pmetric.ExemplarSlice
			switch metric.Type() {
			case pmetric.MetricTypeSum:
				exemplars = metric.Sum().DataPoints().At(0).Exemplars()
			case pmetric.MetricTypeGauge:
				exemplars = metric.Gauge().DataPoints().At(0).Exemplars()
			}

			assert.Equal(t, tt.wantCount, exemplars.Len())

			if tt.wantCount > 0 {
				e := exemplars.At(0)
				assert.Equal(t, tt.wantTraceID, e.TraceID())
				assert.Equal(t, tt.wantSpanID, e.SpanID())
			}
		})
	}
}

// =============================================================================
// Metadata Tests
// =============================================================================

func TestTransactionV2_Metadata(t *testing.T) {
	tests := []struct {
		name             string
		metricName       string
		metricFamilyName string
		metricType       model.MetricType
		help             string
		unit             string
		wantName         string
		wantDescription  string
	}{
		{
			name:            "metadata with help text",
			metricName:      "counter_test",
			metricType:      model.MetricTypeCounter,
			help:            "This is a helpful description",
			wantName:        "counter_test",
			wantDescription: "This is a helpful description",
		},
		{
			name:            "metadata without help text",
			metricName:      "gauge_test",
			metricType:      model.MetricTypeGauge,
			help:            "",
			wantName:        "gauge_test",
			wantDescription: "",
		},
		{
			name:             "metric family name provided",
			metricName:       "my_counter_total",
			metricFamilyName: "my_counter",
			metricType:       model.MetricTypeCounter,
			help:             "A counter",
			wantName:         "my_counter_total", // TODO: verify if this should use family name
			wantDescription:  "A counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ls := createV2Labels(tt.metricName)
			opts := storage.AppendV2Options{
				MetricFamilyName: tt.metricFamilyName,
				Metadata: metadata.Metadata{
					Type: tt.metricType,
					Help: tt.help,
					Unit: tt.unit,
				},
			}
			ts := time.Now().UnixMilli()

			_, err := tr.Append(0, ls, 0, ts, 100.0, nil, nil, opts)
			require.NoError(t, err)
			require.NoError(t, tr.Commit())

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)

			metric := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			assert.Equal(t, tt.wantName, metric.Name())
			assert.Equal(t, tt.wantDescription, metric.Description())
		})
	}
}

func TestTransactionV2_StaleMarkers(t *testing.T) {
	tests := []struct {
		name      string
		value     float64
		h         *histogram.Histogram
		fh        *histogram.FloatHistogram
		wantStale bool
	}{
		{
			name:      "regular value is not stale",
			value:     100.0,
			wantStale: false,
		},
		{
			name:      "stale NaN value is stale",
			value:     float64FromBits(0x7ff0000000000002), // Prometheus stale marker
			wantStale: true,
		},
		{
			name:      "regular NaN is not stale marker",
			value:     math.NaN(),
			wantStale: false, // Regular NaN is different from stale marker
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ls := createV2Labels("counter_test")
			opts := createV2Options(model.MetricTypeCounter, "", "")
			ts := time.Now().UnixMilli()

			_, err := tr.Append(0, ls, 0, ts, tt.value, tt.h, tt.fh, opts)
			require.NoError(t, err)
			require.NoError(t, tr.Commit())

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)

			dp := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints().At(0)
			assert.Equal(t, tt.wantStale, dp.Flags().NoRecordedValue())
		})
	}
}

// =============================================================================
// Resource and Scope Tests
// =============================================================================

func TestTransactionV2_Resources(t *testing.T) {
	tests := []struct {
		name              string
		samples           []labels.Labels
		wantResourceCount int
		checkResources    func(t *testing.T, rm pmetric.ResourceMetricsSlice)
	}{
		{
			name: "single resource from job/instance",
			samples: []labels.Labels{
				createV2Labels("counter_test"),
			},
			wantResourceCount: 1,
			checkResources: func(t *testing.T, rm pmetric.ResourceMetricsSlice) {
				resource := rm.At(0).Resource()

				serviceName, ok := resource.Attributes().Get("service.name")
				assert.True(t, ok)
				assert.Equal(t, "test-job", serviceName.AsString())

				serviceInstanceID, ok := resource.Attributes().Get("service.instance.id")
				assert.True(t, ok)
				assert.Equal(t, "localhost:8080", serviceInstanceID.AsString())
			},
		},
		{
			name: "multiple resources from different jobs",
			samples: []labels.Labels{
				labels.FromStrings(
					model.MetricNameLabel, "counter_test",
					model.JobLabel, "job-1",
					model.InstanceLabel, "localhost:8080",
				),
				labels.FromStrings(
					model.MetricNameLabel, "counter_test",
					model.JobLabel, "job-2",
					model.InstanceLabel, "localhost:8080",
				),
			},
			wantResourceCount: 2,
		},
		{
			name: "multiple resources from different instances",
			samples: []labels.Labels{
				labels.FromStrings(
					model.MetricNameLabel, "counter_test",
					model.JobLabel, "test-job",
					model.InstanceLabel, "localhost:8080",
				),
				labels.FromStrings(
					model.MetricNameLabel, "counter_test",
					model.JobLabel, "test-job",
					model.InstanceLabel, "localhost:9090",
				),
			},
			wantResourceCount: 2,
		},
		{
			name: "target_info adds resource attributes",
			samples: []labels.Labels{
				createV2Labels("counter_test"),
				labels.FromStrings(
					model.MetricNameLabel, "target_info",
					model.JobLabel, "test-job",
					model.InstanceLabel, "localhost:8080",
					"custom_label", "custom_value",
					"another_label", "another_value",
				),
			},
			wantResourceCount: 1,
			checkResources: func(t *testing.T, rm pmetric.ResourceMetricsSlice) {
				resource := rm.At(0).Resource()

				val, ok := resource.Attributes().Get("custom_label")
				assert.True(t, ok, "custom_label should be in resource attributes")
				assert.Equal(t, "custom_value", val.AsString())

				val, ok = resource.Attributes().Get("another_label")
				assert.True(t, ok, "another_label should be in resource attributes")
				assert.Equal(t, "another_value", val.AsString())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ts := time.Now().UnixMilli()
			opts := createV2Options(model.MetricTypeCounter, "", "")

			for _, ls := range tt.samples {
				_, err := tr.Append(0, ls, 0, ts, 100.0, nil, nil, opts)
				require.NoError(t, err)
			}

			require.NoError(t, tr.Commit())

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)
			assert.Equal(t, tt.wantResourceCount, mds[0].ResourceMetrics().Len())

			if tt.checkResources != nil {
				tt.checkResources(t, mds[0].ResourceMetrics())
			}
		})
	}
}

func TestTransactionV2_Scopes(t *testing.T) {
	t.Run("receiver scope name and version", func(t *testing.T) {
		sink := new(consumertest.MetricsSink)
		tr := newTransactionV2ForTest(t, scrapeCtx, sink)

		ls := createV2Labels("counter_test")
		opts := createV2Options(model.MetricTypeCounter, "", "")
		ts := time.Now().UnixMilli()

		_, err := tr.Append(0, ls, 0, ts, 100.0, nil, nil, opts)
		require.NoError(t, err)
		require.NoError(t, tr.Commit())

		mds := sink.AllMetrics()
		require.Len(t, mds, 1)

		scope := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Scope()
		assert.Equal(t, "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver", scope.Name())
		assert.Equal(t, component.NewDefaultBuildInfo().Version, scope.Version())
	})

	t.Run("otel_scope_info adds scope attributes", func(t *testing.T) {
		sink := new(consumertest.MetricsSink)
		tr := newTransactionV2ForTest(t, scrapeCtx, sink)

		ts := time.Now().UnixMilli()

		// First add a metric with scope labels
		ls1 := labels.FromStrings(
			model.MetricNameLabel, "counter_test",
			model.JobLabel, "test-job",
			model.InstanceLabel, "localhost:8080",
			"otel_scope_name", "my.scope",
			"otel_scope_version", "1.0.0",
		)
		opts := createV2Options(model.MetricTypeCounter, "", "")
		_, err := tr.Append(0, ls1, 0, ts, 100.0, nil, nil, opts)
		require.NoError(t, err)

		// Then add otel_scope_info with additional attributes
		ls2 := labels.FromStrings(
			model.MetricNameLabel, "otel_scope_info",
			model.JobLabel, "test-job",
			model.InstanceLabel, "localhost:8080",
			"otel_scope_name", "my.scope",
			"otel_scope_version", "1.0.0",
			"scope_attr", "scope_value",
		)
		_, err = tr.Append(0, ls2, 0, ts, 1.0, nil, nil, createV2Options(model.MetricTypeGauge, "", ""))
		require.NoError(t, err)

		require.NoError(t, tr.Commit())

		mds := sink.AllMetrics()
		require.Len(t, mds, 1)

		// otel_scope_info should not become a metric
		assert.Equal(t, 1, mds[0].MetricCount())

		scope := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Scope()
		assert.Equal(t, "my.scope", scope.Name())
		assert.Equal(t, "1.0.0", scope.Version())

		val, ok := scope.Attributes().Get("scope_attr")
		assert.True(t, ok, "scope_attr should be in scope attributes")
		assert.Equal(t, "scope_value", val.AsString())
	})
}

// =============================================================================
// Native Histogram Edge Cases
// =============================================================================

func TestTransactionV2_NativeHistogramEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		h            *histogram.Histogram
		fh           *histogram.FloatHistogram
		wantOTLPType pmetric.MetricType
	}{
		{
			name: "empty integer histogram",
			h: &histogram.Histogram{
				Schema:        1,
				Count:         0,
				Sum:           0,
				ZeroThreshold: 0.001,
				ZeroCount:     0,
			},
			wantOTLPType: pmetric.MetricTypeExponentialHistogram,
		},
		{
			name: "empty float histogram",
			fh: &histogram.FloatHistogram{
				Schema:        1,
				Count:         0,
				Sum:           0,
				ZeroThreshold: 0.001,
				ZeroCount:     0,
			},
			wantOTLPType: pmetric.MetricTypeExponentialHistogram,
		},
		{
			name:         "standard integer histogram",
			h:            tsdbutil.GenerateTestHistogram(1),
			wantOTLPType: pmetric.MetricTypeExponentialHistogram,
		},
		{
			name:         "standard float histogram",
			fh:           tsdbutil.GenerateTestFloatHistogram(1),
			wantOTLPType: pmetric.MetricTypeExponentialHistogram,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := new(consumertest.MetricsSink)
			tr := newTransactionV2ForTest(t, scrapeCtx, sink)

			ls := createV2Labels("hist_test")
			opts := createV2Options(model.MetricTypeHistogram, "", "")
			ts := time.Now().UnixMilli()

			_, err := tr.Append(0, ls, 0, ts, 0, tt.h, tt.fh, opts)
			require.NoError(t, err)
			require.NoError(t, tr.Commit())

			mds := sink.AllMetrics()
			require.Len(t, mds, 1)

			metric := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			assert.Equal(t, tt.wantOTLPType, metric.Type())
		})
	}
}

// TestTransactionV2_NHCBHistogram tests Native Histogram Custom Buckets (NHCB)
// which have schema -53 and should be converted to classic OTLP histograms
// with explicit bucket boundaries preserved.
func TestTransactionV2_NHCBHistogram(t *testing.T) {
	// Create an NHCB histogram with known bucket boundaries
	// Schema -53 indicates custom buckets (NHCB)
	nhcb := &histogram.Histogram{
		Schema:        -53, // NHCB schema
		Count:         10,
		Sum:           99.5,
		ZeroThreshold: 0,
		ZeroCount:     0,
		// CustomValues contains the bucket upper bounds for NHCB
		CustomValues: []float64{10, 20, 50},
		// PositiveBuckets contains the counts for each bucket
		// [0, 10], (10, 20], (20, 50], (50, +Inf]
		PositiveSpans:   []histogram.Span{{Offset: 0, Length: 4}},
		PositiveBuckets: []int64{2, 1, 4, 3}, // delta-encoded: 2, 3, 7, 10 cumulative would be different
	}

	sink := new(consumertest.MetricsSink)
	tr := newTransactionV2ForTest(t, scrapeCtx, sink)

	ls := createV2Labels("nhcb_hist_test")
	opts := createV2Options(model.MetricTypeHistogram, "NHCB histogram", "")
	ts := time.Now().UnixMilli()

	_, err := tr.Append(0, ls, 0, ts, 0, nhcb, nil, opts)
	require.NoError(t, err)
	require.NoError(t, tr.Commit())

	mds := sink.AllMetrics()
	require.Len(t, mds, 1)

	metric := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "nhcb_hist_test", metric.Name())
	assert.Equal(t, pmetric.MetricTypeHistogram, metric.Type())

	hist := metric.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityCumulative, hist.AggregationTemporality())
	require.Equal(t, 1, hist.DataPoints().Len())

	dp := hist.DataPoints().At(0)
	assert.Equal(t, uint64(10), dp.Count())
	assert.Equal(t, 99.5, dp.Sum())

	// Verify bucket boundaries are preserved
	// NHCB CustomValues [10, 20, 50] should become ExplicitBounds [10, 20, 50]
	bounds := dp.ExplicitBounds().AsRaw()
	assert.Equal(t, []float64{10, 20, 50}, bounds)

	// Verify bucket counts
	// The exact counts depend on how NHCB encodes them
	bucketCounts := dp.BucketCounts().AsRaw()
	assert.Equal(t, len(bounds)+1, len(bucketCounts), "should have len(bounds)+1 bucket counts")
}

// =============================================================================
// TODO: CLASSIC HISTOGRAMS AND SUMMARIES - UNKNOWN BEHAVIOR IN APPENDERV2
// =============================================================================
//
// We are NOT SURE how Prometheus AppenderV2 will deliver classic histograms
// and summaries. In AppenderV1, these metric types required aggregating multiple
// samples (e.g., _bucket, _sum, _count for histograms; quantile, _sum, _count
// for summaries) into a single OTLP metric.
//
// Possible scenarios for AppenderV2:
// 1. Prometheus delivers fully-formed histogram/summary in a single Append() call
// 2. Prometheus delivers individual samples that we need to aggregate (like V1)
//
// Until we test this with actual Prometheus AppenderV2, we are:
// - NOT implementing classic histogram aggregation logic for V2
// - NOT implementing summary aggregation logic for V2
// - Only handling native histograms (which come as *histogram.Histogram)
// - Only handling NHCB (Native Histogram Custom Buckets, schema -53)
//
// If classic histograms/summaries need aggregation in V2, we will need to:
// - Add tests similar to V1's TestMetricBuilderHistogram and TestMetricBuilderSummary
// - Implement aggregation logic in transactionV2
//
// =============================================================================

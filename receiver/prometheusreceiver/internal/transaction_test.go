// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const (
	startTimestamp      = pcommon.Timestamp(1555366608340000000)
	ts                  = int64(1555366610000)
	interval            = int64(15 * 1000)
	tsNanos             = pcommon.Timestamp(ts * 1e6)
	tsPlusIntervalNanos = pcommon.Timestamp((ts + interval) * 1e6)
)

var (
	target = scrape.NewTarget(
		// processedLabels contain label values after processing (e.g. relabeling)
		labels.FromMap(map[string]string{
			model.InstanceLabel: "localhost:8080",
		}),
		// discoveredLabels contain labels prior to any processing
		labels.FromMap(map[string]string{
			model.AddressLabel: "address:8080",
			model.SchemeLabel:  "http",
		}),
		nil)

	scrapeCtx = scrape.ContextWithMetricMetadataStore(
		scrape.ContextWithTarget(context.Background(), target),
		testMetadataStore(testMetadata))
)

func TestTransactionCommitWithoutAdding(t *testing.T) {
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, consumertest.NewNop(), nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	assert.NoError(t, tr.Commit())
}

func TestTransactionRollbackDoesNothing(t *testing.T) {
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, consumertest.NewNop(), nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	assert.NoError(t, tr.Rollback())
}

func TestTransactionUpdateMetadataDoesNothing(t *testing.T) {
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, consumertest.NewNop(), nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	_, err := tr.UpdateMetadata(0, labels.New(), metadata.Metadata{})
	assert.NoError(t, err)
}

func TestTransactionAppendNoTarget(t *testing.T) {
	badLabels := labels.FromStrings(model.MetricNameLabel, "counter_test")
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, consumertest.NewNop(), nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	_, err := tr.Append(0, badLabels, time.Now().Unix()*1000, 1.0)
	assert.Error(t, err)
}

func TestTransactionAppendNoMetricName(t *testing.T) {
	jobNotFoundLb := labels.FromMap(map[string]string{
		model.InstanceLabel: "localhost:8080",
		model.JobLabel:      "test2",
	})
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, consumertest.NewNop(), nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	_, err := tr.Append(0, jobNotFoundLb, time.Now().Unix()*1000, 1.0)
	assert.ErrorIs(t, err, errMetricNameNotFound)

	assert.ErrorIs(t, tr.Commit(), errNoDataToBuild)
}

func TestTransactionAppendEmptyMetricName(t *testing.T) {
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, consumertest.NewNop(), nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	_, err := tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test2",
		model.MetricNameLabel: "",
	}), time.Now().Unix()*1000, 1.0)
	assert.ErrorIs(t, err, errMetricNameNotFound)
}

func TestTransactionAppendResource(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	_, err := tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "counter_test",
	}), time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)
	_, err = tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: startTimeMetricName,
	}), time.Now().UnixMilli(), 1.0)
	assert.NoError(t, err)
	assert.NoError(t, tr.Commit())
	expectedResource := CreateResource("test", "localhost:8080", labels.FromStrings(model.SchemeLabel, "http"))
	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	gotResource := mds[0].ResourceMetrics().At(0).Resource()
	require.Equal(t, expectedResource, gotResource)
}

func TestReceiverVersionAndNameAreAttached(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	_, err := tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "counter_test",
	}), time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)
	assert.NoError(t, tr.Commit())

	expectedResource := CreateResource("test", "localhost:8080", labels.FromStrings(model.SchemeLabel, "http"))
	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	gotResource := mds[0].ResourceMetrics().At(0).Resource()
	require.Equal(t, expectedResource, gotResource)

	gotScope := mds[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Scope()
	require.Equal(t, receiverName, gotScope.Name())
	require.Equal(t, component.NewDefaultBuildInfo().Version, gotScope.Version())
}

func TestTransactionCommitErrorWhenAdjusterError(t *testing.T) {
	goodLabels := labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "counter_test",
	})
	sink := new(consumertest.MetricsSink)
	adjusterErr := errors.New("adjuster error")
	tr := newTransaction(scrapeCtx, &errorAdjuster{err: adjusterErr}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
	_, err := tr.Append(0, goodLabels, time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)
	assert.ErrorIs(t, tr.Commit(), adjusterErr)
}

// Ensure that we reject duplicate label keys. See https://github.com/open-telemetry/wg-prometheus/issues/44.
func TestTransactionAppendDuplicateLabels(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)

	dupLabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "counter_test",
		"a", "1",
		"a", "6",
		"z", "9",
	)

	_, err := tr.Append(0, dupLabels, 1917, 1.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid sample: non-unique label names: "a"`)
}

func TestTransactionAppendHistogramNoLe(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	receiverSettings := receivertest.NewNopCreateSettings()
	core, observedLogs := observer.New(zap.InfoLevel)
	receiverSettings.Logger = zap.New(core)
	tr := newTransaction(
		scrapeCtx,
		&startTimeAdjuster{startTime: startTimestamp},
		sink,
		nil,
		receiverSettings,
		nopObsRecv(t),
		false,
	)

	goodLabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "hist_test_bucket",
	)

	_, err := tr.Append(0, goodLabels, 1917, 1.0)
	require.NoError(t, err)
	assert.Equal(t, 1, observedLogs.Len())
	assert.Equal(t, 1, observedLogs.FilterMessage("failed to add datapoint").Len())

	assert.NoError(t, tr.Commit())
	assert.Len(t, sink.AllMetrics(), 0)
}

func TestTransactionAppendSummaryNoQuantile(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	receiverSettings := receivertest.NewNopCreateSettings()
	core, observedLogs := observer.New(zap.InfoLevel)
	receiverSettings.Logger = zap.New(core)
	tr := newTransaction(
		scrapeCtx,
		&startTimeAdjuster{startTime: startTimestamp},
		sink,
		nil,
		receiverSettings,
		nopObsRecv(t),
		false,
	)

	goodLabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "summary_test",
	)

	_, err := tr.Append(0, goodLabels, 1917, 1.0)
	require.NoError(t, err)
	assert.Equal(t, 1, observedLogs.Len())
	assert.Equal(t, 1, observedLogs.FilterMessage("failed to add datapoint").Len())

	assert.NoError(t, tr.Commit())
	assert.Len(t, sink.AllMetrics(), 0)
}

func TestTransactionAppendValidAndInvalid(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	receiverSettings := receivertest.NewNopCreateSettings()
	core, observedLogs := observer.New(zap.InfoLevel)
	receiverSettings.Logger = zap.New(core)
	tr := newTransaction(
		scrapeCtx,
		&startTimeAdjuster{startTime: startTimestamp},
		sink,
		nil,
		receiverSettings,
		nopObsRecv(t),
		false,
	)

	// a valid counter
	_, err := tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "counter_test",
	}), time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)

	// summary without quantiles, should be ignored
	summarylabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "summary_test",
	)

	_, err = tr.Append(0, summarylabels, 1917, 1.0)
	require.NoError(t, err)

	assert.Equal(t, 1, observedLogs.Len())
	assert.Equal(t, 1, observedLogs.FilterMessage("failed to add datapoint").Len())

	assert.NoError(t, tr.Commit())
	expectedResource := CreateResource("test", "localhost:8080", labels.FromStrings(model.SchemeLabel, "http"))
	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	gotResource := mds[0].ResourceMetrics().At(0).Resource()
	require.Equal(t, expectedResource, gotResource)
	require.Equal(t, 1, mds[0].MetricCount())
}

func TestAppendExemplarWithNoMetricName(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)

	labels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
	)

	_, err := tr.AppendExemplar(0, labels, exemplar.Exemplar{Value: 0})
	assert.Equal(t, errMetricNameNotFound, err)
}

func TestAppendExemplarWithEmptyMetricName(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)

	labels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "",
	)
	_, err := tr.AppendExemplar(0, labels, exemplar.Exemplar{Value: 0})
	assert.Equal(t, errMetricNameNotFound, err)
}

func TestAppendExemplarWithDuplicateLabels(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)

	labels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "",
		"a", "b",
		"a", "c",
	)
	_, err := tr.AppendExemplar(0, labels, exemplar.Exemplar{Value: 0})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid sample: non-unique label names: "a"`)
}

func TestAppendExemplarWithoutAddingMetric(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)

	labels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "counter_test",
		"a", "b",
	)
	_, err := tr.AppendExemplar(0, labels, exemplar.Exemplar{Value: 0})
	assert.NoError(t, err)
}

func TestAppendExemplarWithNoLabels(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)

	_, err := tr.AppendExemplar(0, nil, exemplar.Exemplar{Value: 0})
	assert.Equal(t, errNoJobInstance, err)
}

func TestAppendExemplarWithEmptyLabelArray(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)

	_, err := tr.AppendExemplar(0, []labels.Label{}, exemplar.Exemplar{Value: 0})
	assert.Equal(t, errNoJobInstance, err)
}

func nopObsRecv(t *testing.T) *receiverhelper.ObsReport {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID("prometheus"),
		Transport:              transport,
		ReceiverCreateSettings: receivertest.NewNopCreateSettings(),
	})
	require.NoError(t, err)
	return obsrecv
}

func TestMetricBuilderCounters(t *testing.T) {
	tests := []buildTestData{
		{
			name: "single-item",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("counter_test", 100, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("counter_test")
				sum := m0.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)
				pt0 := sum.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "single-item-with-exemplars",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint(
							"counter_test",
							100,
							[]exemplar.Exemplar{
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: model.MetricNameLabel, Value: "counter_test"}, {Name: model.JobLabel, Value: "job"}, {Name: model.InstanceLabel, Value: "instance"}, {Name: "foo", Value: "bar"}},
								},
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: ""}, {Name: "span_id", Value: ""}},
								},
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "10a47365b8aa04e08291fab9deca84db6170"}, {Name: "span_id", Value: "719cee4a669fd7d109ff"}},
								},
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "174137cab66dc880"}, {Name: "span_id", Value: "dfa4597a9d"}},
								},
							},
							"foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("counter_test")
				sum := m0.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)
				pt0 := sum.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				e0 := pt0.Exemplars().AppendEmpty()
				e0.SetTimestamp(timestampFromMs(1663113420863))
				e0.SetDoubleValue(1)
				e0.FilteredAttributes().PutStr(model.MetricNameLabel, "counter_test")
				e0.FilteredAttributes().PutStr(model.JobLabel, "job")
				e0.FilteredAttributes().PutStr(model.InstanceLabel, "instance")
				e0.FilteredAttributes().PutStr("foo", "bar")

				e1 := pt0.Exemplars().AppendEmpty()
				e1.SetTimestamp(timestampFromMs(1663113420863))
				e1.SetDoubleValue(1)
				e1.FilteredAttributes().PutStr("foo", "bar")

				e2 := pt0.Exemplars().AppendEmpty()
				e2.SetTimestamp(timestampFromMs(1663113420863))
				e2.SetDoubleValue(1)
				e2.FilteredAttributes().PutStr("foo", "bar")
				e2.SetTraceID([16]byte{0x10, 0xa4, 0x73, 0x65, 0xb8, 0xaa, 0x04, 0xe0, 0x82, 0x91, 0xfa, 0xb9, 0xde, 0xca, 0x84, 0xdb})
				e2.SetSpanID([8]byte{0x71, 0x9c, 0xee, 0x4a, 0x66, 0x9f, 0xd7, 0xd1})

				e3 := pt0.Exemplars().AppendEmpty()
				e3.SetTimestamp(timestampFromMs(1663113420863))
				e3.SetDoubleValue(1)
				e3.FilteredAttributes().PutStr("foo", "bar")
				e3.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x17, 0x41, 0x37, 0xca, 0xb6, 0x6d, 0xc8, 0x80})
				e3.SetSpanID([8]byte{0x00, 0x00, 0x00, 0xdf, 0xa4, 0x59, 0x7a, 0x9d})

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "two-items",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("counter_test", 150, nil, "foo", "bar"),
						createDataPoint("counter_test", 25, nil, "foo", "other"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("counter_test")
				sum := m0.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)
				pt0 := sum.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(150.0)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				pt1 := sum.DataPoints().AppendEmpty()
				pt1.SetDoubleValue(25.0)
				pt1.SetStartTimestamp(startTimestamp)
				pt1.SetTimestamp(tsNanos)
				pt1.Attributes().PutStr("foo", "other")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "two-metrics",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("counter_test", 150, nil, "foo", "bar"),
						createDataPoint("counter_test", 25, nil, "foo", "other"),
						createDataPoint("counter_test2", 100, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("counter_test")
				sum0 := m0.SetEmptySum()
				sum0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum0.SetIsMonotonic(true)
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(150.0)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				pt1 := sum0.DataPoints().AppendEmpty()
				pt1.SetDoubleValue(25.0)
				pt1.SetStartTimestamp(startTimestamp)
				pt1.SetTimestamp(tsNanos)
				pt1.Attributes().PutStr("foo", "other")

				m1 := mL0.AppendEmpty()
				m1.SetName("counter_test2")
				sum1 := m1.SetEmptySum()
				sum1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum1.SetIsMonotonic(true)
				pt2 := sum1.DataPoints().AppendEmpty()
				pt2.SetDoubleValue(100.0)
				pt2.SetStartTimestamp(startTimestamp)
				pt2.SetTimestamp(tsNanos)
				pt2.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "metrics-with-poor-names",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("poor_name_count", 100, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("poor_name_count")
				sum := m0.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)
				pt0 := sum.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderGauges(t *testing.T) {
	tests := []buildTestData{
		{
			name: "one-gauge",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 100, nil, "foo", "bar"),
					},
				},
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 90, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				gauge0 := m0.SetEmptyGauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				md1 := pmetric.NewMetrics()
				mL1 := md1.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				gauge1 := m1.SetEmptyGauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleValue(90.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(tsPlusIntervalNanos)
				pt1.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0, md1}
			},
		},
		{
			name: "one-gauge-with-exemplars",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint(
							"gauge_test",
							100,
							[]exemplar.Exemplar{
								{
									Value:  2,
									Ts:     1663350815890,
									Labels: []labels.Label{{Name: model.MetricNameLabel, Value: "counter_test"}, {Name: model.JobLabel, Value: "job"}, {Name: model.InstanceLabel, Value: "instance"}, {Name: "foo", Value: "bar"}},
								},
								{
									Value:  2,
									Ts:     1663350815890,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: ""}, {Name: "span_id", Value: ""}},
								},
								{
									Value:  2,
									Ts:     1663350815890,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "10a47365b8aa04e08291fab9deca84db6170"}, {Name: "span_id", Value: "719cee4a669fd7d109ff"}},
								},
								{
									Value:  2,
									Ts:     1663350815890,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "174137cab66dc880"}, {Name: "span_id", Value: "dfa4597a9d"}},
								},
							},
							"foo", "bar"),
					},
				},
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 90, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				gauge0 := m0.SetEmptyGauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				e0 := pt0.Exemplars().AppendEmpty()
				e0.SetTimestamp(timestampFromMs(1663350815890))
				e0.SetDoubleValue(2)
				e0.FilteredAttributes().PutStr(model.MetricNameLabel, "counter_test")
				e0.FilteredAttributes().PutStr(model.JobLabel, "job")
				e0.FilteredAttributes().PutStr(model.InstanceLabel, "instance")
				e0.FilteredAttributes().PutStr("foo", "bar")

				e1 := pt0.Exemplars().AppendEmpty()
				e1.SetTimestamp(timestampFromMs(1663350815890))
				e1.SetDoubleValue(2)
				e1.FilteredAttributes().PutStr("foo", "bar")

				e2 := pt0.Exemplars().AppendEmpty()
				e2.SetTimestamp(timestampFromMs(1663350815890))
				e2.SetDoubleValue(2)
				e2.FilteredAttributes().PutStr("foo", "bar")
				e2.SetTraceID([16]byte{0x10, 0xa4, 0x73, 0x65, 0xb8, 0xaa, 0x04, 0xe0, 0x82, 0x91, 0xfa, 0xb9, 0xde, 0xca, 0x84, 0xdb})
				e2.SetSpanID([8]byte{0x71, 0x9c, 0xee, 0x4a, 0x66, 0x9f, 0xd7, 0xd1})

				e3 := pt0.Exemplars().AppendEmpty()
				e3.SetTimestamp(timestampFromMs(1663350815890))
				e3.SetDoubleValue(2)
				e3.FilteredAttributes().PutStr("foo", "bar")
				e3.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x17, 0x41, 0x37, 0xca, 0xb6, 0x6d, 0xc8, 0x80})
				e3.SetSpanID([8]byte{0x00, 0x00, 0x00, 0xdf, 0xa4, 0x59, 0x7a, 0x9d})

				md1 := pmetric.NewMetrics()
				mL1 := md1.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				gauge1 := m1.SetEmptyGauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleValue(90.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(tsPlusIntervalNanos)
				pt1.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0, md1}
			},
		},
		{
			name: "gauge-with-different-tags",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 100, nil, "foo", "bar"),
						createDataPoint("gauge_test", 200, nil, "bar", "foo"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				gauge0 := m0.SetEmptyGauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				pt1 := gauge0.DataPoints().AppendEmpty()
				pt1.SetDoubleValue(200.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(tsNanos)
				pt1.Attributes().PutStr("bar", "foo")

				return []pmetric.Metrics{md0}
			},
		},
		{
			// TODO: A decision need to be made. If we want to have the behavior which can generate different tag key
			//  sets because metrics come and go
			name: "gauge-comes-and-go-with-different-tagset",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 100, nil, "foo", "bar"),
						createDataPoint("gauge_test", 200, nil, "bar", "foo"),
					},
				},
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 20, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				gauge0 := m0.SetEmptyGauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				pt1 := gauge0.DataPoints().AppendEmpty()
				pt1.SetDoubleValue(200.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(tsNanos)
				pt1.Attributes().PutStr("bar", "foo")

				md1 := pmetric.NewMetrics()
				mL1 := md1.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				gauge1 := m1.SetEmptyGauge()
				pt2 := gauge1.DataPoints().AppendEmpty()
				pt2.SetDoubleValue(20.0)
				pt2.SetStartTimestamp(0)
				pt2.SetTimestamp(tsPlusIntervalNanos)
				pt2.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0, md1}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderUntyped(t *testing.T) {
	tests := []buildTestData{
		{
			name: "one-unknown",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("unknown_test", 100, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("unknown_test")
				gauge0 := m0.SetEmptyGauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "no-type-hint",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("something_not_exists", 100, nil, "foo", "bar"),
						createDataPoint("theother_not_exists", 200, nil, "foo", "bar"),
						createDataPoint("theother_not_exists", 300, nil, "bar", "foo"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("something_not_exists")
				gauge0 := m0.SetEmptyGauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				m1 := mL0.AppendEmpty()
				m1.SetName("theother_not_exists")
				gauge1 := m1.SetEmptyGauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleValue(200.0)
				pt1.SetTimestamp(tsNanos)
				pt1.Attributes().PutStr("foo", "bar")

				pt2 := gauge1.DataPoints().AppendEmpty()
				pt2.SetDoubleValue(300.0)
				pt2.SetTimestamp(tsNanos)
				pt2.Attributes().PutStr("bar", "foo")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "untype-metric-poor-names",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("some_count", 100, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("some_count")
				gauge0 := m0.SetEmptyGauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleValue(100.0)
				pt0.SetTimestamp(tsNanos)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderHistogram(t *testing.T) {
	tests := []buildTestData{
		{
			name: "single item",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, nil, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 10, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, nil, "foo", "bar"),
						createDataPoint("hist_test_count", 10, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.ExplicitBounds().FromRaw([]float64{10, 20})
				pt0.BucketCounts().FromRaw([]uint64{1, 1, 8})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "single item with exemplars",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint(
							"hist_test_bucket",
							1,
							[]exemplar.Exemplar{
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: model.MetricNameLabel, Value: "counter_test"}, {Name: model.JobLabel, Value: "job"}, {Name: model.InstanceLabel, Value: "instance"}, {Name: "foo", Value: "bar"}},
								},
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: ""}, {Name: "span_id", Value: ""}, {Name: "le", Value: "20"}},
								},
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "10a47365b8aa04e08291fab9deca84db6170"}, {Name: "traceid", Value: "e3688e1aa2961786"}, {Name: "span_id", Value: "719cee4a669fd7d109ff"}},
								},
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "174137cab66dc880"}, {Name: "span_id", Value: "dfa4597a9d"}},
								},
								{
									Value:  1,
									Ts:     1663113420863,
									Labels: []labels.Label{{Name: "foo", Value: "bar"}, {Name: "trace_id", Value: "174137cab66dc88"}, {Name: "span_id", Value: "dfa4597a9"}},
								},
							},
							"foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 10, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, nil, "foo", "bar"),
						createDataPoint("hist_test_count", 10, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.ExplicitBounds().FromRaw([]float64{10, 20})
				pt0.BucketCounts().FromRaw([]uint64{1, 1, 8})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				e0 := pt0.Exemplars().AppendEmpty()
				e0.SetTimestamp(timestampFromMs(1663113420863))
				e0.SetDoubleValue(1)
				e0.FilteredAttributes().PutStr(model.MetricNameLabel, "counter_test")
				e0.FilteredAttributes().PutStr(model.JobLabel, "job")
				e0.FilteredAttributes().PutStr(model.InstanceLabel, "instance")
				e0.FilteredAttributes().PutStr("foo", "bar")

				e1 := pt0.Exemplars().AppendEmpty()
				e1.SetTimestamp(timestampFromMs(1663113420863))
				e1.SetDoubleValue(1)
				e1.FilteredAttributes().PutStr("foo", "bar")
				e1.FilteredAttributes().PutStr("le", "20")

				e2 := pt0.Exemplars().AppendEmpty()
				e2.SetTimestamp(timestampFromMs(1663113420863))
				e2.SetDoubleValue(1)
				e2.FilteredAttributes().PutStr("foo", "bar")
				e2.FilteredAttributes().PutStr("traceid", "e3688e1aa2961786")
				e2.SetTraceID([16]byte{0x10, 0xa4, 0x73, 0x65, 0xb8, 0xaa, 0x04, 0xe0, 0x82, 0x91, 0xfa, 0xb9, 0xde, 0xca, 0x84, 0xdb})
				e2.SetSpanID([8]byte{0x71, 0x9c, 0xee, 0x4a, 0x66, 0x9f, 0xd7, 0xd1})

				e3 := pt0.Exemplars().AppendEmpty()
				e3.SetTimestamp(timestampFromMs(1663113420863))
				e3.SetDoubleValue(1)
				e3.FilteredAttributes().PutStr("foo", "bar")
				e3.SetTraceID([16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x17, 0x41, 0x37, 0xca, 0xb6, 0x6d, 0xc8, 0x80})
				e3.SetSpanID([8]byte{0x00, 0x00, 0x00, 0xdf, 0xa4, 0x59, 0x7a, 0x9d})

				e4 := pt0.Exemplars().AppendEmpty()
				e4.SetTimestamp(timestampFromMs(1663113420863))
				e4.SetDoubleValue(1)
				e4.FilteredAttributes().PutStr("foo", "bar")
				e4.FilteredAttributes().PutStr("trace_id", "174137cab66dc88")
				e4.FilteredAttributes().PutStr("span_id", "dfa4597a9")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "multi-groups",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, nil, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 10, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, nil, "foo", "bar"),
						createDataPoint("hist_test_count", 10, nil, "foo", "bar"),
						createDataPoint("hist_test_bucket", 1, nil, "key2", "v2", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "key2", "v2", "le", "20"),
						createDataPoint("hist_test_bucket", 3, nil, "key2", "v2", "le", "+inf"),
						createDataPoint("hist_test_sum", 50, nil, "key2", "v2"),
						createDataPoint("hist_test_count", 3, nil, "key2", "v2"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.ExplicitBounds().FromRaw([]float64{10, 20})
				pt0.BucketCounts().FromRaw([]uint64{1, 1, 8})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				pt1 := hist0.DataPoints().AppendEmpty()
				pt1.SetCount(3)
				pt1.SetSum(50)
				pt1.ExplicitBounds().FromRaw([]float64{10, 20})
				pt1.BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt1.SetTimestamp(tsNanos)
				pt1.SetStartTimestamp(startTimestamp)
				pt1.Attributes().PutStr("key2", "v2")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "multi-groups-and-families",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, nil, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 10, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, nil, "foo", "bar"),
						createDataPoint("hist_test_count", 10, nil, "foo", "bar"),
						createDataPoint("hist_test_bucket", 1, nil, "key2", "v2", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "key2", "v2", "le", "20"),
						createDataPoint("hist_test_bucket", 3, nil, "key2", "v2", "le", "+inf"),
						createDataPoint("hist_test_sum", 50, nil, "key2", "v2"),
						createDataPoint("hist_test_count", 3, nil, "key2", "v2"),
						createDataPoint("hist_test2_bucket", 1, nil, "foo", "bar", "le", "10"),
						createDataPoint("hist_test2_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test2_bucket", 3, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test2_sum", 50, nil, "foo", "bar"),
						createDataPoint("hist_test2_count", 3, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.ExplicitBounds().FromRaw([]float64{10, 20})
				pt0.BucketCounts().FromRaw([]uint64{1, 1, 8})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				pt1 := hist0.DataPoints().AppendEmpty()
				pt1.SetCount(3)
				pt1.SetSum(50)
				pt1.ExplicitBounds().FromRaw([]float64{10, 20})
				pt1.BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt1.SetTimestamp(tsNanos)
				pt1.SetStartTimestamp(startTimestamp)
				pt1.Attributes().PutStr("key2", "v2")

				m1 := mL0.AppendEmpty()
				m1.SetName("hist_test2")
				hist1 := m1.SetEmptyHistogram()
				hist1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt2 := hist1.DataPoints().AppendEmpty()
				pt2.SetCount(3)
				pt2.SetSum(50)
				pt2.ExplicitBounds().FromRaw([]float64{10, 20})
				pt2.BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt2.SetTimestamp(tsNanos)
				pt2.SetStartTimestamp(startTimestamp)
				pt2.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "unordered-buckets",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 10, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_bucket", 1, nil, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_sum", 99, nil, "foo", "bar"),
						createDataPoint("hist_test_count", 10, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.ExplicitBounds().FromRaw([]float64{10, 20})
				pt0.BucketCounts().FromRaw([]uint64{1, 1, 8})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 3, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_count", 3, nil, "foo", "bar"),
						createDataPoint("hist_test_sum", 100, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(3)
				pt0.SetSum(100)
				pt0.BucketCounts().FromRaw([]uint64{3})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket-noninf",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 3, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_count", 3, nil, "foo", "bar"),
						createDataPoint("hist_test_sum", 100, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(3)
				pt0.SetSum(100)
				pt0.BucketCounts().FromRaw([]uint64{3, 0})
				pt0.ExplicitBounds().FromRaw([]float64{20})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "no-sum",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, nil, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 3, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_count", 3, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(3)
				pt0.ExplicitBounds().FromRaw([]float64{10, 20})
				pt0.BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "corrupted-no-buckets",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_sum", 99, nil, "foo", "bar"),
						createDataPoint("hist_test_count", 10, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				hist0 := m0.SetEmptyHistogram()
				hist0.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.BucketCounts().FromRaw([]uint64{10})
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "corrupted-no-count",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, nil, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, nil, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 3, nil, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				return []pmetric.Metrics{pmetric.NewMetrics()}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderSummary(t *testing.T) {
	tests := []buildTestData{
		{
			name: "no-sum-and-count",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 5, nil, "foo", "bar", "quantile", "1"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				return []pmetric.Metrics{pmetric.NewMetrics()}
			},
		},
		{
			name: "no-count",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 1, nil, "foo", "bar", "quantile", "0.5"),
						createDataPoint("summary_test", 2, nil, "foo", "bar", "quantile", "0.75"),
						createDataPoint("summary_test", 5, nil, "foo", "bar", "quantile", "1"),
						createDataPoint("summary_test_sum", 500, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				return []pmetric.Metrics{pmetric.NewMetrics()}
			},
		},
		{
			name: "no-sum",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 1, nil, "foo", "bar", "quantile", "0.5"),
						createDataPoint("summary_test", 2, nil, "foo", "bar", "quantile", "0.75"),
						createDataPoint("summary_test", 5, nil, "foo", "bar", "quantile", "1"),
						createDataPoint("summary_test_count", 500, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("summary_test")
				sum0 := m0.SetEmptySummary()
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetTimestamp(tsNanos)
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetCount(500)
				pt0.SetSum(0.0)
				pt0.Attributes().PutStr("foo", "bar")
				qvL := pt0.QuantileValues()
				q50 := qvL.AppendEmpty()
				q50.SetQuantile(.50)
				q50.SetValue(1.0)
				q75 := qvL.AppendEmpty()
				q75.SetQuantile(.75)
				q75.SetValue(2.0)
				q100 := qvL.AppendEmpty()
				q100.SetQuantile(1)
				q100.SetValue(5.0)
				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "empty-quantiles",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test_sum", 100, nil, "foo", "bar"),
						createDataPoint("summary_test_count", 500, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("summary_test")
				sum0 := m0.SetEmptySummary()
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetTimestamp(tsNanos)
				pt0.SetCount(500)
				pt0.SetSum(100.0)
				pt0.Attributes().PutStr("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "regular-summary",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 1, nil, "foo", "bar", "quantile", "0.5"),
						createDataPoint("summary_test", 2, nil, "foo", "bar", "quantile", "0.75"),
						createDataPoint("summary_test", 5, nil, "foo", "bar", "quantile", "1"),
						createDataPoint("summary_test_sum", 100, nil, "foo", "bar"),
						createDataPoint("summary_test_count", 500, nil, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("summary_test")
				sum0 := m0.SetEmptySummary()
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(startTimestamp)
				pt0.SetTimestamp(tsNanos)
				pt0.SetCount(500)
				pt0.SetSum(100.0)
				pt0.Attributes().PutStr("foo", "bar")
				qvL := pt0.QuantileValues()
				q50 := qvL.AppendEmpty()
				q50.SetQuantile(.50)
				q50.SetValue(1.0)
				q75 := qvL.AppendEmpty()
				q75.SetQuantile(.75)
				q75.SetValue(2.0)
				q100 := qvL.AppendEmpty()
				q100.SetQuantile(1)
				q100.SetValue(5.0)

				return []pmetric.Metrics{md0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}

}

type buildTestData struct {
	name   string
	inputs []*testScrapedPage
	wants  func() []pmetric.Metrics
}

func (tt buildTestData) run(t *testing.T) {
	wants := tt.wants()
	assert.EqualValues(t, len(wants), len(tt.inputs))
	st := ts
	for i, page := range tt.inputs {
		sink := new(consumertest.MetricsSink)
		tr := newTransaction(scrapeCtx, &startTimeAdjuster{startTime: startTimestamp}, sink, nil, receivertest.NewNopCreateSettings(), nopObsRecv(t), false)
		for _, pt := range page.pts {
			// set ts for testing
			pt.t = st
			_, err := tr.Append(0, pt.lb, pt.t, pt.v)
			assert.NoError(t, err)

			for _, e := range pt.exemplars {
				_, err := tr.AppendExemplar(0, pt.lb, e)
				assert.NoError(t, err)
			}
		}
		assert.NoError(t, tr.Commit())
		mds := sink.AllMetrics()
		if wants[i].ResourceMetrics().Len() == 0 {
			// Receiver does not emit empty metrics, so will not have anything in the sink.
			require.Len(t, mds, 0)
			st += interval
			continue
		}
		require.Len(t, mds, 1)
		assertEquivalentMetrics(t, wants[i], mds[0])
		st += interval
	}
}

type errorAdjuster struct {
	err error
}

func (ea *errorAdjuster) AdjustMetrics(pmetric.Metrics) error {
	return ea.err
}

type startTimeAdjuster struct {
	startTime pcommon.Timestamp
}

func (s *startTimeAdjuster) AdjustMetrics(metrics pmetric.Metrics) error {
	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			ilm := rm.ScopeMetrics().At(j)
			for k := 0; k < ilm.Metrics().Len(); k++ {
				metric := ilm.Metrics().At(k)
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).SetStartTimestamp(s.startTime)
					}
				case pmetric.MetricTypeSummary:
					dps := metric.Summary().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).SetStartTimestamp(s.startTime)
					}
				case pmetric.MetricTypeHistogram:
					dps := metric.Histogram().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dps.At(l).SetStartTimestamp(s.startTime)
					}
				case pmetric.MetricTypeEmpty, pmetric.MetricTypeGauge, pmetric.MetricTypeExponentialHistogram:
				}
			}
		}
	}
	return nil
}

type testDataPoint struct {
	lb        labels.Labels
	t         int64
	v         float64
	exemplars []exemplar.Exemplar
}

type testScrapedPage struct {
	pts []*testDataPoint
}

func createDataPoint(mname string, value float64, es []exemplar.Exemplar, tagPairs ...string) *testDataPoint {
	var lbls []string
	lbls = append(lbls, tagPairs...)
	lbls = append(lbls, model.MetricNameLabel, mname)
	lbls = append(lbls, model.JobLabel, "job")
	lbls = append(lbls, model.InstanceLabel, "instance")

	return &testDataPoint{
		lb:        labels.FromStrings(lbls...),
		t:         ts,
		v:         value,
		exemplars: es,
	}
}

func assertEquivalentMetrics(t *testing.T, want, got pmetric.Metrics) {
	require.Equal(t, want.ResourceMetrics().Len(), got.ResourceMetrics().Len())
	if want.ResourceMetrics().Len() == 0 {
		return
	}
	for i := 0; i < want.ResourceMetrics().Len(); i++ {
		wantSm := want.ResourceMetrics().At(i).ScopeMetrics()
		gotSm := got.ResourceMetrics().At(i).ScopeMetrics()
		require.Equal(t, wantSm.Len(), gotSm.Len())
		if wantSm.Len() == 0 {
			return
		}

		for j := 0; j < wantSm.Len(); j++ {
			wantMs := wantSm.At(j).Metrics()
			gotMs := gotSm.At(j).Metrics()
			require.Equal(t, wantMs.Len(), gotMs.Len())

			wmap := map[string]pmetric.Metric{}
			gmap := map[string]pmetric.Metric{}

			for k := 0; k < wantMs.Len(); k++ {
				wi := wantMs.At(k)
				wmap[wi.Name()] = wi
				gi := gotMs.At(k)
				gmap[gi.Name()] = gi
			}
			assert.EqualValues(t, wmap, gmap)
		}
	}

}

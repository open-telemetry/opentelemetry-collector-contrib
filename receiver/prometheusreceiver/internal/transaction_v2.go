// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	mdata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

// transactionV2 builds pmetric.Metrics directly during Append() calls.

type transactionV2 struct {
	isNew          bool
	trimSuffixes   bool
	ctx            context.Context
	md             pmetric.Metrics
	sink           consumer.Metrics
	externalLabels labels.Labels
	logger         *zap.Logger
	buildInfo      component.BuildInfo
	obsrecv        *receiverhelper.ObsReport

	// Indices for efficient lookup during Append()
	resourceIndex map[resourceKey]pmetric.ResourceMetrics
	scopeIndex    map[resourceKey]map[scopeID]pmetric.ScopeMetrics
	metricIndex   map[resourceKey]map[scopeID]map[metricFamilyKey]pmetric.Metric

	// Store scope attributes from otel_scope_info
	scopeAttributes map[resourceKey]map[scopeID]pcommon.Map
}

func newTransactionV2(
	ctx context.Context,
	sink consumer.Metrics,
	externalLabels labels.Labels,
	settings receiver.Settings,
	obsrecv *receiverhelper.ObsReport,
	trimSuffixes bool,
) *transactionV2 {
	return &transactionV2{
		ctx:             ctx,
		md:              pmetric.NewMetrics(),
		isNew:           true,
		trimSuffixes:    trimSuffixes,
		sink:            sink,
		externalLabels:  externalLabels,
		logger:          settings.Logger,
		buildInfo:       settings.BuildInfo,
		obsrecv:         obsrecv,
		resourceIndex:   make(map[resourceKey]pmetric.ResourceMetrics),
		scopeIndex:      make(map[resourceKey]map[scopeID]pmetric.ScopeMetrics),
		metricIndex:     make(map[resourceKey]map[scopeID]map[metricFamilyKey]pmetric.Metric),
		scopeAttributes: make(map[resourceKey]map[scopeID]pcommon.Map),
	}
}

func (t *transactionV2) Append(
	_ storage.SeriesRef,
	ls labels.Labels,
	st, ts int64,
	v float64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
	opts storage.AppendV2Options,
) (storage.SeriesRef, error) {
	select {
	case <-t.ctx.Done():
		return 0, errTransactionAborted
	default:
	}

	// Apply external labels
	if t.externalLabels.Len() != 0 {
		b := labels.NewBuilder(ls)
		t.externalLabels.Range(func(l labels.Label) {
			b.Set(l.Name, l.Value)
		})
		ls = b.Labels()
	}

	// Get job/instance for resource key
	rKey, err := t.initTransaction(ls)
	if err != nil {
		return 0, err
	}

	// Reject duplicate labels
	if dupLabel, hasDup := ls.HasDuplicateLabelNames(); hasDup {
		return 0, fmt.Errorf("invalid sample: non-unique label names: %q", dupLabel)
	}

	metricName := ls.Get(model.MetricNameLabel)
	if metricName == "" {
		return 0, errMetricNameNotFound
	}

	// Handle special metrics that add attributes
	if metricName == prometheus.TargetInfoMetricName {
		t.addTargetInfo(*rKey, ls)
		return 0, nil
	}
	if metricName == prometheus.ScopeInfoMetricName {
		t.addScopeInfo(*rKey, ls)
		return 0, nil
	}

	// Determine metric type
	isNativeHist := h != nil || fh != nil
	isNHCB := (h != nil && h.Schema == -53) || (fh != nil && fh.Schema == -53)
	mtype, isMonotonic := convToMetricType(opts.Metadata.Type, isNativeHist && !isNHCB)

	// Get or create the metric and add the data point directly
	scope := getScopeID(ls)
	metric := t.getOrCreateMetric(*rKey, scope, metricName, mtype, isMonotonic, opts, isNativeHist, isNHCB)

	// Add data point directly to the metric
	t.addDataPoint(metric, mtype, ls, st, ts, v, h, fh, opts)

	t.isNew = false
	return storage.SeriesRef(ls.Hash()), nil
}

func (t *transactionV2) initTransaction(ls labels.Labels) (*resourceKey, error) {
	target, ok := scrape.TargetFromContext(t.ctx)
	if !ok {
		return nil, errors.New("unable to find target in context")
	}

	rKey, err := getJobAndInstance(t.ctx, ls)
	if err != nil {
		return nil, err
	}

	// Create resource if not exists
	if _, ok := t.resourceIndex[*rKey]; !ok {
		rm := t.md.ResourceMetrics().AppendEmpty()
		resource := CreateResource(rKey.job, rKey.instance, target.DiscoveredLabels(labels.NewBuilder(labels.EmptyLabels())))
		resource.CopyTo(rm.Resource())
		t.resourceIndex[*rKey] = rm
		t.scopeIndex[*rKey] = make(map[scopeID]pmetric.ScopeMetrics)
		t.metricIndex[*rKey] = make(map[scopeID]map[metricFamilyKey]pmetric.Metric)
	}

	return rKey, nil
}

func (t *transactionV2) getOrCreateMetric(
	rKey resourceKey,
	scope scopeID,
	metricName string,
	mtype pmetric.MetricType,
	isMonotonic bool,
	opts storage.AppendV2Options,
	isNativeHist bool,
	isNHCB bool,
) pmetric.Metric {
	// Get or create scope
	if _, ok := t.scopeIndex[rKey][scope]; !ok {
		rm := t.resourceIndex[rKey]
		sm := rm.ScopeMetrics().AppendEmpty()
		if scope == emptyScopeID {
			sm.Scope().SetName(mdata.ScopeName)
			sm.Scope().SetVersion(t.buildInfo.Version)
		} else {
			sm.Scope().SetName(scope.name)
			sm.Scope().SetVersion(scope.version)
			if scope.schemaURL != "" {
				sm.SetSchemaUrl(scope.schemaURL)
			}
		}
		// Apply scope attributes if found in cache.
		if attrs, ok := t.scopeAttributes[rKey][scope]; ok {
			attrs.CopyTo(sm.Scope().Attributes())
		}
		t.scopeIndex[rKey][scope] = sm
		t.metricIndex[rKey][scope] = make(map[metricFamilyKey]pmetric.Metric)
	}

	// Get or create metric
	mfKey := metricFamilyKey{isExponentialHistogram: isNativeHist && !isNHCB, name: metricName}
	if metric, ok := t.metricIndex[rKey][scope][mfKey]; ok {
		return metric
	}

	sm := t.scopeIndex[rKey][scope]
	metric := sm.Metrics().AppendEmpty()
	metric.SetName(metricName)
	metric.SetDescription(opts.Metadata.Help)
	metric.SetUnit(prometheus.UnitWordToUCUM(opts.Metadata.Unit))
	metric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, string(opts.Metadata.Type))

	// Initialize the metric type
	switch mtype {
	case pmetric.MetricTypeGauge:
		metric.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		sum := metric.SetEmptySum()
		sum.SetIsMonotonic(isMonotonic)
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	case pmetric.MetricTypeHistogram:
		hist := metric.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	case pmetric.MetricTypeExponentialHistogram:
		expHist := metric.SetEmptyExponentialHistogram()
		expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	case pmetric.MetricTypeSummary:
		metric.SetEmptySummary()
	}

	t.metricIndex[rKey][scope][mfKey] = metric
	return metric
}

func (t *transactionV2) addDataPoint(
	metric pmetric.Metric,
	mtype pmetric.MetricType,
	ls labels.Labels,
	st, ts int64,
	v float64,
	h *histogram.Histogram,
	fh *histogram.FloatHistogram,
	opts storage.AppendV2Options,
) {
	switch mtype {
	case pmetric.MetricTypeGauge:
		t.addGaugeDataPoint(metric.Gauge().DataPoints(), ls, ts, v, opts)
	case pmetric.MetricTypeSum:
		t.addSumDataPoint(metric.Sum().DataPoints(), ls, st, ts, v, opts)
	case pmetric.MetricTypeHistogram:
		t.addHistogramDataPoint(metric.Histogram().DataPoints(), ls, st, ts, h, fh, opts)
	case pmetric.MetricTypeExponentialHistogram:
		t.addExponentialHistogramDataPoint(metric.ExponentialHistogram().DataPoints(), ls, st, ts, h, fh, opts)
	case pmetric.MetricTypeSummary:
		t.addSummaryDataPoint(metric.Summary().DataPoints(), ls, st, ts, v, opts)
	}
}

func (*transactionV2) addGaugeDataPoint(dps pmetric.NumberDataPointSlice, ls labels.Labels, ts int64, v float64, opts storage.AppendV2Options) {
	dp := dps.AppendEmpty()
	dp.SetTimestamp(timestampFromMs(ts))
	if value.IsStaleNaN(v) {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetDoubleValue(v)
	populateAttributes(pmetric.MetricTypeGauge, ls, dp.Attributes())
	for _, e := range opts.Exemplars {
		convertExemplar(e, dp.Exemplars().AppendEmpty())
	}
}

func (*transactionV2) addSumDataPoint(dps pmetric.NumberDataPointSlice, ls labels.Labels, st, ts int64, v float64, opts storage.AppendV2Options) {
	dp := dps.AppendEmpty()
	dp.SetTimestamp(timestampFromMs(ts))
	if st > 0 {
		dp.SetStartTimestamp(timestampFromMs(st))
	}
	if value.IsStaleNaN(v) {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	}
	dp.SetDoubleValue(v)
	populateAttributes(pmetric.MetricTypeSum, ls, dp.Attributes())
	for _, e := range opts.Exemplars {
		convertExemplar(e, dp.Exemplars().AppendEmpty())
	}
}

func (*transactionV2) addHistogramDataPoint(dps pmetric.HistogramDataPointSlice, ls labels.Labels, st, ts int64, h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options) {
	dp := dps.AppendEmpty()
	dp.SetTimestamp(timestampFromMs(ts))
	if st > 0 {
		dp.SetStartTimestamp(timestampFromMs(st))
	}

	switch {
	case h != nil:
		if len(h.CustomValues) == 0 {
			return
		}
		bounds := make([]float64, len(h.CustomValues))
		copy(bounds, h.CustomValues)
		dp.ExplicitBounds().FromRaw(bounds)
		dp.BucketCounts().FromRaw(convertNHCBBDeltBuckets(h))
		dp.SetCount(h.Count)
		dp.SetSum(h.Sum)
	case fh != nil:
		if len(fh.CustomValues) == 0 {
			return
		}
		bounds := make([]float64, len(fh.CustomValues))
		copy(bounds, fh.CustomValues)
		dp.ExplicitBounds().FromRaw(bounds)
		dp.BucketCounts().FromRaw(convertNHCBAbsoluteBuckets(fh))
		dp.SetCount(uint64(fh.Count))
		dp.SetSum(fh.Sum)
	}

	populateAttributes(pmetric.MetricTypeHistogram, ls, dp.Attributes())
	for _, e := range opts.Exemplars {
		convertExemplar(e, dp.Exemplars().AppendEmpty())
	}
}

func (*transactionV2) addExponentialHistogramDataPoint(dps pmetric.ExponentialHistogramDataPointSlice, ls labels.Labels, st, ts int64, h *histogram.Histogram, fh *histogram.FloatHistogram, opts storage.AppendV2Options) {
	dp := dps.AppendEmpty()
	dp.SetTimestamp(timestampFromMs(ts))
	if st > 0 {
		dp.SetStartTimestamp(timestampFromMs(st))
	}

	switch {
	case h != nil:
		if value.IsStaleNaN(h.Sum) {
			dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
		} else {
			dp.SetScale(h.Schema)
			dp.SetCount(h.Count)
			dp.SetSum(h.Sum)
			dp.SetZeroThreshold(h.ZeroThreshold)
			dp.SetZeroCount(h.ZeroCount)
			if len(h.PositiveSpans) > 0 {
				dp.Positive().SetOffset(h.PositiveSpans[0].Offset - 1)
				convertDeltaBuckets(h.PositiveSpans, h.PositiveBuckets, dp.Positive().BucketCounts())
			}
			if len(h.NegativeSpans) > 0 {
				dp.Negative().SetOffset(h.NegativeSpans[0].Offset - 1)
				convertDeltaBuckets(h.NegativeSpans, h.NegativeBuckets, dp.Negative().BucketCounts())
			}
		}
	case fh != nil:
		if value.IsStaleNaN(fh.Sum) {
			dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
		} else {
			dp.SetScale(fh.Schema)
			dp.SetCount(uint64(fh.Count))
			dp.SetSum(fh.Sum)
			dp.SetZeroThreshold(fh.ZeroThreshold)
			dp.SetZeroCount(uint64(fh.ZeroCount))
			if len(fh.PositiveSpans) > 0 {
				dp.Positive().SetOffset(fh.PositiveSpans[0].Offset - 1)
				convertAbsoluteBuckets(fh.PositiveSpans, fh.PositiveBuckets, dp.Positive().BucketCounts())
			}
			if len(fh.NegativeSpans) > 0 {
				dp.Negative().SetOffset(fh.NegativeSpans[0].Offset - 1)
				convertAbsoluteBuckets(fh.NegativeSpans, fh.NegativeBuckets, dp.Negative().BucketCounts())
			}
		}
	}

	populateAttributes(pmetric.MetricTypeExponentialHistogram, ls, dp.Attributes())
	for _, e := range opts.Exemplars {
		convertExemplar(e, dp.Exemplars().AppendEmpty())
	}
}

func (*transactionV2) addSummaryDataPoint(dps pmetric.SummaryDataPointSlice, ls labels.Labels, st, ts int64, v float64, _ storage.AppendV2Options) {
	dp := dps.AppendEmpty()
	dp.SetTimestamp(timestampFromMs(ts))
	if st > 0 {
		dp.SetStartTimestamp(timestampFromMs(st))
	}
	dp.SetSum(v)
	populateAttributes(pmetric.MetricTypeSummary, ls, dp.Attributes())
}

func (t *transactionV2) addTargetInfo(rKey resourceKey, ls labels.Labels) {
	// Resource already exists
	rm := t.resourceIndex[rKey]
	ls.Range(func(lbl labels.Label) {
		if lbl.Name == model.JobLabel || lbl.Name == model.InstanceLabel || lbl.Name == model.MetricNameLabel {
			return
		}
		rm.Resource().Attributes().PutStr(lbl.Name, lbl.Value)
	})
}

func (t *transactionV2) addScopeInfo(rKey resourceKey, ls labels.Labels) {
	scope := scopeID{}
	attrs := pcommon.NewMap()
	ls.Range(func(lbl labels.Label) {
		if lbl.Name == model.JobLabel || lbl.Name == model.InstanceLabel || lbl.Name == model.MetricNameLabel {
			return
		}
		if lbl.Name == prometheus.ScopeNameLabelKey {
			scope.name = lbl.Value
			return
		}
		if lbl.Name == prometheus.ScopeVersionLabelKey {
			scope.version = lbl.Value
			return
		}
		if lbl.Name == prometheus.ScopeSchemaURLLabelKey {
			scope.schemaURL = lbl.Value
			return
		}
		attrs.PutStr(lbl.Name, lbl.Value)
	})

	// If scope already exists, apply attributes directly
	if sm, ok := t.scopeIndex[rKey][scope]; ok {
		attrs.CopyTo(sm.Scope().Attributes())
		return
	}

	// Otherwise store for when scope is created
	if _, ok := t.scopeAttributes[rKey]; !ok {
		t.scopeAttributes[rKey] = make(map[scopeID]pcommon.Map)
	}
	t.scopeAttributes[rKey][scope] = attrs
}

func (t *transactionV2) Commit() error {
	if t.isNew {
		return nil
	}

	// Remove empty resources/scopes
	t.md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(sm pmetric.ScopeMetrics) bool {
			return sm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})

	if t.md.DataPointCount() == 0 {
		return errNoDataToBuild
	}

	ctx := t.obsrecv.StartMetricsOp(t.ctx)
	numPoints := t.md.DataPointCount()
	err := t.sink.ConsumeMetrics(ctx, t.md)
	t.obsrecv.EndMetricsOp(ctx, dataformat, numPoints, err)
	return err
}

func (*transactionV2) Rollback() error {
	// Rollback is a no-op in the receiver since we don't buffer to external storage
	return nil
}

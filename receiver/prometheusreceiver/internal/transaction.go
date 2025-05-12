// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	mdata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

var removeStartTimeAdjustment = featuregate.GlobalRegistry().MustRegister(
	"receiver.prometheusreceiver.RemoveStartTimeAdjustment",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the Prometheus receiver will"+
		" leave the start time unset. Use the new metricstarttime processor instead."),
)

type resourceKey struct {
	job      string
	instance string
}

// The name of the metric family doesn't include magic suffixes (e.g. _bucket),
// so for a classic histgram and a native histogram of the same family, the
// metric family will be the same. To be able to tell them apart, we need to
// store whether the metric is a native histogram or not.
type metricFamilyKey struct {
	isExponentialHistogram bool
	name                   string
}

type transaction struct {
	isNew                  bool
	trimSuffixes           bool
	enableNativeHistograms bool
	addingNativeHistogram  bool // true if the last sample was a native histogram.
	ctx                    context.Context
	families               map[resourceKey]map[scopeID]map[metricFamilyKey]*metricFamily
	mc                     scrape.MetricMetadataStore
	sink                   consumer.Metrics
	externalLabels         labels.Labels
	nodeResources          map[resourceKey]pcommon.Resource
	scopeAttributes        map[resourceKey]map[scopeID]pcommon.Map
	logger                 *zap.Logger
	buildInfo              component.BuildInfo
	metricAdjuster         MetricsAdjuster
	obsrecv                *receiverhelper.ObsReport
	// Used as buffer to calculate series ref hash.
	bufBytes []byte
}

var emptyScopeID scopeID

type scopeID struct {
	name    string
	version string
}

func newTransaction(
	ctx context.Context,
	metricAdjuster MetricsAdjuster,
	sink consumer.Metrics,
	externalLabels labels.Labels,
	settings receiver.Settings,
	obsrecv *receiverhelper.ObsReport,
	trimSuffixes bool,
	enableNativeHistograms bool,
) *transaction {
	return &transaction{
		ctx:                    ctx,
		families:               make(map[resourceKey]map[scopeID]map[metricFamilyKey]*metricFamily),
		isNew:                  true,
		trimSuffixes:           trimSuffixes,
		enableNativeHistograms: enableNativeHistograms,
		sink:                   sink,
		metricAdjuster:         metricAdjuster,
		externalLabels:         externalLabels,
		logger:                 settings.Logger,
		buildInfo:              settings.BuildInfo,
		obsrecv:                obsrecv,
		bufBytes:               make([]byte, 0, 1024),
		scopeAttributes:        make(map[resourceKey]map[scopeID]pcommon.Map),
		nodeResources:          map[resourceKey]pcommon.Resource{},
	}
}

// Append always returns 0 to disable label caching.
func (t *transaction) Append(_ storage.SeriesRef, ls labels.Labels, atMs int64, val float64) (storage.SeriesRef, error) {
	t.addingNativeHistogram = false

	select {
	case <-t.ctx.Done():
		return 0, errTransactionAborted
	default:
	}

	if t.externalLabels.Len() != 0 {
		b := labels.NewBuilder(ls)
		t.externalLabels.Range(func(l labels.Label) {
			b.Set(l.Name, l.Value)
		})
		ls = b.Labels()
	}

	rKey, err := t.initTransaction(ls)
	if err != nil {
		return 0, err
	}

	// Any datapoint with duplicate labels MUST be rejected per:
	// * https://github.com/open-telemetry/wg-prometheus/issues/44
	// * https://github.com/open-telemetry/opentelemetry-collector/issues/3407
	// as Prometheus rejects such too as of version 2.16.0, released on 2020-02-13.
	if dupLabel, hasDup := ls.HasDuplicateLabelNames(); hasDup {
		return 0, fmt.Errorf("invalid sample: non-unique label names: %q", dupLabel)
	}

	metricName := ls.Get(model.MetricNameLabel)
	if metricName == "" {
		return 0, errMetricNameNotFound
	}

	// See https://www.prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series
	// up: 1 if the instance is healthy, i.e. reachable, or 0 if the scrape failed.
	// But it can also be a staleNaN, which is inserted when the target goes away.
	if metricName == scrapeUpMetricName && val != 1.0 && !value.IsStaleNaN(val) {
		if val == 0.0 {
			t.logger.Warn("Failed to scrape Prometheus endpoint",
				zap.Int64("scrape_timestamp", atMs),
				zap.Stringer("target_labels", ls))
		} else {
			t.logger.Warn("The 'up' metric contains invalid value",
				zap.Float64("value", val),
				zap.Int64("scrape_timestamp", atMs),
				zap.Stringer("target_labels", ls))
		}
	}

	// For the `target_info` metric we need to convert it to resource attributes.
	if metricName == prometheus.TargetInfoMetricName {
		t.AddTargetInfo(*rKey, ls)
		return 0, nil
	}

	// For the `otel_scope_info` metric we need to convert it to scope attributes.
	if metricName == prometheus.ScopeInfoMetricName {
		t.addScopeInfo(*rKey, ls)
		return 0, nil
	}

	scope := getScopeID(ls)

	if t.enableNativeHistograms && value.IsStaleNaN(val) {
		if t.detectAndStoreNativeHistogramStaleness(atMs, rKey, scope, metricName, ls) {
			return 0, nil
		}
	}

	curMF := t.getOrCreateMetricFamily(*rKey, scope, metricName)

	seriesRef := t.getSeriesRef(ls, curMF.mtype)
	err = curMF.addSeries(seriesRef, metricName, ls, atMs, val)
	if err != nil {
		t.logger.Warn("failed to add datapoint", zap.Error(err), zap.String("metric_name", metricName), zap.Any("labels", ls))
	}

	return 0, nil // never return errors, as that fails the whole scrape
}

// detectAndStoreNativeHistogramStaleness returns true if it detects
// and stores a native histogram staleness marker.
func (t *transaction) detectAndStoreNativeHistogramStaleness(atMs int64, key *resourceKey, scope scopeID, metricName string, ls labels.Labels) bool {
	// Detect the special case of stale native histogram series.
	// Currently Prometheus does not store the histogram type in
	// its staleness tracker.
	md, ok := t.mc.GetMetadata(metricName)
	if !ok {
		// Native histograms always have metadata.
		return false
	}
	if md.Type != model.MetricTypeHistogram {
		// Not a histogram.
		return false
	}
	if md.Metric != metricName {
		// Not a native histogram because it has magic suffixes (e.g. _bucket).
		return false
	}
	// Store the staleness marker as a native histogram.
	t.addingNativeHistogram = true

	curMF := t.getOrCreateMetricFamily(*key, scope, metricName)
	seriesRef := t.getSeriesRef(ls, curMF.mtype)

	_ = curMF.addExponentialHistogramSeries(seriesRef, metricName, ls, atMs, &histogram.Histogram{Sum: math.Float64frombits(value.StaleNaN)}, nil)
	// ignore errors here, this is best effort.

	return true
}

// getOrCreateMetricFamily returns the metric family for the given metric name and scope,
// and true if an existing family was found.
func (t *transaction) getOrCreateMetricFamily(key resourceKey, scope scopeID, mn string) *metricFamily {
	if _, ok := t.families[key]; !ok {
		t.families[key] = make(map[scopeID]map[metricFamilyKey]*metricFamily)
	}
	if _, ok := t.families[key][scope]; !ok {
		t.families[key][scope] = make(map[metricFamilyKey]*metricFamily)
	}

	mfKey := metricFamilyKey{isExponentialHistogram: t.addingNativeHistogram, name: mn}

	curMf, ok := t.families[key][scope][mfKey]

	if !ok {
		fn := mn
		if _, ok := t.mc.GetMetadata(mn); !ok {
			fn = normalizeMetricName(mn)
		}
		fnKey := metricFamilyKey{isExponentialHistogram: mfKey.isExponentialHistogram, name: fn}
		mf, ok := t.families[key][scope][fnKey]
		if !ok || !mf.includesMetric(mn) {
			curMf = newMetricFamily(mn, t.mc, t.logger)
			if curMf.mtype == pmetric.MetricTypeHistogram && mfKey.isExponentialHistogram {
				curMf.mtype = pmetric.MetricTypeExponentialHistogram
			}
			t.families[key][scope][metricFamilyKey{isExponentialHistogram: mfKey.isExponentialHistogram, name: curMf.name}] = curMf
			return curMf
		}
		curMf = mf
	}
	return curMf
}

func (t *transaction) AppendExemplar(_ storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	select {
	case <-t.ctx.Done():
		return 0, errTransactionAborted
	default:
	}

	rKey, err := t.initTransaction(l)
	if err != nil {
		return 0, err
	}

	l = l.WithoutEmpty()

	if dupLabel, hasDup := l.HasDuplicateLabelNames(); hasDup {
		return 0, fmt.Errorf("invalid sample: non-unique label names: %q", dupLabel)
	}

	mn := l.Get(model.MetricNameLabel)
	if mn == "" {
		return 0, errMetricNameNotFound
	}

	mf := t.getOrCreateMetricFamily(*rKey, getScopeID(l), mn)

	// Workaround for https://github.com/prometheus/prometheus/issues/16217
	if !t.enableNativeHistograms && mf.mtype == pmetric.MetricTypeHistogram && l.Get(model.MetricNameLabel) == mf.name {
		return 0, nil
	}

	mf.addExemplar(t.getSeriesRef(l, mf.mtype), e)

	return 0, nil
}

func (t *transaction) AppendHistogram(_ storage.SeriesRef, ls labels.Labels, atMs int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	if !t.enableNativeHistograms {
		return 0, nil
	}

	select {
	case <-t.ctx.Done():
		return 0, errTransactionAborted
	default:
	}

	t.addingNativeHistogram = true

	if t.externalLabels.Len() != 0 {
		b := labels.NewBuilder(ls)
		t.externalLabels.Range(func(l labels.Label) {
			b.Set(l.Name, l.Value)
		})
		ls = b.Labels()
	}

	rKey, err := t.initTransaction(ls)
	if err != nil {
		return 0, err
	}

	// Any datapoint with duplicate labels MUST be rejected per:
	// * https://github.com/open-telemetry/wg-prometheus/issues/44
	// * https://github.com/open-telemetry/opentelemetry-collector/issues/3407
	// as Prometheus rejects such too as of version 2.16.0, released on 2020-02-13.
	if dupLabel, hasDup := ls.HasDuplicateLabelNames(); hasDup {
		return 0, fmt.Errorf("invalid sample: non-unique label names: %q", dupLabel)
	}

	metricName := ls.Get(model.MetricNameLabel)
	if metricName == "" {
		return 0, errMetricNameNotFound
	}

	// The `up`, `target_info`, `otel_scope_info` metrics should never generate native histograms,
	// thus we don't check for them here as opposed to the Append function.

	curMF := t.getOrCreateMetricFamily(*rKey, getScopeID(ls), metricName)

	if h != nil && h.CounterResetHint == histogram.GaugeType || fh != nil && fh.CounterResetHint == histogram.GaugeType {
		t.logger.Warn("dropping unsupported gauge histogram datapoint", zap.String("metric_name", metricName), zap.Any("labels", ls))
	}

	err = curMF.addExponentialHistogramSeries(t.getSeriesRef(ls, curMF.mtype), metricName, ls, atMs, h, fh)
	if err != nil {
		t.logger.Warn("failed to add histogram datapoint", zap.Error(err), zap.String("metric_name", metricName), zap.Any("labels", ls))
	}

	return 0, nil // never return errors, as that fails the whole scrape
}

func (t *transaction) AppendCTZeroSample(_ storage.SeriesRef, ls labels.Labels, atMs, ctMs int64) (storage.SeriesRef, error) {
	t.addingNativeHistogram = false
	return t.setCreationTimestamp(ls, atMs, ctMs)
}

func (t *transaction) AppendHistogramCTZeroSample(_ storage.SeriesRef, ls labels.Labels, atMs, ctMs int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	t.addingNativeHistogram = true
	return t.setCreationTimestamp(ls, atMs, ctMs)
}

func (t *transaction) setCreationTimestamp(ls labels.Labels, atMs, ctMs int64) (storage.SeriesRef, error) {
	select {
	case <-t.ctx.Done():
		return 0, errTransactionAborted
	default:
	}

	if t.externalLabels.Len() != 0 {
		b := labels.NewBuilder(ls)
		t.externalLabels.Range(func(l labels.Label) {
			b.Set(l.Name, l.Value)
		})
		ls = b.Labels()
	}

	rKey, err := t.initTransaction(ls)
	if err != nil {
		return 0, err
	}

	// Any datapoint with duplicate labels MUST be rejected per:
	// * https://github.com/open-telemetry/wg-prometheus/issues/44
	// * https://github.com/open-telemetry/opentelemetry-collector/issues/3407
	// as Prometheus rejects such too as of version 2.16.0, released on 2020-02-13.
	if dupLabel, hasDup := ls.HasDuplicateLabelNames(); hasDup {
		return 0, fmt.Errorf("invalid sample: non-unique label names: %q", dupLabel)
	}

	metricName := ls.Get(model.MetricNameLabel)
	if metricName == "" {
		return 0, errMetricNameNotFound
	}

	curMF := t.getOrCreateMetricFamily(*rKey, getScopeID(ls), metricName)

	seriesRef := t.getSeriesRef(ls, curMF.mtype)
	curMF.addCreationTimestamp(seriesRef, ls, atMs, ctMs)

	return storage.SeriesRef(seriesRef), nil
}

func (t *transaction) SetOptions(_ *storage.AppendOptions) {
	// TODO: implement this func
}

func (t *transaction) getSeriesRef(ls labels.Labels, mtype pmetric.MetricType) uint64 {
	var hash uint64
	hash, t.bufBytes = getSeriesRef(t.bufBytes, ls, mtype)
	return hash
}

// getMetrics returns all metrics to the given slice.
// The only error returned by this function is errNoDataToBuild.
func (t *transaction) getMetrics() (pmetric.Metrics, error) {
	if len(t.families) == 0 {
		return pmetric.Metrics{}, errNoDataToBuild
	}

	md := pmetric.NewMetrics()

	for rKey, families := range t.families {
		if len(families) == 0 {
			continue
		}
		resource, ok := t.nodeResources[rKey]
		if !ok {
			continue
		}
		rms := md.ResourceMetrics().AppendEmpty()
		resource.CopyTo(rms.Resource())

		for scope, mfs := range families {
			ils := rms.ScopeMetrics().AppendEmpty()
			// If metrics don't include otel_scope_name or otel_scope_version
			// labels, use the receiver name and version.
			if scope == emptyScopeID {
				ils.Scope().SetName(mdata.ScopeName)
				ils.Scope().SetVersion(t.buildInfo.Version)
			} else {
				// Otherwise, use the scope that was provided with the metrics.
				ils.Scope().SetName(scope.name)
				ils.Scope().SetVersion(scope.version)
				// If we got an otel_scope_info metric for that scope, get scope
				// attributes from it.
				if scopeAttributes, ok := t.scopeAttributes[rKey]; ok {
					if attributes, ok := scopeAttributes[scope]; ok {
						attributes.CopyTo(ils.Scope().Attributes())
					}
				}
			}
			metrics := ils.Metrics()
			for _, mf := range mfs {
				mf.appendMetric(metrics, t.trimSuffixes)
			}
		}
	}
	// remove the resource if no metrics were added to avoid returning resources with empty data points
	md.ResourceMetrics().RemoveIf(func(metrics pmetric.ResourceMetrics) bool {
		if metrics.ScopeMetrics().Len() == 0 {
			return true
		}
		remove := true
		for i := 0; i < metrics.ScopeMetrics().Len(); i++ {
			if metrics.ScopeMetrics().At(i).Metrics().Len() > 0 {
				remove = false
				break
			}
		}
		return remove
	})

	return md, nil
}

func getScopeID(ls labels.Labels) scopeID {
	var scope scopeID
	ls.Range(func(lbl labels.Label) {
		if lbl.Name == prometheus.ScopeNameLabelKey {
			scope.name = lbl.Value
		}
		if lbl.Name == prometheus.ScopeVersionLabelKey {
			scope.version = lbl.Value
		}
	})
	return scope
}

func (t *transaction) initTransaction(labels labels.Labels) (*resourceKey, error) {
	target, ok := scrape.TargetFromContext(t.ctx)
	if !ok {
		return nil, errors.New("unable to find target in context")
	}
	t.mc, ok = scrape.MetricMetadataStoreFromContext(t.ctx)
	if !ok {
		return nil, errors.New("unable to find MetricMetadataStore in context")
	}

	rKey, err := t.getJobAndInstance(labels)
	if err != nil {
		return nil, err
	}
	if _, ok := t.nodeResources[*rKey]; !ok {
		t.nodeResources[*rKey] = CreateResource(rKey.job, rKey.instance, target.DiscoveredLabels())
	}

	t.isNew = false
	return rKey, nil
}

func (t *transaction) getJobAndInstance(labels labels.Labels) (*resourceKey, error) {
	// first, try to get job and instance from the labels
	job, instance := labels.Get(model.JobLabel), labels.Get(model.InstanceLabel)
	if job != "" && instance != "" {
		return &resourceKey{
			job:      job,
			instance: instance,
		}, nil
	}

	// if not available in the labels, try to fall back to the scrape job associated
	// with the transaction.
	// this can be the case for, e.g., aggregated metrics coming from a federate endpoint
	// that represent the whole cluster, rather than an individual workload.
	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32555 for reference
	if target, ok := scrape.TargetFromContext(t.ctx); ok {
		if job == "" {
			job = target.GetValue(model.JobLabel)
		}
		if instance == "" {
			instance = target.GetValue(model.InstanceLabel)
		}
		if job != "" && instance != "" {
			return &resourceKey{
				job:      job,
				instance: instance,
			}, nil
		}
	}
	return nil, errNoJobInstance
}

func (t *transaction) Commit() error {
	if t.isNew {
		return nil
	}

	ctx := t.obsrecv.StartMetricsOp(t.ctx)
	md, err := t.getMetrics()
	if err != nil {
		t.obsrecv.EndMetricsOp(ctx, dataformat, 0, err)
		return err
	}

	numPoints := md.DataPointCount()
	if numPoints == 0 {
		return nil
	}

	if !removeStartTimeAdjustment.IsEnabled() {
		if err = t.metricAdjuster.AdjustMetrics(md); err != nil {
			t.obsrecv.EndMetricsOp(ctx, dataformat, numPoints, err)
			return err
		}
	}

	err = t.sink.ConsumeMetrics(ctx, md)
	t.obsrecv.EndMetricsOp(ctx, dataformat, numPoints, err)
	return err
}

func (t *transaction) Rollback() error {
	return nil
}

func (t *transaction) UpdateMetadata(_ storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	// TODO: implement this func
	return 0, nil
}

func (t *transaction) AddTargetInfo(key resourceKey, ls labels.Labels) {
	t.addingNativeHistogram = false
	if resource, ok := t.nodeResources[key]; ok {
		attrs := resource.Attributes()
		ls.Range(func(lbl labels.Label) {
			if lbl.Name == model.JobLabel || lbl.Name == model.InstanceLabel || lbl.Name == model.MetricNameLabel {
				return
			}
			attrs.PutStr(lbl.Name, lbl.Value)
		})
	}
}

func (t *transaction) addScopeInfo(key resourceKey, ls labels.Labels) {
	t.addingNativeHistogram = false
	attrs := pcommon.NewMap()
	scope := scopeID{}
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
		attrs.PutStr(lbl.Name, lbl.Value)
	})
	if _, ok := t.scopeAttributes[key]; !ok {
		t.scopeAttributes[key] = make(map[scopeID]pcommon.Map)
	}
	t.scopeAttributes[key][scope] = attrs
}

func getSeriesRef(bytes []byte, ls labels.Labels, mtype pmetric.MetricType) (uint64, []byte) {
	return ls.HashWithoutLabels(bytes, getSortedNotUsefulLabels(mtype)...)
}

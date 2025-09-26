// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

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

var RemoveScopeInfoGate = featuregate.GlobalRegistry().MustRegister(
	"receiver.prometheusreceiver.RemoveScopeInfo",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Controls the processing of 'otel_scope_info' metrics. "+
		"The receiver always extracts scope attributes from 'otel_scope_' prefixed labels on all metrics. "+
		"When this feature gate is disabled (legacy mode), 'otel_scope_info' metrics are also processed "+
		"for scope attributes and merged with any 'otel_scope_' labels (with 'otel_scope_' taking precedence). "+
		"When enabled, 'otel_scope_info' metrics are treated as regular metrics and not processed for scope attributes."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-specification/pull/4505"),
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
	removeScopeInfo        bool
	ctx                    context.Context
	families               map[resourceKey]map[string]map[metricFamilyKey]*metricFamily
	mc                     scrape.MetricMetadataStore
	sink                   consumer.Metrics
	externalLabels         labels.Labels
	nodeResources          map[resourceKey]pcommon.Resource
	scopeMap               map[resourceKey]map[string]ScopeIdentifier
	scopeAttributes        map[resourceKey]map[LegacyScopeID]pcommon.Map // Legacy mode only
	logger                 *zap.Logger
	buildInfo              component.BuildInfo
	metricAdjuster         MetricsAdjuster
	obsrecv                *receiverhelper.ObsReport
	// Used as buffer to calculate series ref hash.
	bufBytes []byte
}

// ScopeIdentifier represents an identifier for a metric scope, which can include
// just the basic scope information or also include scope attributes.
type ScopeIdentifier interface {
	Key() string
	Name() string
	Version() string
	SchemaURL() string
	Attributes() pcommon.Map
	IsEmpty() bool
}

// LegacyScopeID represents a scope identified only by name, version, and schema URL.
// This is used when the removeScopeInfo feature gate is disabled (legacy behavior).
type LegacyScopeID struct {
	name      string
	version   string
	schemaURL string
}

func (l LegacyScopeID) Key() string {
	return l.name + "\x0f" + l.version + "\x0f" + l.schemaURL
}

func (l LegacyScopeID) Name() string {
	return l.name
}

func (l LegacyScopeID) Version() string {
	return l.version
}

func (l LegacyScopeID) SchemaURL() string {
	return l.schemaURL
}

func (LegacyScopeID) Attributes() pcommon.Map {
	return pcommon.NewMap() // Return empty but valid map for legacy scopes
}

func (l LegacyScopeID) IsEmpty() bool {
	return l.name == "" && l.version == "" && l.schemaURL == ""
}

// ScopeID represents a scope identified by name, version, schema URL, and attributes.
// This is used when the removeScopeInfo feature gate is enabled (new behavior).
type ScopeID struct {
	name       string
	version    string
	schemaURL  string
	attributes pcommon.Map
}

func (s ScopeID) Key() string {
	// Create a deterministic key that includes attributes
	// Using non-printable characters as separators to avoid collisions
	key := s.name + "\x0f" + s.version + "\x0f" + s.schemaURL + "\x0f"
	if s.attributes.Len() > 0 {
		// Sort attribute keys for deterministic ordering
		keys := make([]string, 0, s.attributes.Len())
		s.attributes.Range(func(k string, _ pcommon.Value) bool {
			keys = append(keys, k)
			return true
		})
		sort.Strings(keys)
		for _, k := range keys {
			v, _ := s.attributes.Get(k)
			key += k + "\x00" + v.AsString() + "\x01"
		}
	}
	return key
}

func (s ScopeID) Name() string {
	return s.name
}

func (s ScopeID) Version() string {
	return s.version
}

func (s ScopeID) SchemaURL() string {
	return s.schemaURL
}

func (s ScopeID) Attributes() pcommon.Map {
	return s.attributes
}

func (s ScopeID) IsEmpty() bool {
	return s.name == "" && s.version == "" && s.schemaURL == "" && s.attributes.Len() == 0
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
		families:               make(map[resourceKey]map[string]map[metricFamilyKey]*metricFamily),
		isNew:                  true,
		trimSuffixes:           trimSuffixes,
		enableNativeHistograms: enableNativeHistograms,
		removeScopeInfo:        RemoveScopeInfoGate.IsEnabled(),
		sink:                   sink,
		metricAdjuster:         metricAdjuster,
		externalLabels:         externalLabels,
		logger:                 settings.Logger,
		buildInfo:              settings.BuildInfo,
		obsrecv:                obsrecv,
		bufBytes:               make([]byte, 0, 1024),
		scopeMap:               make(map[resourceKey]map[string]ScopeIdentifier),
		scopeAttributes:        make(map[resourceKey]map[LegacyScopeID]pcommon.Map),
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

	// For the `otel_scope_info` metric we have different behavior depending on the feature gate.
	// It becomes a new scope when the feature gate is disabled, and a regular metric when it is enabled.
	if metricName == prometheus.ScopeInfoMetricName && !t.removeScopeInfo {
		t.addScopeInfo(*rKey, ls)
		return 0, nil
	}

	scope := t.getScopeIdentifier(ls)

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
func (t *transaction) detectAndStoreNativeHistogramStaleness(atMs int64, key *resourceKey, scope ScopeIdentifier, metricName string, ls labels.Labels) bool {
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
	if md.MetricFamily != metricName {
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
func (t *transaction) getOrCreateMetricFamily(key resourceKey, scope ScopeIdentifier, mn string) *metricFamily {
	scopeKey := scope.Key()

	if _, ok := t.families[key]; !ok {
		t.families[key] = make(map[string]map[metricFamilyKey]*metricFamily)
	}
	if _, ok := t.families[key][scopeKey]; !ok {
		t.families[key][scopeKey] = make(map[metricFamilyKey]*metricFamily)
	}

	// Store the scope identifier for later use
	if _, ok := t.scopeMap[key]; !ok {
		t.scopeMap[key] = make(map[string]ScopeIdentifier)
	}
	t.scopeMap[key][scopeKey] = scope

	mfKey := metricFamilyKey{isExponentialHistogram: t.addingNativeHistogram, name: mn}

	curMf, ok := t.families[key][scopeKey][mfKey]

	if !ok {
		fn := mn
		if _, ok := t.mc.GetMetadata(mn); !ok {
			fn = normalizeMetricName(mn)
		}
		fnKey := metricFamilyKey{isExponentialHistogram: mfKey.isExponentialHistogram, name: fn}
		mf, ok := t.families[key][scopeKey][fnKey]
		if !ok || !mf.includesMetric(mn) {
			curMf = newMetricFamily(mn, t.mc, t.logger)
			if curMf.mtype == pmetric.MetricTypeHistogram && mfKey.isExponentialHistogram {
				curMf.mtype = pmetric.MetricTypeExponentialHistogram
			}
			t.families[key][scopeKey][metricFamilyKey{isExponentialHistogram: mfKey.isExponentialHistogram, name: curMf.name}] = curMf
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

	mf := t.getOrCreateMetricFamily(*rKey, t.getScopeIdentifier(l), mn)
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

	curMF := t.getOrCreateMetricFamily(*rKey, t.getScopeIdentifier(ls), metricName)

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

	curMF := t.getOrCreateMetricFamily(*rKey, t.getScopeIdentifier(ls), metricName)

	seriesRef := t.getSeriesRef(ls, curMF.mtype)
	curMF.addCreationTimestamp(seriesRef, ls, atMs, ctMs)

	return storage.SeriesRef(seriesRef), nil
}

func (*transaction) SetOptions(_ *storage.AppendOptions) {
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

		for scopeKey, mfs := range families {
			scope, ok := t.scopeMap[rKey][scopeKey]
			if !ok {
				continue // Should not happen, but skip if scope not found
			}

			ils := rms.ScopeMetrics().AppendEmpty()
			// If metrics don't include otel_scope_name or otel_scope_version
			// labels, use the receiver name and version.
			if scope.IsEmpty() {
				ils.Scope().SetName(mdata.ScopeName)
				ils.Scope().SetVersion(t.buildInfo.Version)
			} else {
				// Otherwise, use the scope that was provided with the metrics.
				ils.Scope().SetName(scope.Name())
				ils.Scope().SetVersion(scope.Version())
				if scope.SchemaURL() != "" {
					ils.SetSchemaUrl(scope.SchemaURL())
				}
				// Copy scope attributes if they exist
				if scope.Attributes().Len() > 0 {
					scope.Attributes().CopyTo(ils.Scope().Attributes())
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

func (t *transaction) getScopeIdentifier(ls labels.Labels) ScopeIdentifier {
	// Check if this is an otel_scope_info metric when feature gate is enabled
	metricName := ls.Get("__name__")
	if t.removeScopeInfo && metricName == prometheus.ScopeInfoMetricName {
		// When feature gate is enabled, otel_scope_info should extract only basic scope info
		// (name, version, schema_url) but NOT other otel_scope_ attributes
		scope := ScopeID{
			attributes: pcommon.NewMap(),
		}
		ls.Range(func(lbl labels.Label) {
			switch lbl.Name {
			case prometheus.ScopeNameLabelKey:
				scope.name = lbl.Value
			case prometheus.ScopeVersionLabelKey:
				scope.version = lbl.Value
			case prometheus.ScopeSchemaURLLabelKey:
				scope.schemaURL = lbl.Value
				// Don't extract other otel_scope_ labels as scope attributes for otel_scope_info when gate is enabled
			}
		})
		return scope
	}

	// Always extract otel_scope_ labels as scope attributes for all other cases
	return t.getScopeWithAttributes(ls)
}

func (*transaction) getScopeWithAttributes(ls labels.Labels) ScopeID {
	scope := ScopeID{
		attributes: pcommon.NewMap(),
	}

	ls.Range(func(lbl labels.Label) {
		switch {
		case lbl.Name == prometheus.ScopeNameLabelKey:
			scope.name = lbl.Value
			return
		case lbl.Name == prometheus.ScopeVersionLabelKey:
			scope.version = lbl.Value
			return
		case lbl.Name == prometheus.ScopeSchemaURLLabelKey:
			scope.schemaURL = lbl.Value
			return
		case strings.HasPrefix(lbl.Name, "otel_scope_"):
			// Extract scope attributes from otel_scope_ prefixed labels
			attrName := strings.TrimPrefix(lbl.Name, "otel_scope_")
			scope.attributes.PutStr(attrName, lbl.Value)
			return
		}
	})
	return scope
}

func (t *transaction) initTransaction(lbs labels.Labels) (*resourceKey, error) {
	target, ok := scrape.TargetFromContext(t.ctx)
	if !ok {
		return nil, errors.New("unable to find target in context")
	}
	t.mc, ok = scrape.MetricMetadataStoreFromContext(t.ctx)
	if !ok {
		return nil, errors.New("unable to find MetricMetadataStore in context")
	}

	rKey, err := t.getJobAndInstance(lbs)
	if err != nil {
		return nil, err
	}
	if _, ok := t.nodeResources[*rKey]; !ok {
		t.nodeResources[*rKey] = CreateResource(rKey.job, rKey.instance, target.DiscoveredLabels(labels.NewBuilder(labels.EmptyLabels())))
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

func (*transaction) Rollback() error {
	return nil
}

func (*transaction) UpdateMetadata(_ storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
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

	// Extract scope information from otel_scope_info metric
	scopeInfoAttrs := pcommon.NewMap()
	scopeFromInfo := ScopeID{
		attributes: pcommon.NewMap(),
	}

	ls.Range(func(lbl labels.Label) {
		if lbl.Name == model.JobLabel || lbl.Name == model.InstanceLabel || lbl.Name == model.MetricNameLabel {
			return
		}
		switch lbl.Name {
		case prometheus.ScopeNameLabelKey:
			scopeFromInfo.name = lbl.Value
		case prometheus.ScopeVersionLabelKey:
			scopeFromInfo.version = lbl.Value
		case prometheus.ScopeSchemaURLLabelKey:
			scopeFromInfo.schemaURL = lbl.Value
		default:
			// All other labels from otel_scope_info become scope attributes
			scopeInfoAttrs.PutStr(lbl.Name, lbl.Value)
		}
	})

	// Initialize scope map if needed
	if _, ok := t.scopeMap[key]; !ok {
		t.scopeMap[key] = make(map[string]ScopeIdentifier)
	}

	// Look for existing scopes with the same name/version/schema (ignoring attributes for now)
	var existingScopeKey string
	var existingScope ScopeID
	var found bool

	for scopeKey, scope := range t.scopeMap[key] {
		if scopeID, ok := scope.(ScopeID); ok {
			if scopeID.name == scopeFromInfo.name &&
				scopeID.version == scopeFromInfo.version &&
				scopeID.schemaURL == scopeFromInfo.schemaURL {
				existingScopeKey = scopeKey
				existingScope = scopeID
				found = true
				break
			}
		}
	}

	if found {
		// Remove the old scope entry
		delete(t.scopeMap[key], existingScopeKey)

		// Merge attributes: otel_scope_ labels take precedence over otel_scope_info.
		mergedAttrs := pcommon.NewMap()
		scopeInfoAttrs.CopyTo(mergedAttrs)
		existingScope.attributes.Range(func(k string, v pcommon.Value) bool {
			mergedAttrs.PutStr(k, v.AsString())
			return true
		})

		// Create merged scope with new key
		mergedScope := ScopeID{
			name:       scopeFromInfo.name,
			version:    scopeFromInfo.version,
			schemaURL:  scopeFromInfo.schemaURL,
			attributes: mergedAttrs,
		}
		t.scopeMap[key][mergedScope.Key()] = mergedScope

		// Update any existing metric families that were using the old scope key
		if t.families[key] != nil {
			if familiesForOldScope, exists := t.families[key][existingScopeKey]; exists {
				t.families[key][mergedScope.Key()] = familiesForOldScope
				delete(t.families[key], existingScopeKey)
			}
		}
	} else {
		// No existing scope, just store the otel_scope_info attributes
		scopeFromInfo.attributes = scopeInfoAttrs
		t.scopeMap[key][scopeFromInfo.Key()] = scopeFromInfo
	}
}

func getSeriesRef(bytes []byte, ls labels.Labels, mtype pmetric.MetricType) (uint64, []byte) {
	excludeLabels := getSortedNotUsefulLabels(mtype)

	// Always exclude otel_scope_ prefixed labels from series reference hash generation
	// as they are extracted as scope attributes instead of datapoint attributes
	var scopeLabels []string
	ls.Range(func(l labels.Label) {
		if strings.HasPrefix(l.Name, "otel_scope_") {
			scopeLabels = append(scopeLabels, l.Name)
		}
	})
	if len(scopeLabels) > 0 {
		excludeLabels = append(excludeLabels, scopeLabels...)
	}

	return ls.HashWithoutLabels(bytes, excludeLabels...)
}

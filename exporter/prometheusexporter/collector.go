// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"cmp"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	prom "github.com/prometheus/prometheus/storage/remote/otlptranslator/prometheusremotewrite"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

var separatorString = string([]byte{model.SeparatorByte})

type collector struct {
	accumulator accumulator
	logger      *zap.Logger

	sendTimestamps   bool
	namespace        string
	constLabels      prometheus.Labels
	metricFamilies   sync.Map
	metricExpiration time.Duration
	withoutScopeInfo bool

	metricNamer otlptranslator.MetricNamer
	labelNamer  otlptranslator.LabelNamer
}

type metricFamily struct {
	lastSeen time.Time
	mf       *dto.MetricFamily
}

type attrPair struct {
	key   string
	value string
}

type labelBuilder struct {
	keys   []string
	values []string
	index  map[string]int
}

func newLabelBuilder(capacity int) labelBuilder {
	return labelBuilder{
		keys:   make([]string, 0, capacity),
		values: make([]string, 0, capacity),
		index:  make(map[string]int, capacity),
	}
}

func (b *labelBuilder) set(key, value string) {
	if idx, exists := b.index[key]; exists {
		b.values[idx] = value
		return
	}
	b.index[key] = len(b.keys)
	b.keys = append(b.keys, key)
	b.values = append(b.values, value)
}

// addMerged normalizes attrs in lexicographical order of original keys,
// concatenating distinct values of colliding normalized labels with ";".
// Distinct-value tracking is scoped to a single call, so callers should invoke
// addMerged on a fresh or key-disjoint labelBuilder.
func (b *labelBuilder) addMerged(
	attrs pcommon.Map,
	labelNamer otlptranslator.LabelNamer,
	prefix string,
	skipRaw, skipNormalized func(string) bool,
) error {
	if attrs.Len() == 0 {
		return nil
	}

	pairs := make([]attrPair, 0, attrs.Len())
	for k, v := range attrs.All() {
		if skipRaw != nil && skipRaw(k) {
			continue
		}
		pairs = append(pairs, attrPair{key: k, value: v.AsString()})
	}
	if len(pairs) > 1 {
		slices.SortStableFunc(pairs, func(a, b attrPair) int {
			return cmp.Compare(a.key, b.key)
		})
	}

	var multiErrs error
	var seenValues map[string][]string
	for _, p := range pairs {
		rawKey := p.key
		if prefix != "" {
			rawKey = prefix + rawKey
		}
		labelName, err := labelNamer.Build(rawKey)
		if err != nil {
			multiErrs = multierr.Append(multiErrs, err)
			continue
		}
		if skipNormalized != nil && skipNormalized(labelName) {
			continue
		}
		idx, exists := b.index[labelName]
		if !exists {
			b.index[labelName] = len(b.keys)
			b.keys = append(b.keys, labelName)
			b.values = append(b.values, p.value)
			continue
		}
		existing := b.values[idx]
		if seen, hasSeen := seenValues[labelName]; hasSeen {
			if slices.Contains(seen, p.value) {
				continue
			}
			seenValues[labelName] = append(seen, p.value)
		} else {
			if existing == p.value {
				continue
			}
			if seenValues == nil {
				seenValues = make(map[string][]string)
			}
			seenValues[labelName] = []string{existing, p.value}
		}
		b.values[idx] = existing + ";" + p.value
	}
	return multiErrs
}

func filterConstLabels(constLabels prometheus.Labels, variableLabels map[string]int) prometheus.Labels {
	if len(constLabels) == 0 || len(variableLabels) == 0 {
		return constLabels
	}
	for k := range constLabels {
		if _, exists := variableLabels[k]; exists {
			filtered := make(prometheus.Labels, len(constLabels)-1)
			for ck, cv := range constLabels {
				if _, dup := variableLabels[ck]; !dup {
					filtered[ck] = cv
				}
			}
			return filtered
		}
	}
	return constLabels
}

func newCollector(config *Config, logger *zap.Logger) *collector {
	labelNamer := configureLabelNamer(config)

	return &collector{
		accumulator:      newAccumulator(logger, config.MetricExpiration),
		logger:           logger,
		namespace:        normalizeNamespace(config.Namespace, labelNamer, logger),
		sendTimestamps:   config.SendTimestamps,
		constLabels:      config.ConstLabels,
		metricExpiration: config.MetricExpiration,
		withoutScopeInfo: config.WithoutScopeInfo,
		metricNamer:      configureMetricNamer(config),
		labelNamer:       labelNamer,
	}
}

// normalizeNamespace builds and returns the namespace if specified in the config
// If not specified, it returns an empty string
// If building the namespace fails, it logs the error and returns an empty string
func normalizeNamespace(configNamespace string, labelNamer otlptranslator.LabelNamer, logger *zap.Logger) string {
	namespace := ""
	if configNamespace != "" {
		var err error
		namespace, err = labelNamer.Build(configNamespace)
		if err != nil {
			logger.Error("failed to build namespace, ignoring", zap.Error(err))
			namespace = ""
		}
	}
	return namespace
}

// configureMetricNamer configures the MetricNamer based on the translation strategy or legacy configuration
func configureMetricNamer(config *Config) otlptranslator.MetricNamer {
	withSuffixes, utf8Allowed := getTranslationConfiguration(config)
	return otlptranslator.MetricNamer{
		WithMetricSuffixes: withSuffixes,
		Namespace:          config.Namespace,
		UTF8Allowed:        utf8Allowed,
	}
}

// configureLabelNamer configures the LabelNamer based on the translation strategy or legacy configuration
func configureLabelNamer(config *Config) otlptranslator.LabelNamer {
	_, utf8Allowed := getTranslationConfiguration(config)
	return otlptranslator.LabelNamer{
		UTF8Allowed:                 utf8Allowed,
		PreserveMultipleUnderscores: true,
	}
}

// getTranslationConfiguration returns the translation configuration based on the strategy
// Returns (withSuffixes, allowUTF8)
func getTranslationConfiguration(config *Config) (bool, bool) {
	switch config.TranslationStrategy {
	case underscoreEscapingWithSuffixes:
		return true, false
	case underscoreEscapingWithoutSuffixes:
		return false, false
	case noUTF8EscapingWithSuffixes:
		return true, true
	case noTranslation:
		return false, true
	default:
		// Default to UnderscoreEscapingWithSuffixes behavior
		return true, false
	}
}

func convertExemplars(exemplars pmetric.ExemplarSlice) []prometheus.Exemplar {
	length := exemplars.Len()
	result := make([]prometheus.Exemplar, length)

	for i := range length {
		e := exemplars.At(i)
		exemplarLabels := make(prometheus.Labels, 0)

		if traceID := e.TraceID(); !traceID.IsEmpty() {
			exemplarLabels[otlptranslator.ExemplarTraceIDKey] = hex.EncodeToString(traceID[:])
		}

		if spanID := e.SpanID(); !spanID.IsEmpty() {
			exemplarLabels[otlptranslator.ExemplarSpanIDKey] = hex.EncodeToString(spanID[:])
		}

		var value float64
		switch e.ValueType() {
		case pmetric.ExemplarValueTypeDouble:
			value = e.DoubleValue()
		case pmetric.ExemplarValueTypeInt:
			value = float64(e.IntValue())
		}

		result[i] = prometheus.Exemplar{
			Value:     value,
			Labels:    exemplarLabels,
			Timestamp: e.Timestamp().AsTime(),
		}
	}
	return result
}

// Describe is a no-op, because the collector dynamically allocates metrics.
// https://github.com/prometheus/client_golang/blob/v1.9.0/prometheus/collector.go#L28-L40
func (*collector) Describe(chan<- *prometheus.Desc) {}

/*
Processing
*/
func (c *collector) processMetrics(rm pmetric.ResourceMetrics) (n int) {
	return c.accumulator.Accumulate(rm)
}

var errUnknownMetricType = errors.New("unknown metric type")

func (c *collector) convertMetric(metric pmetric.Metric, resourceAttrs pcommon.Map, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map) (prometheus.Metric, error) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return c.convertGauge(metric, resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	case pmetric.MetricTypeSum:
		return c.convertSum(metric, resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	case pmetric.MetricTypeHistogram:
		return c.convertDoubleHistogram(metric, resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	case pmetric.MetricTypeExponentialHistogram:
		return c.convertExponentialHistogram(metric, resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	case pmetric.MetricTypeSummary:
		return c.convertSummary(metric, resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	}

	return nil, errUnknownMetricType
}

const cbnhScale = -53

func bucketsToNativeMap(buckets pmetric.ExponentialHistogramDataPointBuckets, scaleDown int32) map[int]int64 {
	counts := buckets.BucketCounts()
	if counts.Len() == 0 {
		return nil
	}
	out := make(map[int]int64, counts.Len())
	baseOffset := buckets.Offset()
	for i := 0; i < counts.Len(); i++ {
		// Effective bucket index after downscaling: ((offset + i) >> scaleDown) + 1
		idx := (int32(i) + baseOffset) >> scaleDown
		idx++
		out[int(idx)] += int64(counts.At(i))
	}
	return out
}

func (c *collector) convertExponentialHistogram(metric pmetric.Metric, resourceAttrs pcommon.Map, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map) (prometheus.Metric, error) {
	dp := metric.ExponentialHistogram().DataPoints().At(0)

	// Build metadata/labels first.
	desc, attributes, err := c.getMetricMetadata(metric, dto.MetricType_HISTOGRAM.Enum(), dp.Attributes(), resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	if err != nil {
		return nil, err
	}

	schema := dp.Scale()

	// Schema -53 (CBNH) is already supported via the classic histogram path.
	// The Prometheus receiver converts NHCB to OTLP classic Histogram (not ExponentialHistogram),
	// which is then handled by convertDoubleHistogram.
	if schema == cbnhScale {
		return nil, errors.New("unexpected ExponentialHistogram with schema -53; CBNH is supported via classic histogram")
	}
	if schema < -4 {
		return nil, fmt.Errorf("cannot convert exponential to native histogram: scale must be >= -4, was %d", schema)
	}
	if !dp.HasSum() {
		return nil, errors.New("cannot convert exponential to native histogram: sum is not set")
	}
	var scaleDown int32
	if schema > 8 {
		scaleDown = schema - 8
		schema = 8
	}

	pos := bucketsToNativeMap(dp.Positive(), scaleDown)
	neg := bucketsToNativeMap(dp.Negative(), scaleDown)

	// Use created timestamp if start time is set (> 0), else zero value.
	created := time.Time{}
	if dp.StartTimestamp().AsTime().Unix() > 0 {
		created = dp.StartTimestamp().AsTime()
	}

	m, err := prometheus.NewConstNativeHistogram(
		desc,
		dp.Count(),
		dp.Sum(),
		pos,
		neg,
		dp.ZeroCount(),
		schema,
		dp.ZeroThreshold(),
		created,
		attributes...,
	)
	if err != nil {
		return nil, err
	}

	exemplars := convertExemplars(dp.Exemplars())
	if len(exemplars) > 0 {
		m, err = prometheus.NewMetricWithExemplars(m, exemplars...)
		if err != nil {
			return nil, err
		}
	}

	if c.sendTimestamps {
		return prometheus.NewMetricWithTimestamp(dp.Timestamp().AsTime(), m), nil
	}
	return m, nil
}

func (c *collector) getMetricMetadata(metric pmetric.Metric, mType *dto.MetricType, attributes, resourceAttrs pcommon.Map, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map) (*prometheus.Desc, []string, error) {
	name, err := c.metricNamer.Build(prom.TranslatorMetricFromOtelMetric(metric))
	if err != nil {
		return nil, nil, err
	}
	help, err := c.validateMetrics(name, metric.Description(), mType)
	if err != nil {
		return nil, nil, err
	}

	capacity := attributes.Len() + 2 // +2 for job and instance labels
	if !c.withoutScopeInfo {
		capacity += scopeAttributes.Len() + 3 // +3 for scope name, version and schema url
	}

	// Fast path: stream labels directly into slices and rely on prometheus.NewDesc's
	// duplicate label check to detect collisions without extra allocations or sorting.
	keys := make([]string, 0, capacity)
	values := make([]string, 0, capacity)

	var multiErrs error
	for k, v := range attributes.All() {
		labelName, err := c.labelNamer.Build(k)
		if err != nil {
			multiErrs = multierr.Append(multiErrs, err)
			continue
		}
		keys = append(keys, labelName)
		values = append(values, v.AsString())
	}

	if !c.withoutScopeInfo {
		for k, v := range scopeAttributes.All() {
			labelName, err := c.labelNamer.Build("otel_scope_" + k)
			if err != nil {
				multiErrs = multierr.Append(multiErrs, err)
				continue
			}
			if isReservedScopeLabel(labelName) {
				continue
			}
			keys = append(keys, labelName)
			values = append(values, v.AsString())
		}

		keys = append(keys, "otel_scope_name", "otel_scope_version", "otel_scope_schema_url")
		values = append(values, scopeName, scopeVersion, scopeSchemaURL)
	}

	if job, ok := extractJob(resourceAttrs); ok {
		keys = append(keys, model.JobLabel)
		values = append(values, job)
	}
	if instance, ok := extractInstance(resourceAttrs); ok {
		keys = append(keys, model.InstanceLabel)
		values = append(values, instance)
	}
	if multiErrs != nil {
		return nil, nil, multiErrs
	}

	desc := prometheus.NewDesc(name, help, keys, c.constLabels)
	if desc.Err() == nil {
		return desc, values, nil
	}

	return c.buildDeduplicatedDesc(name, help, capacity, attributes, resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
}

func (c *collector) buildDeduplicatedDesc(
	name, help string,
	capacity int,
	attributes, resourceAttrs pcommon.Map,
	scopeName, scopeVersion, scopeSchemaURL string,
	scopeAttributes pcommon.Map,
) (*prometheus.Desc, []string, error) {
	labels := newLabelBuilder(capacity)

	var multiErrs error
	multiErrs = multierr.Append(multiErrs, labels.addMerged(attributes, c.labelNamer, "", nil, nil))

	if !c.withoutScopeInfo {
		if scopeAttributes.Len() > 0 {
			// Merge scope attributes separately so they concatenate intra-source
			// and overwrite colliding datapoint labels via labels.set.
			scopeLabels := newLabelBuilder(scopeAttributes.Len())
			multiErrs = multierr.Append(multiErrs, scopeLabels.addMerged(scopeAttributes, c.labelNamer, "otel_scope_", nil, isReservedScopeLabel))
			for i, k := range scopeLabels.keys {
				labels.set(k, scopeLabels.values[i])
			}
		}

		labels.set("otel_scope_name", scopeName)
		labels.set("otel_scope_version", scopeVersion)
		labels.set("otel_scope_schema_url", scopeSchemaURL)
	}

	if job, ok := extractJob(resourceAttrs); ok {
		labels.set(model.JobLabel, job)
	}
	if instance, ok := extractInstance(resourceAttrs); ok {
		labels.set(model.InstanceLabel, instance)
	}
	if multiErrs != nil {
		return nil, nil, multiErrs
	}
	constLabels := filterConstLabels(c.constLabels, labels.index)
	return prometheus.NewDesc(name, help, labels.keys, constLabels), labels.values, nil
}

func isReservedScopeLabel(labelName string) bool {
	switch labelName {
	case "otel_scope_name", "otel_scope_version", "otel_scope_schema_url":
		return true
	default:
		return false
	}
}

func isIdentifyingResourceAttribute(k string) bool {
	switch k {
	case string(conventions.ServiceNameKey), string(conventions.ServiceNamespaceKey), string(conventions.ServiceInstanceIDKey):
		return true
	default:
		return false
	}
}

func (c *collector) convertGauge(metric pmetric.Metric, resourceAttrs pcommon.Map, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map) (prometheus.Metric, error) {
	ip := metric.Gauge().DataPoints().At(0)

	desc, attributes, err := c.getMetricMetadata(metric, dto.MetricType_GAUGE.Enum(), ip.Attributes(), resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	if err != nil {
		return nil, err
	}

	var value float64
	switch ip.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		value = float64(ip.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		value = ip.DoubleValue()
	}
	metricType := prometheus.GaugeValue
	originalType, ok := metric.Metadata().Get(prometheustranslator.MetricMetadataTypeKey)
	if ok && originalType.Str() == string(model.MetricTypeUnknown) {
		metricType = prometheus.UntypedValue
	}
	m, err := prometheus.NewConstMetric(desc, metricType, value, attributes...)
	if err != nil {
		return nil, err
	}

	if c.sendTimestamps {
		return prometheus.NewMetricWithTimestamp(ip.Timestamp().AsTime(), m), nil
	}
	return m, nil
}

func (c *collector) convertSum(metric pmetric.Metric, resourceAttrs pcommon.Map, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map) (prometheus.Metric, error) {
	ip := metric.Sum().DataPoints().At(0)

	metricType := prometheus.GaugeValue
	mType := dto.MetricType_GAUGE.Enum()
	if metric.Sum().IsMonotonic() {
		metricType = prometheus.CounterValue
		mType = dto.MetricType_COUNTER.Enum()
	}

	desc, attributes, err := c.getMetricMetadata(metric, mType, ip.Attributes(), resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	if err != nil {
		return nil, err
	}
	var value float64
	switch ip.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		value = float64(ip.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		value = ip.DoubleValue()
	}

	var exemplars []prometheus.Exemplar
	// Prometheus currently only supports exporting counters
	if metricType == prometheus.CounterValue {
		exemplars = convertExemplars(ip.Exemplars())
	}

	var m prometheus.Metric
	if metricType == prometheus.CounterValue && ip.StartTimestamp().AsTime().Unix() > 0 {
		m, err = prometheus.NewConstMetricWithCreatedTimestamp(desc, metricType, value, ip.StartTimestamp().AsTime(), attributes...)
	} else {
		m, err = prometheus.NewConstMetric(desc, metricType, value, attributes...)
	}
	if err != nil {
		return nil, err
	}

	if len(exemplars) > 0 {
		m, err = prometheus.NewMetricWithExemplars(m, exemplars...)
		if err != nil {
			return nil, err
		}
	}

	if c.sendTimestamps {
		return prometheus.NewMetricWithTimestamp(ip.Timestamp().AsTime(), m), nil
	}
	return m, nil
}

func (c *collector) convertSummary(metric pmetric.Metric, resourceAttrs pcommon.Map, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map) (prometheus.Metric, error) {
	// TODO: In the off chance that we have multiple points
	// within the same metric, how should we handle them?
	point := metric.Summary().DataPoints().At(0)

	quantiles := make(map[float64]float64)
	qv := point.QuantileValues()
	for j := 0; j < qv.Len(); j++ {
		qvj := qv.At(j)
		// There should be EXACTLY one quantile value lest it is an invalid exposition.
		quantiles[qvj.Quantile()] = qvj.Value()
	}

	desc, attributes, err := c.getMetricMetadata(metric, dto.MetricType_SUMMARY.Enum(), point.Attributes(), resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	if err != nil {
		return nil, err
	}
	var m prometheus.Metric
	if point.StartTimestamp().AsTime().Unix() > 0 {
		m, err = prometheus.NewConstSummaryWithCreatedTimestamp(desc, point.Count(), point.Sum(), quantiles, point.StartTimestamp().AsTime(), attributes...)
	} else {
		m, err = prometheus.NewConstSummary(desc, point.Count(), point.Sum(), quantiles, attributes...)
	}
	if err != nil {
		return nil, err
	}
	if c.sendTimestamps {
		return prometheus.NewMetricWithTimestamp(point.Timestamp().AsTime(), m), nil
	}
	return m, nil
}

func (c *collector) convertDoubleHistogram(metric pmetric.Metric, resourceAttrs pcommon.Map, scopeName, scopeVersion, scopeSchemaURL string, scopeAttributes pcommon.Map) (prometheus.Metric, error) {
	ip := metric.Histogram().DataPoints().At(0)
	desc, attributes, err := c.getMetricMetadata(metric, dto.MetricType_HISTOGRAM.Enum(), ip.Attributes(), resourceAttrs, scopeName, scopeVersion, scopeSchemaURL, scopeAttributes)
	if err != nil {
		return nil, err
	}

	indicesMap := make(map[float64]int)
	buckets := make([]float64, 0, ip.BucketCounts().Len())
	for index := 0; index < ip.ExplicitBounds().Len(); index++ {
		bucket := ip.ExplicitBounds().At(index)
		if _, added := indicesMap[bucket]; !added {
			indicesMap[bucket] = index
			buckets = append(buckets, bucket)
		}
	}
	sort.Float64s(buckets)

	cumCount := uint64(0)

	points := make(map[float64]uint64)
	for _, bucket := range buckets {
		index := indicesMap[bucket]
		var countPerBucket uint64
		if ip.ExplicitBounds().Len() > 0 && index < ip.ExplicitBounds().Len() && index < ip.BucketCounts().Len() {
			countPerBucket = ip.BucketCounts().At(index)
		}
		cumCount += countPerBucket
		points[bucket] = cumCount
	}

	exemplars := convertExemplars(ip.Exemplars())

	var m prometheus.Metric
	if ip.StartTimestamp().AsTime().Unix() > 0 {
		m, err = prometheus.NewConstHistogramWithCreatedTimestamp(desc, ip.Count(), ip.Sum(), points, ip.StartTimestamp().AsTime(), attributes...)
	} else {
		m, err = prometheus.NewConstHistogram(desc, ip.Count(), ip.Sum(), points, attributes...)
	}
	if err != nil {
		return nil, err
	}

	if len(exemplars) > 0 {
		m, err = prometheus.NewMetricWithExemplars(m, exemplars...)
		if err != nil {
			return nil, err
		}
	}

	if c.sendTimestamps {
		return prometheus.NewMetricWithTimestamp(ip.Timestamp().AsTime(), m), nil
	}
	return m, nil
}

func (c *collector) createTargetInfoMetrics(resourceAttrs []pcommon.Map) ([]prometheus.Metric, error) {
	var lastErr error

	// deduplicate resourceAttrs by job and instance
	deduplicatedResourceAttrs := make([]pcommon.Map, 0, len(resourceAttrs))
	seenResource := map[string]struct{}{}
	for _, attrs := range resourceAttrs {
		sig := resourceSignature(attrs)
		if sig == "" {
			continue
		}
		if _, ok := seenResource[sig]; !ok {
			seenResource[sig] = struct{}{}
			deduplicatedResourceAttrs = append(deduplicatedResourceAttrs, attrs)
		}
	}

	var skipRaw func(string) bool
	if !prometheustranslator.KeepIdentifyingResourceAttributesGate.IsEnabled() {
		skipRaw = isIdentifyingResourceAttribute
	}

	metrics := make([]prometheus.Metric, 0, len(deduplicatedResourceAttrs))
	for _, rAttributes := range deduplicatedResourceAttrs {
		labels := newLabelBuilder(rAttributes.Len() + 2) // +2 for job and instance labels.
		if err := labels.addMerged(rAttributes, c.labelNamer, "", skipRaw, nil); err != nil {
			return nil, err
		}

		// Map service.name + service.namespace to job
		if job, ok := extractJob(rAttributes); ok {
			labels.set(model.JobLabel, job)
		}
		// Map service.instance.id to instance
		if instance, ok := extractInstance(rAttributes); ok {
			labels.set(model.InstanceLabel, instance)
		}

		name := prometheustranslator.TargetInfoMetricName
		if c.namespace != "" {
			name = c.namespace + "_" + name
		}

		metric, err := prometheus.NewConstMetric(
			prometheus.NewDesc(name, "Target metadata", labels.keys, nil),
			prometheus.GaugeValue,
			1,
			labels.values...,
		)
		if err != nil {
			lastErr = err
			continue
		}

		metrics = append(metrics, metric)
	}
	return metrics, lastErr
}

/*
Reporting
*/
func (c *collector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Debug("collect called")

	inMetrics, resourceAttrs, scopeNames, scopeVersions, scopeSchemaURLs, scopeAttributes := c.accumulator.Collect()

	targetMetrics, err := c.createTargetInfoMetrics(resourceAttrs)
	if err != nil {
		c.logger.Error(fmt.Sprintf("failed to convert metric %s: %s", otlptranslator.TargetInfoMetricName, err.Error()))
	}
	for _, m := range targetMetrics {
		ch <- m
		c.logger.Debug(fmt.Sprintf("metric served: %s", m.Desc().String()))
	}

	for i := range inMetrics {
		pMetric := inMetrics[i]
		rAttr := resourceAttrs[i]

		m, err := c.convertMetric(pMetric, rAttr, scopeNames[i], scopeVersions[i], scopeSchemaURLs[i], scopeAttributes[i])
		if err != nil {
			c.logger.Error(fmt.Sprintf("failed to convert metric %s: %s", pMetric.Name(), err.Error()))
			continue
		}

		ch <- m
		c.logger.Debug(fmt.Sprintf("metric served: %s", m.Desc().String()))
	}
	c.cleanupMetricFamilies()
}

func (c *collector) validateMetrics(name, description string, metricType *dto.MetricType) (help string, err error) {
	now := time.Now()
	v, exist := c.metricFamilies.Load(name)
	if !exist {
		c.metricFamilies.Store(name, metricFamily{
			lastSeen: now,
			mf: &dto.MetricFamily{
				Name: new(name),
				Help: new(description),
				Type: metricType,
			},
		})
		return description, nil
	}
	emf := v.(metricFamily)
	if emf.mf.GetType() != *metricType {
		return "", fmt.Errorf("instrument type conflict, using existing type definition. instrument: %s, existing: %s, dropped: %s", name, emf.mf.GetType(), *metricType)
	}
	emf.lastSeen = now
	c.metricFamilies.Store(name, emf)
	if emf.mf.GetHelp() != description {
		c.logger.Info(
			"Instrument description conflict, using existing",
			zap.String("instrument", name),
			zap.String("existing", emf.mf.GetHelp()),
			zap.String("dropped", description),
		)
	}
	return emf.mf.GetHelp(), nil
}

func (c *collector) cleanupMetricFamilies() {
	expirationTime := time.Now().Add(-c.metricExpiration)

	c.metricFamilies.Range(func(key, value any) bool {
		v := value.(metricFamily)
		if expirationTime.After(v.lastSeen) {
			c.logger.Debug("metric expired", zap.String("instrument", key.(string)))
			c.metricFamilies.Delete(key)
			return true
		}
		return true
	})
}

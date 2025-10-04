// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"slices"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/cespare/xxhash/v2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/multierr"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

const (
	sumStr        = "_sum"
	countStr      = "_count"
	bucketStr     = "_bucket"
	leStr         = "le"
	quantileStr   = "quantile"
	pInfStr       = "+Inf"
	createdSuffix = "_created"
	// maxExemplarRunes is the maximum number of UTF-8 exemplar characters
	// according to the prometheus specification
	// https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#exemplars
	maxExemplarRunes = 128
	infoType         = "info"
	scopeMetricName  = "otel_scope_info"
	scopeAttrName    = "otel_scope_name"
	scopeAttrVersion = "otel_scope_version"
)

type bucketBoundsData struct {
	ts    *prompb.TimeSeries
	bound float64
}

// byBucketBoundsData enables the usage of sort.Sort() with a slice of bucket bounds
type byBucketBoundsData []bucketBoundsData

func (m byBucketBoundsData) Len() int           { return len(m) }
func (m byBucketBoundsData) Less(i, j int) bool { return m[i].bound < m[j].bound }
func (m byBucketBoundsData) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// ByLabelName enables the usage of sort.Sort() with a slice of labels
type ByLabelName []prompb.Label

func (a ByLabelName) Len() int           { return len(a) }
func (a ByLabelName) Less(i, j int) bool { return a[i].Name < a[j].Name }
func (a ByLabelName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// timeSeriesSignature returns a hashed label set signature.
// The label slice should not contain duplicate label names; this method sorts the slice by label name before creating
// the signature.
// The algorithm is the same as in Prometheus' labels.StableHash function.
func timeSeriesSignature(labels []prompb.Label) uint64 {
	sort.Sort(ByLabelName(labels))

	// Use xxhash.Sum64(b) for fast path as it's faster.
	b := make([]byte, 0, 1024)
	for i, v := range labels {
		if len(b)+len(v.Name)+len(v.Value)+2 >= cap(b) {
			// If labels entry is 1KB+ do not allocate whole entry.
			h := xxhash.New()
			_, _ = h.Write(b)
			for _, v := range labels[i:] {
				_, _ = h.WriteString(v.Name)
				_, _ = h.Write(seps)
				_, _ = h.WriteString(v.Value)
				_, _ = h.Write(seps)
			}
			return h.Sum64()
		}

		b = append(b, v.Name...)
		b = append(b, seps[0])
		b = append(b, v.Value...)
		b = append(b, seps[0])
	}
	return xxhash.Sum64(b)
}

var seps = []byte{'\xff'}

// createAttributes creates a slice of Prometheus Labels with OTLP attributes and pairs of string values.
// Unpaired string values are ignored. String pairs overwrite OTLP labels if collisions happen and
// if logOnOverwrite is true, the overwrite is logged. Resulting label names are sanitized.
func createAttributes(resource pcommon.Resource, attributes pcommon.Map, externalLabels map[string]string,
	ignoreAttrs []string, logOnOverwrite bool, labelNamer otlptranslator.LabelNamer, extras ...string,
) ([]prompb.Label, error) {
	resourceAttrs := resource.Attributes()
	serviceName, haveServiceName := resourceAttrs.Get(string(conventions.ServiceNameKey))
	instance, haveInstanceID := resourceAttrs.Get(string(conventions.ServiceInstanceIDKey))

	// Calculate the maximum possible number of labels we could return so we can preallocate l
	maxLabelCount := attributes.Len() + len(externalLabels) + len(extras)/2

	if haveServiceName {
		maxLabelCount++
	}

	if haveInstanceID {
		maxLabelCount++
	}

	// map ensures no duplicate label name
	l := make(map[string]string, maxLabelCount)

	// Ensure attributes are sorted by key for consistent merging of keys which
	// collide when sanitized.
	labels := make([]prompb.Label, 0, maxLabelCount)
	// XXX: Should we always drop service namespace/service name/service instance ID from the labels
	// (as they get mapped to other Prometheus labels)?
	for key, value := range attributes.All() {
		if !slices.Contains(ignoreAttrs, key) {
			labels = append(labels, prompb.Label{Name: key, Value: value.AsString()})
		}
	}
	sort.Stable(ByLabelName(labels))

	for _, label := range labels {
		finalKey, err := labelNamer.Build(label.Name)
		if err != nil {
			return nil, err
		}
		if existingValue, alreadyExists := l[finalKey]; alreadyExists {
			// Only append to existing value if the new value is different
			if existingValue != label.Value {
				l[finalKey] = existingValue + ";" + label.Value
			}
		} else {
			l[finalKey] = label.Value
		}
	}

	// Map service.name + service.namespace to job
	if haveServiceName {
		val := serviceName.AsString()
		if serviceNamespace, ok := resourceAttrs.Get(string(conventions.ServiceNamespaceKey)); ok {
			val = fmt.Sprintf("%s/%s", serviceNamespace.AsString(), val)
		}
		l[model.JobLabel] = val
	}
	// Map service.instance.id to instance
	if haveInstanceID {
		l[model.InstanceLabel] = instance.AsString()
	}
	for key, value := range externalLabels {
		// External labels have already been sanitized
		if _, alreadyExists := l[key]; alreadyExists {
			// Skip external labels if they are overridden by metric attributes
			continue
		}
		l[key] = value
	}

	for i := 0; i < len(extras); i += 2 {
		if i+1 >= len(extras) {
			break
		}
		_, found := l[extras[i]]
		if found && logOnOverwrite {
			log.Println("label " + extras[i] + " is overwritten. Check if Prometheus reserved labels are used.")
		}
		// internal labels should be maintained
		name := extras[i]
		var err error
		if len(name) <= 4 || name[:2] != "__" || name[len(name)-2:] != "__" {
			name, err = labelNamer.Build(name)
			if err != nil {
				return nil, err
			}
		}
		l[name] = extras[i+1]
	}

	labels = labels[:0]
	for k, v := range l {
		labels = append(labels, prompb.Label{Name: k, Value: v})
	}

	return labels, nil
}

// isValidAggregationTemporality checks whether an OTel metric has a valid
// aggregation temporality for conversion to a Prometheus metric.
func isValidAggregationTemporality(metric pmetric.Metric) bool {
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge, pmetric.MetricTypeSummary:
		return true
	case pmetric.MetricTypeSum:
		return metric.Sum().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	case pmetric.MetricTypeHistogram:
		return metric.Histogram().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	case pmetric.MetricTypeExponentialHistogram:
		return metric.ExponentialHistogram().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	}
	return false
}

func (c *prometheusConverter) addHistogramDataPoints(dataPoints pmetric.HistogramDataPointSlice,
	resource pcommon.Resource, settings Settings, baseName string, scopeName, scopeVersion string,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		timestamp := convertTimeStamp(pt.Timestamp())
		baseLabels, err := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nil, false, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		// Add scope labels to the base labels
		scopeLabels, err := createScopeLabels(scopeName, scopeVersion, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		baseLabels = append(baseLabels, scopeLabels...)

		// If the sum is unset, it indicates the _sum metric point should be
		// omitted
		if pt.HasSum() {
			// treat sum as a sample in an individual TimeSeries
			sum := &prompb.Sample{
				Value:     pt.Sum(),
				Timestamp: timestamp,
			}
			if pt.Flags().NoRecordedValue() {
				sum.Value = math.Float64frombits(value.StaleNaN)
			}

			sumlabels := createLabels(baseName+sumStr, baseLabels)
			c.addSample(sum, sumlabels)
		}

		// treat count as a sample in an individual TimeSeries
		count := &prompb.Sample{
			Value:     float64(pt.Count()),
			Timestamp: timestamp,
		}
		if pt.Flags().NoRecordedValue() {
			count.Value = math.Float64frombits(value.StaleNaN)
		}

		countlabels := createLabels(baseName+countStr, baseLabels)
		c.addSample(count, countlabels)

		// cumulative count for conversion to cumulative histogram
		var cumulativeCount uint64

		var bucketBounds []bucketBoundsData

		// process each bound, based on histograms proto definition, # of buckets = # of explicit bounds + 1
		for i := 0; i < pt.ExplicitBounds().Len() && i < pt.BucketCounts().Len(); i++ {
			bound := pt.ExplicitBounds().At(i)
			cumulativeCount += pt.BucketCounts().At(i)
			bucket := &prompb.Sample{
				Value:     float64(cumulativeCount),
				Timestamp: timestamp,
			}
			if pt.Flags().NoRecordedValue() {
				bucket.Value = math.Float64frombits(value.StaleNaN)
			}
			boundStr := strconv.FormatFloat(bound, 'f', -1, 64)
			labels := createLabels(baseName+bucketStr, baseLabels, leStr, boundStr)
			ts := c.addSample(bucket, labels)

			bucketBounds = append(bucketBounds, bucketBoundsData{ts: ts, bound: bound})
		}
		// add le=+Inf bucket
		infBucket := &prompb.Sample{
			Timestamp: timestamp,
		}
		if pt.Flags().NoRecordedValue() {
			infBucket.Value = math.Float64frombits(value.StaleNaN)
		} else {
			infBucket.Value = float64(pt.Count())
		}
		infLabels := createLabels(baseName+bucketStr, baseLabels, leStr, pInfStr)
		ts := c.addSample(infBucket, infLabels)

		bucketBounds = append(bucketBounds, bucketBoundsData{ts: ts, bound: math.Inf(1)})
		c.addExemplars(pt, bucketBounds)
	}
	return errs
}

type exemplarType interface {
	pmetric.ExponentialHistogramDataPoint | pmetric.HistogramDataPoint | pmetric.NumberDataPoint
	Exemplars() pmetric.ExemplarSlice
}

func getPromExemplars[T exemplarType](pt T) []prompb.Exemplar {
	promExemplars := make([]prompb.Exemplar, 0, pt.Exemplars().Len())
	for i := 0; i < pt.Exemplars().Len(); i++ {
		exemplar := pt.Exemplars().At(i)
		exemplarRunes := 0

		var promExemplar prompb.Exemplar
		switch exemplar.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			promExemplar = prompb.Exemplar{
				Value:     float64(exemplar.IntValue()),
				Timestamp: timestamp.FromTime(exemplar.Timestamp().AsTime()),
			}
		case pmetric.ExemplarValueTypeDouble:
			promExemplar = prompb.Exemplar{
				Value:     exemplar.DoubleValue(),
				Timestamp: timestamp.FromTime(exemplar.Timestamp().AsTime()),
			}
		}
		if traceID := exemplar.TraceID(); !traceID.IsEmpty() {
			val := hex.EncodeToString(traceID[:])
			exemplarRunes += utf8.RuneCountInString(otlptranslator.ExemplarTraceIDKey) + utf8.RuneCountInString(val)
			promLabel := prompb.Label{
				Name:  otlptranslator.ExemplarTraceIDKey,
				Value: val,
			}
			promExemplar.Labels = append(promExemplar.Labels, promLabel)
		}
		if spanID := exemplar.SpanID(); !spanID.IsEmpty() {
			val := hex.EncodeToString(spanID[:])
			exemplarRunes += utf8.RuneCountInString(otlptranslator.ExemplarSpanIDKey) + utf8.RuneCountInString(val)
			promLabel := prompb.Label{
				Name:  otlptranslator.ExemplarSpanIDKey,
				Value: val,
			}
			promExemplar.Labels = append(promExemplar.Labels, promLabel)
		}

		attrs := exemplar.FilteredAttributes()
		labelsFromAttributes := make([]prompb.Label, 0, attrs.Len())
		for key, value := range attrs.All() {
			val := value.AsString()
			exemplarRunes += utf8.RuneCountInString(key) + utf8.RuneCountInString(val)
			promLabel := prompb.Label{
				Name:  key,
				Value: val,
			}

			labelsFromAttributes = append(labelsFromAttributes, promLabel)
		}
		if exemplarRunes <= maxExemplarRunes {
			// only append filtered attributes if it does not cause exemplar
			// labels to exceed the max number of runes
			promExemplar.Labels = append(promExemplar.Labels, labelsFromAttributes...)
		}

		promExemplars = append(promExemplars, promExemplar)
	}

	return promExemplars
}

// mostRecentTimestampInMetric returns the latest timestamp in a batch of metrics
func mostRecentTimestampInMetric(metric pmetric.Metric) pcommon.Timestamp {
	var ts pcommon.Timestamp
	// handle individual metric based on type
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPoints := metric.Gauge().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = max(ts, dataPoints.At(x).Timestamp())
		}
	case pmetric.MetricTypeSum:
		dataPoints := metric.Sum().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = max(ts, dataPoints.At(x).Timestamp())
		}
	case pmetric.MetricTypeHistogram:
		dataPoints := metric.Histogram().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = max(ts, dataPoints.At(x).Timestamp())
		}
	case pmetric.MetricTypeExponentialHistogram:
		dataPoints := metric.ExponentialHistogram().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = max(ts, dataPoints.At(x).Timestamp())
		}
	case pmetric.MetricTypeSummary:
		dataPoints := metric.Summary().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = max(ts, dataPoints.At(x).Timestamp())
		}
	}
	return ts
}

func (c *prometheusConverter) addSummaryDataPoints(dataPoints pmetric.SummaryDataPointSlice, resource pcommon.Resource,
	settings Settings, baseName string, scopeName, scopeVersion string,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)
		timestamp := convertTimeStamp(pt.Timestamp())
		baseLabels, err := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nil, false, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		// Add scope labels to the base labels
		scopeLabels, err := createScopeLabels(scopeName, scopeVersion, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}
		baseLabels = append(baseLabels, scopeLabels...)

		// treat sum as a sample in an individual TimeSeries
		sum := &prompb.Sample{
			Value:     pt.Sum(),
			Timestamp: timestamp,
		}
		if pt.Flags().NoRecordedValue() {
			sum.Value = math.Float64frombits(value.StaleNaN)
		}
		// sum and count of the summary should append suffix to baseName
		sumlabels := createLabels(baseName+sumStr, baseLabels)
		c.addSample(sum, sumlabels)

		// treat count as a sample in an individual TimeSeries
		count := &prompb.Sample{
			Value:     float64(pt.Count()),
			Timestamp: timestamp,
		}
		if pt.Flags().NoRecordedValue() {
			count.Value = math.Float64frombits(value.StaleNaN)
		}
		countlabels := createLabels(baseName+countStr, baseLabels)
		c.addSample(count, countlabels)

		// process each percentile/quantile
		for i := 0; i < pt.QuantileValues().Len(); i++ {
			qt := pt.QuantileValues().At(i)
			quantile := &prompb.Sample{
				Value:     qt.Value(),
				Timestamp: timestamp,
			}
			if pt.Flags().NoRecordedValue() {
				quantile.Value = math.Float64frombits(value.StaleNaN)
			}
			percentileStr := strconv.FormatFloat(qt.Quantile(), 'f', -1, 64)
			qtlabels := createLabels(baseName, baseLabels, quantileStr, percentileStr)
			c.addSample(quantile, qtlabels)
		}
	}
	return errs
}

// createLabels returns a copy of baseLabels, adding to it the pair model.MetricNameLabel=name.
// If extras are provided, corresponding label pairs are also added to the returned slice.
// If extras is uneven length, the last (unpaired) extra will be ignored.
func createLabels(name string, baseLabels []prompb.Label, extras ...string) []prompb.Label {
	extraLabelCount := len(extras) / 2
	labels := make([]prompb.Label, len(baseLabels), len(baseLabels)+extraLabelCount+1) // +1 for name
	copy(labels, baseLabels)

	n := len(extras)
	n -= n % 2
	for extrasIdx := 0; extrasIdx < n; extrasIdx += 2 {
		labels = append(labels, prompb.Label{Name: extras[extrasIdx], Value: extras[extrasIdx+1]})
	}

	labels = append(labels, prompb.Label{Name: model.MetricNameLabel, Value: name})
	return labels
}

// getOrCreateTimeSeries returns the time series corresponding to the label set if existent, and false.
// Otherwise it creates a new one and returns that, and true.
func (c *prometheusConverter) getOrCreateTimeSeries(lbls []prompb.Label) (*prompb.TimeSeries, bool) {
	h := timeSeriesSignature(lbls)
	ts := c.unique[h]
	if ts != nil {
		if isSameMetric(ts, lbls) {
			// We already have this metric
			return ts, false
		}

		// Look for a matching conflict
		for _, cTS := range c.conflicts[h] {
			if isSameMetric(cTS, lbls) {
				// We already have this metric
				return cTS, false
			}
		}

		// New conflict
		ts = &prompb.TimeSeries{
			Labels: lbls,
		}
		c.conflicts[h] = append(c.conflicts[h], ts)
		return ts, true
	}

	// This metric is new
	ts = &prompb.TimeSeries{
		Labels: lbls,
	}
	c.unique[h] = ts
	return ts, true
}

// addResourceTargetInfo converts the resource to the target info metric.
func addResourceTargetInfo(resource pcommon.Resource, settings Settings, timestamp pcommon.Timestamp, converter *prometheusConverter) error {
	if settings.DisableTargetInfo || timestamp == 0 {
		return nil
	}

	attributes := resource.Attributes()
	identifyingAttrs := []string{
		string(conventions.ServiceNamespaceKey),
		string(conventions.ServiceNameKey),
		string(conventions.ServiceInstanceIDKey),
	}
	nonIdentifyingAttrsCount := attributes.Len()
	for _, a := range identifyingAttrs {
		_, haveAttr := attributes.Get(a)
		if haveAttr {
			nonIdentifyingAttrsCount--
		}
	}
	if nonIdentifyingAttrsCount == 0 {
		// If we only have job + instance, then target_info isn't useful, so don't add it.
		return nil
	}

	name := otlptranslator.TargetInfoMetricName
	if settings.Namespace != "" {
		name = settings.Namespace + "_" + name
	}

	labels, err := createAttributes(resource, attributes, settings.ExternalLabels, identifyingAttrs, false, otlptranslator.LabelNamer{PreserveMultipleUnderscores: !prometheustranslator.DropSanitizationGate.IsEnabled()}, model.MetricNameLabel, name)
	if err != nil {
		return err
	}
	haveIdentifier := false
	for _, l := range labels {
		if l.Name == model.JobLabel || l.Name == model.InstanceLabel {
			haveIdentifier = true
			break
		}
	}

	if !haveIdentifier {
		// We need at least one identifying label to generate target_info.
		return nil
	}

	sample := &prompb.Sample{
		Value: float64(1),
		// convert ns to ms
		Timestamp: convertTimeStamp(timestamp),
	}
	converter.addSample(sample, labels)
	return nil
}

// addScopeInfo generates otel_scope_info metrics for each unique scope per resource.
// It only generates the metric when the scope has additional attributes beyond name and version,
// following the same pattern as addResourceTargetInfo.
func addScopeInfo(scopeName, scopeVersion string, scopeAttrs pcommon.Map, resource pcommon.Resource,
	settings Settings, timestamp pcommon.Timestamp, converter *prometheusConverter) error {

	if settings.DisableScopeInfo || timestamp == 0 {
		return nil
	}

	// Only generate otel_scope_info if scope has additional attributes beyond name and version
	if scopeAttrs.Len() == 0 {
		return nil
	}

	// Build the metric name with namespace prefix if configured
	name := scopeMetricName
	if settings.Namespace != "" {
		name = settings.Namespace + "_" + name
	}

	// Create base labels from resource attributes and external labels
	baseLabels, err := createAttributes(resource, pcommon.NewMap(), settings.ExternalLabels, nil, false, converter.labelNamer)
	if err != nil {
		return fmt.Errorf("failed to create base labels for scope info: %w", err)
	}

	// Create scope info labels including scope name, version, and all scope attributes
	labels, err := createScopeInfoLabels(scopeName, scopeVersion, scopeAttrs, baseLabels, converter.labelNamer)
	if err != nil {
		return fmt.Errorf("failed to create scope info labels: %w", err)
	}

	// Add the metric name label
	labels = append(labels, prompb.Label{Name: model.MetricNameLabel, Value: name})

	haveIdentifier := false
	for _, l := range labels {
		if l.Name == model.JobLabel || l.Name == model.InstanceLabel {
			haveIdentifier = true
			break
		}
	}

	if !haveIdentifier {
		// We need at least one identifying label to generate scope_info, similar to target_info
		return nil
	}

	sample := &prompb.Sample{
		Value:     float64(1),
		Timestamp: convertTimeStamp(timestamp),
	}

	converter.addSample(sample, labels)
	return nil
}

// convertTimeStamp converts OTLP timestamp in ns to timestamp in ms
func convertTimeStamp(timestamp pcommon.Timestamp) int64 {
	return timestamp.AsTime().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

// getScopeSignature creates a unique signature for scopes to avoid duplicate otel_scope_info metrics.
// The signature is based on scope name, version, attributes, and resource identifiers to ensure
// proper deduplication per resource+scope combination.
func getScopeSignature(scopeName, scopeVersion string, scopeAttrs pcommon.Map, resource pcommon.Resource) uint64 {
	h := xxhash.New()

	// Include scope name and version
	_, _ = h.WriteString(scopeName)
	_, _ = h.Write(seps)
	_, _ = h.WriteString(scopeVersion)
	_, _ = h.Write(seps)

	// Include resource identifiers for proper scoping
	resourceAttrs := resource.Attributes()
	if serviceName, ok := resourceAttrs.Get(string(conventions.ServiceNameKey)); ok {
		_, _ = h.WriteString(serviceName.AsString())
	}
	_, _ = h.Write(seps)
	if serviceNamespace, ok := resourceAttrs.Get(string(conventions.ServiceNamespaceKey)); ok {
		_, _ = h.WriteString(serviceNamespace.AsString())
	}
	_, _ = h.Write(seps)
	if instanceID, ok := resourceAttrs.Get(string(conventions.ServiceInstanceIDKey)); ok {
		_, _ = h.WriteString(instanceID.AsString())
	}
	_, _ = h.Write(seps)

	// Include scope attributes in sorted order for consistent hashing
	if scopeAttrs.Len() > 0 {
		// Create sorted slice of attribute keys for consistent hashing
		keys := make([]string, 0, scopeAttrs.Len())
		scopeAttrs.Range(func(k string, _ pcommon.Value) bool {
			keys = append(keys, k)
			return true
		})
		sort.Strings(keys)

		for _, key := range keys {
			if value, ok := scopeAttrs.Get(key); ok {
				_, _ = h.WriteString(key)
				_, _ = h.Write(seps)
				_, _ = h.WriteString(value.AsString())
				_, _ = h.Write(seps)
			}
		}
	}

	return h.Sum64()
}

// createScopeLabels creates scope-related labels (otel_scope_name and otel_scope_version)
// to be added to metric data points. Empty scope name or version are omitted.
func createScopeLabels(scopeName, scopeVersion string, labelNamer otlptranslator.LabelNamer) ([]prompb.Label, error) {
	var scopeLabels []prompb.Label

	if scopeName != "" {
		nameLabel, err := labelNamer.Build(scopeAttrName)
		if err != nil {
			return nil, fmt.Errorf("failed to sanitize scope name label: %w", err)
		}
		scopeLabels = append(scopeLabels, prompb.Label{
			Name:  nameLabel,
			Value: scopeName,
		})
	}

	if scopeVersion != "" {
		versionLabel, err := labelNamer.Build(scopeAttrVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to sanitize scope version label: %w", err)
		}
		scopeLabels = append(scopeLabels, prompb.Label{
			Name:  versionLabel,
			Value: scopeVersion,
		})
	}

	return scopeLabels, nil
}

// createScopeInfoLabels creates labels for the otel_scope_info metric, including
// scope name, version, and all additional scope attributes as labels.
func createScopeInfoLabels(scopeName, scopeVersion string, scopeAttrs pcommon.Map,
	baseLabels []prompb.Label, labelNamer otlptranslator.LabelNamer) ([]prompb.Label, error) {

	labels := make([]prompb.Label, len(baseLabels))
	copy(labels, baseLabels)

	// Add scope name and version labels for joining
	scopeLabels, err := createScopeLabels(scopeName, scopeVersion, labelNamer)
	if err != nil {
		return nil, err
	}
	labels = append(labels, scopeLabels...)

	// Add all scope attributes as additional labels
	if scopeAttrs.Len() > 0 {
		attrLabels := make([]prompb.Label, 0, scopeAttrs.Len())
		scopeAttrs.Range(func(key string, value pcommon.Value) bool {
			sanitizedKey, err := labelNamer.Build(key)
			if err != nil {
				// Log warning but continue processing other attributes
				log.Printf("Warning: failed to sanitize scope attribute key %q: %v", key, err)
				return true
			}
			attrLabels = append(attrLabels, prompb.Label{
				Name:  sanitizedKey,
				Value: value.AsString(),
			})
			return true
		})

		sort.Sort(ByLabelName(attrLabels))
		labels = append(labels, attrLabels...)
	}

	return labels, nil
}

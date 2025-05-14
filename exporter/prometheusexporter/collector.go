// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

var separatorString = string([]byte{model.SeparatorByte})

type collector struct {
	accumulator accumulator
	logger      *zap.Logger

	sendTimestamps    bool
	addMetricSuffixes bool
	namespace         string
	constLabels       prometheus.Labels
	metricFamilies    sync.Map
	metricExpiration  time.Duration
}

type metricFamily struct {
	lastSeen time.Time
	mf       *dto.MetricFamily
}

func newCollector(config *Config, logger *zap.Logger) *collector {
	return &collector{
		accumulator:       newAccumulator(logger, config.MetricExpiration),
		logger:            logger,
		namespace:         prometheustranslator.CleanUpString(config.Namespace),
		sendTimestamps:    config.SendTimestamps,
		constLabels:       config.ConstLabels,
		addMetricSuffixes: config.AddMetricSuffixes,
		metricExpiration:  config.MetricExpiration,
	}
}

func convertExemplars(exemplars pmetric.ExemplarSlice) []prometheus.Exemplar {
	length := exemplars.Len()
	result := make([]prometheus.Exemplar, length)

	for i := 0; i < length; i++ {
		e := exemplars.At(i)
		exemplarLabels := make(prometheus.Labels, 0)

		if traceID := e.TraceID(); !traceID.IsEmpty() {
			exemplarLabels[prometheustranslator.ExemplarTraceIDKey] = hex.EncodeToString(traceID[:])
		}

		if spanID := e.SpanID(); !spanID.IsEmpty() {
			exemplarLabels[prometheustranslator.ExemplarSpanIDKey] = hex.EncodeToString(spanID[:])
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
func (c *collector) Describe(_ chan<- *prometheus.Desc) {}

/*
Processing
*/
func (c *collector) processMetrics(rm pmetric.ResourceMetrics) (n int) {
	return c.accumulator.Accumulate(rm)
}

var errUnknownMetricType = errors.New("unknown metric type")

func (c *collector) convertMetric(metric pmetric.Metric, resourceAttrs pcommon.Map) (prometheus.Metric, error) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return c.convertGauge(metric, resourceAttrs)
	case pmetric.MetricTypeSum:
		return c.convertSum(metric, resourceAttrs)
	case pmetric.MetricTypeHistogram:
		return c.convertDoubleHistogram(metric, resourceAttrs)
	case pmetric.MetricTypeSummary:
		return c.convertSummary(metric, resourceAttrs)
	}

	return nil, errUnknownMetricType
}

func (c *collector) getMetricMetadata(metric pmetric.Metric, mType *dto.MetricType, attributes pcommon.Map, resourceAttrs pcommon.Map) (*prometheus.Desc, []string, error) {
	name := prometheustranslator.BuildCompliantName(metric, c.namespace, c.addMetricSuffixes)
	help, err := c.validateMetrics(name, metric.Description(), mType)
	if err != nil {
		return nil, nil, err
	}

	keys := make([]string, 0, attributes.Len()+2) // +2 for job and instance labels.
	values := make([]string, 0, attributes.Len()+2)

	for k, v := range attributes.All() {
		keys = append(keys, prometheustranslator.NormalizeLabel(k))
		values = append(values, v.AsString())
	}

	if job, ok := extractJob(resourceAttrs); ok {
		keys = append(keys, model.JobLabel)
		values = append(values, job)
	}
	if instance, ok := extractInstance(resourceAttrs); ok {
		keys = append(keys, model.InstanceLabel)
		values = append(values, instance)
	}

	return prometheus.NewDesc(name, help, keys, c.constLabels), values, nil
}

func (c *collector) convertGauge(metric pmetric.Metric, resourceAttrs pcommon.Map) (prometheus.Metric, error) {
	ip := metric.Gauge().DataPoints().At(0)

	desc, attributes, err := c.getMetricMetadata(metric, dto.MetricType_GAUGE.Enum(), ip.Attributes(), resourceAttrs)
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

func (c *collector) convertSum(metric pmetric.Metric, resourceAttrs pcommon.Map) (prometheus.Metric, error) {
	ip := metric.Sum().DataPoints().At(0)

	metricType := prometheus.GaugeValue
	mType := dto.MetricType_GAUGE.Enum()
	if metric.Sum().IsMonotonic() {
		metricType = prometheus.CounterValue
		mType = dto.MetricType_COUNTER.Enum()
	}

	desc, attributes, err := c.getMetricMetadata(metric, mType, ip.Attributes(), resourceAttrs)
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

func (c *collector) convertSummary(metric pmetric.Metric, resourceAttrs pcommon.Map) (prometheus.Metric, error) {
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

	desc, attributes, err := c.getMetricMetadata(metric, dto.MetricType_SUMMARY.Enum(), point.Attributes(), resourceAttrs)
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

func (c *collector) convertDoubleHistogram(metric pmetric.Metric, resourceAttrs pcommon.Map) (prometheus.Metric, error) {
	ip := metric.Histogram().DataPoints().At(0)
	desc, attributes, err := c.getMetricMetadata(metric, dto.MetricType_HISTOGRAM.Enum(), ip.Attributes(), resourceAttrs)
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
		if ip.ExplicitBounds().Len() > 0 && index < ip.ExplicitBounds().Len() {
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

	metrics := make([]prometheus.Metric, 0, len(deduplicatedResourceAttrs))
	for _, rAttributes := range deduplicatedResourceAttrs {
		// map ensures no duplicate label name
		labels := make(map[string]string, rAttributes.Len()+2) // +2 for job and instance labels.

		// Use resource attributes (other than those used for job+instance) as the
		// metric labels for the target info metric
		attributes := pcommon.NewMap()
		rAttributes.CopyTo(attributes)
		attributes.RemoveIf(func(k string, _ pcommon.Value) bool {
			switch k {
			case string(conventions.ServiceNameKey), string(conventions.ServiceNamespaceKey), string(conventions.ServiceInstanceIDKey):
				// Remove resource attributes used for job + instance
				return true
			default:
				return false
			}
		})

		for k, v := range attributes.All() {
			finalKey := prometheustranslator.NormalizeLabel(k)
			if existingVal, ok := labels[finalKey]; ok {
				labels[finalKey] = existingVal + ";" + v.AsString()
			} else {
				labels[finalKey] = v.AsString()
			}
		}

		// Map service.name + service.namespace to job
		if job, ok := extractJob(rAttributes); ok {
			labels[model.JobLabel] = job
		}
		// Map service.instance.id to instance
		if instance, ok := extractInstance(rAttributes); ok {
			labels[model.InstanceLabel] = instance
		}

		name := prometheustranslator.TargetInfoMetricName
		if len(c.namespace) > 0 {
			name = c.namespace + "_" + name
		}

		keys := make([]string, 0, len(labels))
		values := make([]string, 0, len(labels))
		for key, value := range labels {
			keys = append(keys, key)
			values = append(values, value)
		}

		metric, err := prometheus.NewConstMetric(
			prometheus.NewDesc(name, "Target metadata", keys, nil),
			prometheus.GaugeValue,
			1,
			values...,
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

	inMetrics, resourceAttrs := c.accumulator.Collect()

	targetMetrics, err := c.createTargetInfoMetrics(resourceAttrs)
	if err != nil {
		c.logger.Error(fmt.Sprintf("failed to convert metric %s: %s", prometheustranslator.TargetInfoMetricName, err.Error()))
	}
	for _, m := range targetMetrics {
		ch <- m
		c.logger.Debug(fmt.Sprintf("metric served: %s", m.Desc().String()))
	}

	for i := range inMetrics {
		pMetric := inMetrics[i]
		rAttr := resourceAttrs[i]

		m, err := c.convertMetric(pMetric, rAttr)
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
				Name: proto.String(name),
				Help: proto.String(description),
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

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"hash/fnv"
	"math"
	"strconv"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// unknownHostName is the default host name when no hostname label is passed.
	unknownHostName = "unknown"
	// splunkMetricTypeKey is the key which maps to the type of the metric.
	splunkMetricTypeKey = "metric_type"
	// splunkMetricValue is the splunk metric value prefix.
	splunkMetricValue = "metric_name"
	// countSuffix is the count metric value suffix.
	countSuffix = "_count"
	// sumSuffix is the sum metric value suffix.
	sumSuffix = "_sum"
	// bucketSuffix is the bucket metric value suffix.
	bucketSuffix = "_bucket"
	// nanValue is the string representation of a NaN value in HEC events
	nanValue = "NaN"
	// plusInfValue is the string representation of a +Inf value in HEC events
	plusInfValue = "+Inf"
	// minusInfValue is the string representation of a -Inf value in HEC events
	minusInfValue = "-Inf"
)

func sanitizeFloat(value float64) interface{} {
	if math.IsNaN(value) {
		return nanValue
	}
	if math.IsInf(value, 1) {
		return plusInfValue
	}
	if math.IsInf(value, -1) {
		return minusInfValue
	}
	return value
}

func mapMetricToSplunkEvent(res pcommon.Resource, m pmetric.Metric, config *Config, logger *zap.Logger) []*splunk.Event {
	sourceKey := config.HecToOtelAttrs.Source
	sourceTypeKey := config.HecToOtelAttrs.SourceType
	indexKey := config.HecToOtelAttrs.Index
	hostKey := config.HecToOtelAttrs.Host
	host := unknownHostName
	source := config.Source
	sourceType := config.SourceType
	index := config.Index
	commonFields := map[string]interface{}{}

	res.Attributes().Range(func(k string, v pcommon.Value) bool {
		switch k {
		case hostKey:
			host = v.Str()
		case sourceKey:
			source = v.Str()
		case sourceTypeKey:
			sourceType = v.Str()
		case indexKey:
			index = v.Str()
		case splunk.HecTokenLabel:
			// ignore
		default:
			commonFields[k] = v.AsString()
		}
		return true
	})
	metricFieldName := splunkMetricValue + ":" + m.Name()
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		pts := m.Gauge().DataPoints()
		splunkMetrics := make([]*splunk.Event, pts.Len())

		for gi := 0; gi < pts.Len(); gi++ {
			dataPt := pts.At(gi)
			fields := cloneMap(commonFields)
			populateAttributes(fields, dataPt.Attributes())
			switch dataPt.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				fields[metricFieldName] = dataPt.IntValue()
			case pmetric.NumberDataPointValueTypeDouble:
				fields[metricFieldName] = sanitizeFloat(dataPt.DoubleValue())
			}
			fields[splunkMetricTypeKey] = pmetric.MetricTypeGauge.String()
			splunkMetrics[gi] = createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
		}
		return splunkMetrics
	case pmetric.MetricTypeHistogram:
		pts := m.Histogram().DataPoints()
		var splunkMetrics []*splunk.Event
		for gi := 0; gi < pts.Len(); gi++ {
			dataPt := pts.At(gi)
			bounds := dataPt.ExplicitBounds()
			counts := dataPt.BucketCounts()
			// first, add one event for sum, and one for count
			if dataPt.HasSum() && !math.IsNaN(dataPt.Sum()) {
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPt.Attributes())
				fields[metricFieldName+sumSuffix] = dataPt.Sum()
				fields[splunkMetricTypeKey] = pmetric.MetricTypeHistogram.String()
				splunkMetrics = append(splunkMetrics, createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields))
			}
			{
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPt.Attributes())
				fields[metricFieldName+countSuffix] = dataPt.Count()
				fields[splunkMetricTypeKey] = pmetric.MetricTypeHistogram.String()
				splunkMetrics = append(splunkMetrics, createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields))
			}
			// Spec says counts is optional but if present it must have one more
			// element than the bounds array.
			if counts.Len() == 0 || counts.Len() != bounds.Len()+1 {
				continue
			}
			value := uint64(0)
			// now create buckets for each bound.
			for bi := 0; bi < bounds.Len(); bi++ {
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPt.Attributes())
				fields["le"] = float64ToDimValue(bounds.At(bi))
				value += counts.At(bi)
				fields[metricFieldName+bucketSuffix] = value
				fields[splunkMetricTypeKey] = pmetric.MetricTypeHistogram.String()
				sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
				splunkMetrics = append(splunkMetrics, sm)
			}
			// add an upper bound for +Inf
			{
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPt.Attributes())
				fields["le"] = float64ToDimValue(math.Inf(1))
				fields[metricFieldName+bucketSuffix] = value + counts.At(counts.Len()-1)
				fields[splunkMetricTypeKey] = pmetric.MetricTypeHistogram.String()
				sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
				splunkMetrics = append(splunkMetrics, sm)
			}
		}
		return splunkMetrics
	case pmetric.MetricTypeSum:
		pts := m.Sum().DataPoints()
		splunkMetrics := make([]*splunk.Event, pts.Len())
		for gi := 0; gi < pts.Len(); gi++ {
			dataPt := pts.At(gi)
			fields := cloneMap(commonFields)
			populateAttributes(fields, dataPt.Attributes())
			switch dataPt.ValueType() {
			case pmetric.NumberDataPointValueTypeInt:
				fields[metricFieldName] = dataPt.IntValue()
			case pmetric.NumberDataPointValueTypeDouble:
				fields[metricFieldName] = sanitizeFloat(dataPt.DoubleValue())
			}
			fields[splunkMetricTypeKey] = pmetric.MetricTypeSum.String()
			sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
			splunkMetrics[gi] = sm
		}
		return splunkMetrics
	case pmetric.MetricTypeSummary:
		pts := m.Summary().DataPoints()
		var splunkMetrics []*splunk.Event
		for gi := 0; gi < pts.Len(); gi++ {
			dataPt := pts.At(gi)
			// first, add one event for sum, and one for count
			if !math.IsNaN(dataPt.Sum()) {
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPt.Attributes())
				fields[metricFieldName+sumSuffix] = dataPt.Sum()
				fields[splunkMetricTypeKey] = pmetric.MetricTypeSummary.String()
				sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
				splunkMetrics = append(splunkMetrics, sm)
			}
			{
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPt.Attributes())
				fields[metricFieldName+countSuffix] = dataPt.Count()
				fields[splunkMetricTypeKey] = pmetric.MetricTypeSummary.String()
				sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
				splunkMetrics = append(splunkMetrics, sm)
			}

			// now create values for each quantile.
			for bi := 0; bi < dataPt.QuantileValues().Len(); bi++ {
				fields := cloneMap(commonFields)
				populateAttributes(fields, dataPt.Attributes())
				dp := dataPt.QuantileValues().At(bi)
				fields["qt"] = float64ToDimValue(dp.Quantile())
				fields[metricFieldName+"_"+strconv.FormatFloat(dp.Quantile(), 'f', -1, 64)] = sanitizeFloat(dp.Value())
				fields[splunkMetricTypeKey] = pmetric.MetricTypeSummary.String()
				sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
				splunkMetrics = append(splunkMetrics, sm)
			}
		}
		return splunkMetrics
	case pmetric.MetricTypeEmpty:
		return nil
	default:
		logger.Warn(
			"Point with unsupported type",
			zap.Any("metric", m))
		return nil
	}
}

func createEvent(timestamp pcommon.Timestamp, host string, source string, sourceType string, index string, fields map[string]interface{}) *splunk.Event {
	return &splunk.Event{
		Time:       timestampToSecondsWithMillisecondPrecision(timestamp),
		Host:       host,
		Source:     source,
		SourceType: sourceType,
		Index:      index,
		Event:      splunk.HecEventMetricType,
		Fields:     fields,
	}
}

func copyEventWithoutValues(event *splunk.Event) *splunk.Event {
	return &splunk.Event{
		Time:       event.Time,
		Host:       event.Host,
		Source:     event.Source,
		SourceType: event.SourceType,
		Index:      event.Index,
		Event:      event.Event,
		Fields: cloneMapWithSelector(event.Fields, func(key string) bool {
			return !strings.HasPrefix(key, splunkMetricValue)
		}),
	}
}

func populateAttributes(fields map[string]interface{}, attributeMap pcommon.Map) {
	attributeMap.Range(func(k string, v pcommon.Value) bool {
		fields[k] = v.AsString()
		return true
	})
}

func cloneMap(fields map[string]interface{}) map[string]interface{} {
	newFields := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		newFields[k] = v
	}
	return newFields
}

func cloneMapWithSelector(fields map[string]interface{}, selector func(string) bool) map[string]interface{} {
	newFields := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		if selector(k) {
			newFields[k] = v
		}
	}
	return newFields
}

func timestampToSecondsWithMillisecondPrecision(ts pcommon.Timestamp) float64 {
	return math.Round(float64(ts)/1e6) / 1e3
}

func float64ToDimValue(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// merge metric events to adhere to the multimetric format event.
func mergeEventsToMultiMetricFormat(events []*splunk.Event) ([]*splunk.Event, error) {
	hashes := map[uint32]*splunk.Event{}
	hasher := fnv.New32a()
	var merged []*splunk.Event

	for _, e := range events {
		cloned := copyEventWithoutValues(e)
		marshaler := jsoniter.ConfigCompatibleWithStandardLibrary
		data, err := marshaler.Marshal(cloned)
		if err != nil {
			return nil, err
		}
		_, err = hasher.Write(data)
		if err != nil {
			return nil, err
		}
		hashed := hasher.Sum32()
		hasher.Reset()
		src, ok := hashes[hashed]
		if !ok {
			hashes[hashed] = e
			merged = append(merged, e)
		} else {
			for field, value := range e.Fields {
				if strings.HasPrefix(field, splunkMetricValue) {
					src.Fields[field] = value
				}
			}
		}
	}
	return merged, nil
}

// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter

import (
	"math"
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
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
)

func metricDataToSplunk(logger *zap.Logger, data pdata.Metrics, config *Config) ([]*splunk.Event, int) {
	numDroppedTimeSeries := 0
	splunkMetrics := make([]*splunk.Event, 0, data.DataPointCount())
	rms := data.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		host := unknownHostName
		source := config.Source
		sourceType := config.SourceType
		index := config.Index
		commonFields := map[string]interface{}{}
		resource := rm.Resource()
		attributes := resource.Attributes()
		if conventionHost, isSet := attributes.Get(conventions.AttributeHostName); isSet {
			host = conventionHost.StringVal()
		}
		if sourceSet, isSet := attributes.Get(splunk.DefaultSourceLabel); isSet {
			source = sourceSet.StringVal()
		}
		if sourcetypeSet, isSet := attributes.Get(splunk.DefaultSourceTypeLabel); isSet {
			sourceType = sourcetypeSet.StringVal()
		}
		if indexSet, isSet := attributes.Get(splunk.DefaultIndexLabel); isSet {
			index = indexSet.StringVal()
		}
		attributes.Range(func(k string, v pdata.AttributeValue) bool {
			commonFields[k] = v.AsString()
			return true
		})

		rm.Resource().Attributes().Range(func(k string, v pdata.AttributeValue) bool {
			commonFields[k] = v.AsString()
			return true
		})
		ilms := rm.InstrumentationLibraryMetrics()
		for ilmi := 0; ilmi < ilms.Len(); ilmi++ {
			ilm := ilms.At(ilmi)
			metrics := ilm.Metrics()
			for tmi := 0; tmi < metrics.Len(); tmi++ {
				tm := metrics.At(tmi)
				metricFieldName := splunkMetricValue + ":" + tm.Name()
				switch tm.DataType() {
				case pdata.MetricDataTypeGauge:
					pts := tm.Gauge().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						fields := cloneMap(commonFields)
						populateAttributes(fields, dataPt.Attributes())
						switch dataPt.Type() {
						case pdata.MetricValueTypeInt:
							fields[metricFieldName] = dataPt.IntVal()
						case pdata.MetricValueTypeDouble:
							fields[metricFieldName] = dataPt.DoubleVal()
						}
						fields[splunkMetricTypeKey] = pdata.MetricDataTypeGauge.String()
						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeHistogram:
					pts := tm.Histogram().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						bounds := dataPt.ExplicitBounds()
						counts := dataPt.BucketCounts()
						// first, add one event for sum, and one for count
						{
							fields := cloneMap(commonFields)
							populateAttributes(fields, dataPt.Attributes())
							fields[metricFieldName+sumSuffix] = dataPt.Sum()
							fields[splunkMetricTypeKey] = pdata.MetricDataTypeHistogram.String()
							sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
							splunkMetrics = append(splunkMetrics, sm)
						}
						{
							fields := cloneMap(commonFields)
							populateAttributes(fields, dataPt.Attributes())
							fields[metricFieldName+countSuffix] = dataPt.Count()
							fields[splunkMetricTypeKey] = pdata.MetricDataTypeHistogram.String()
							sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
							splunkMetrics = append(splunkMetrics, sm)
						}
						// Spec says counts is optional but if present it must have one more
						// element than the bounds array.
						if len(counts) == 0 || len(counts) != len(bounds)+1 {
							continue
						}
						value := uint64(0)
						// now create buckets for each bound.
						for bi := 0; bi < len(bounds); bi++ {
							fields := cloneMap(commonFields)
							populateAttributes(fields, dataPt.Attributes())
							fields["le"] = float64ToDimValue(bounds[bi])
							value += counts[bi]
							fields[metricFieldName+bucketSuffix] = value
							fields[splunkMetricTypeKey] = pdata.MetricDataTypeHistogram.String()
							sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
							splunkMetrics = append(splunkMetrics, sm)
						}
						// add an upper bound for +Inf
						{
							fields := cloneMap(commonFields)
							populateAttributes(fields, dataPt.Attributes())
							fields["le"] = float64ToDimValue(math.Inf(1))
							fields[metricFieldName+bucketSuffix] = value + counts[len(counts)-1]
							fields[splunkMetricTypeKey] = pdata.MetricDataTypeHistogram.String()
							sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
							splunkMetrics = append(splunkMetrics, sm)
						}
					}
				case pdata.MetricDataTypeSum:
					pts := tm.Sum().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						fields := cloneMap(commonFields)
						populateAttributes(fields, dataPt.Attributes())
						switch dataPt.Type() {
						case pdata.MetricValueTypeInt:
							fields[metricFieldName] = dataPt.IntVal()
						case pdata.MetricValueTypeDouble:
							fields[metricFieldName] = dataPt.DoubleVal()
						}
						fields[splunkMetricTypeKey] = pdata.MetricDataTypeSum.String()
						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeSummary:
					pts := tm.Summary().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						// first, add one event for sum, and one for count
						{
							fields := cloneMap(commonFields)
							populateAttributes(fields, dataPt.Attributes())
							fields[metricFieldName+sumSuffix] = dataPt.Sum()
							fields[splunkMetricTypeKey] = pdata.MetricDataTypeSummary.String()
							sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
							splunkMetrics = append(splunkMetrics, sm)
						}
						{
							fields := cloneMap(commonFields)
							populateAttributes(fields, dataPt.Attributes())
							fields[metricFieldName+countSuffix] = dataPt.Count()
							fields[splunkMetricTypeKey] = pdata.MetricDataTypeSummary.String()
							sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
							splunkMetrics = append(splunkMetrics, sm)
						}

						// now create values for each quantile.
						for bi := 0; bi < dataPt.QuantileValues().Len(); bi++ {
							fields := cloneMap(commonFields)
							populateAttributes(fields, dataPt.Attributes())
							dp := dataPt.QuantileValues().At(bi)
							fields["qt"] = float64ToDimValue(dp.Quantile())
							fields[metricFieldName+"_"+strconv.FormatFloat(dp.Quantile(), 'f', -1, 64)] = dp.Value()
							fields[splunkMetricTypeKey] = pdata.MetricDataTypeSummary.String()
							sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
							splunkMetrics = append(splunkMetrics, sm)
						}
					}
				case pdata.MetricDataTypeNone:
					fallthrough
				default:
					logger.Warn(
						"Point with unsupported type",
						zap.Any("metric", rm))
					numDroppedTimeSeries++
				}
			}
		}
	}

	return splunkMetrics, numDroppedTimeSeries
}

func createEvent(timestamp pdata.Timestamp, host string, source string, sourceType string, index string, fields map[string]interface{}) *splunk.Event {
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

func populateAttributes(fields map[string]interface{}, attributeMap pdata.AttributeMap) {
	attributeMap.Range(func(k string, v pdata.AttributeValue) bool {
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

func timestampToSecondsWithMillisecondPrecision(ts pdata.Timestamp) *float64 {
	if ts == 0 {
		// some telemetry sources send data with timestamps set to 0 by design, as their original target destinations
		// (i.e. before Open Telemetry) are setup with the know-how on how to consume them. In this case,
		// we want to omit the time field when sending data to the Splunk HEC so that the HEC adds a timestamp
		// at indexing time, which will be much more useful than a 0-epoch-time value.
		return nil
	}

	val := math.Round(float64(ts)/1e6) / 1e3

	return &val
}

func float64ToDimValue(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

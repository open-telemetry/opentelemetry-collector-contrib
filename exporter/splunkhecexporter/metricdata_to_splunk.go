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
	"fmt"
	"math"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// unknownHostName is the default host name when no hostname label is passed.
	unknownHostName = "unknown"
	// splunkMetricValue is the splunk metric value prefix.
	splunkMetricValue = "metric_name"
	// countSuffix is the count metric value suffix.
	countSuffix = "count"
)

func metricDataToSplunk(logger *zap.Logger, data pdata.Metrics, config *Config) ([]*splunk.Event, int, error) {
	numDroppedTimeSeries := 0
	_, dpCount := data.MetricAndDataPointCount()
	splunkMetrics := make([]*splunk.Event, 0, dpCount)
	rms := data.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		if rm.IsNil() {
			continue
		}

		host := unknownHostName
		source := config.Source
		sourceType := config.SourceType
		index := config.Index
		commonFields := map[string]interface{}{}
		if conventionHost, isSet := rm.Resource().Attributes().Get(conventions.AttributeHostHostname); isSet {
			host = conventionHost.StringVal()
		}
		if sourceSet, isSet := rm.Resource().Attributes().Get(conventions.AttributeServiceName); isSet {
			source = sourceSet.StringVal()
		}
		if sourcetypeSet, isSet := rm.Resource().Attributes().Get(splunk.SourcetypeLabel); isSet {
			sourceType = sourcetypeSet.StringVal()
		}
		rm.Resource().Attributes().ForEach(func(k string, v pdata.AttributeValue) {
			commonFields[k] = tracetranslator.AttributeValueToString(v, false)
		})
		ilms := rm.InstrumentationLibraryMetrics()
		for ilmi := 0; ilmi < ilms.Len(); ilmi++ {
			ilm := ilms.At(ilmi)
			if ilm.IsNil() {
				continue
			}
			metrics := ilm.Metrics()
			for tmi := 0; tmi < metrics.Len(); tmi++ {
				tm := metrics.At(tmi)
				if tm.IsNil() {
					continue
				}
				metricFieldName := splunkMetricValue + ":" + tm.Name()
				switch tm.DataType() {
				case pdata.MetricDataTypeIntGauge:
					if tm.IntGauge().IsNil() {
						continue
					}
					pts := tm.IntGauge().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						if dataPt.IsNil() {
							continue
						}
						fields := cloneMap(commonFields)
						populateLabels(fields, dataPt.LabelsMap())
						fields[metricFieldName] = dataPt.Value()

						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeDoubleGauge:
					if tm.DoubleGauge().IsNil() {
						continue
					}
					pts := tm.DoubleGauge().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						if dataPt.IsNil() {
							continue
						}
						fields := cloneMap(commonFields)
						populateLabels(fields, dataPt.LabelsMap())
						fields[metricFieldName] = dataPt.Value()
						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeDoubleHistogram:
					if tm.DoubleHistogram().IsNil() {
						continue
					}
					pts := tm.DoubleHistogram().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						if dataPt.IsNil() {
							continue
						}
						fields := cloneMap(commonFields)
						populateLabels(fields, dataPt.LabelsMap())
						fields[fmt.Sprintf("%s:%s_%s", splunkMetricValue, tm.Name(), countSuffix)] = dataPt.Count()
						fields[metricFieldName] = dataPt.Sum()
						for bi := 0; bi < len(dataPt.ExplicitBounds()); bi++ {
							bound := dataPt.ExplicitBounds()[bi]
							fields[fmt.Sprintf("%s:%s_%f", splunkMetricValue, tm.Name(), bound)] = dataPt.BucketCounts()[bi]
						}

						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeIntHistogram:
					if tm.IntHistogram().IsNil() {
						continue
					}
					pts := tm.IntHistogram().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						if dataPt.IsNil() {
							continue
						}
						fields := cloneMap(commonFields)
						populateLabels(fields, dataPt.LabelsMap())
						fields[fmt.Sprintf("%s:%s_%s", splunkMetricValue, tm.Name(), countSuffix)] = dataPt.Count()
						fields[metricFieldName] = dataPt.Sum()
						for bi := 0; bi < len(dataPt.ExplicitBounds()); bi++ {
							bound := dataPt.ExplicitBounds()[bi]
							fields[fmt.Sprintf("%s:%s_%f", splunkMetricValue, tm.Name(), bound)] = dataPt.BucketCounts()[bi]
						}

						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeDoubleSum:
					if tm.DoubleSum().IsNil() {
						continue
					}
					pts := tm.DoubleSum().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						if dataPt.IsNil() {
							continue
						}
						fields := cloneMap(commonFields)
						populateLabels(fields, dataPt.LabelsMap())
						fields[metricFieldName] = dataPt.Value()

						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeIntSum:
					if tm.IntSum().IsNil() {
						continue
					}
					pts := tm.IntSum().DataPoints()
					for gi := 0; gi < pts.Len(); gi++ {
						dataPt := pts.At(gi)
						if dataPt.IsNil() {
							continue
						}
						fields := cloneMap(commonFields)
						populateLabels(fields, dataPt.LabelsMap())
						fields[metricFieldName] = dataPt.Value()

						sm := createEvent(dataPt.Timestamp(), host, source, sourceType, index, fields)
						splunkMetrics = append(splunkMetrics, sm)
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

	return splunkMetrics, numDroppedTimeSeries, nil
}

func createEvent(timestamp pdata.TimestampUnixNano, host string, source string, sourceType string, index string, fields map[string]interface{}) *splunk.Event {
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

func populateLabels(fields map[string]interface{}, labelsMap pdata.StringMap) {
	labelsMap.ForEach(func(k string, v string) {
		fields[k] = v
	})
}

func cloneMap(fields map[string]interface{}) map[string]interface{} {
	newFields := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		newFields[k] = v
	}
	return newFields
}

func timestampToSecondsWithMillisecondPrecision(ts pdata.TimestampUnixNano) *float64 {
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

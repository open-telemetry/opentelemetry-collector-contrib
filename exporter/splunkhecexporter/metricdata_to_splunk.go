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
	var splunkMetrics []*splunk.Event
	for i := 0; i < data.ResourceMetrics().Len(); i++ {
		rm := data.ResourceMetrics().At(i)

		host := unknownHostName
		source := config.Source
		sourceType := config.SourceType
		if !rm.Resource().IsNil() {
			if conventionHost, isSet := rm.Resource().Attributes().Get(conventions.AttributeHostHostname); isSet {
				host = conventionHost.StringVal()
			}
			if sourceSet, isSet := rm.Resource().Attributes().Get(conventions.AttributeServiceName); isSet {
				source = sourceSet.StringVal()
			}
			if sourcetypeSet, isSet := rm.Resource().Attributes().Get(splunk.SourcetypeLabel); isSet {
				sourceType = sourcetypeSet.StringVal()
			}
		}
		rm.InstrumentationLibraryMetrics().Len()
		for ilmi := 0; ilmi < rm.InstrumentationLibraryMetrics().Len(); ilmi++ {
			ilm := rm.InstrumentationLibraryMetrics().At(ilmi)
			for tmi := 0; tmi < ilm.Metrics().Len(); tmi++ {
				tm := ilm.Metrics().At(tmi)

				switch tm.DataType() {
				case pdata.MetricDataTypeIntGauge:
					for gi := 0; gi < tm.IntGauge().DataPoints().Len(); gi++ {
						fields := map[string]interface{}{}
						if !rm.Resource().IsNil() {
							rm.Resource().Attributes().ForEach(func(k string, v pdata.AttributeValue) {
								fields[k] = v.StringVal()
							})
						}
						rm.Resource().Attributes()
						dataPt := tm.IntGauge().DataPoints().At(gi)
						fields[fmt.Sprintf("%s:%s", splunkMetricValue, tm.Name())] = dataPt.Value()
						dataPt.LabelsMap().ForEach(func(k string, v string) {
							fields[k] = v
						})
						sm := &splunk.Event{
							Time:       timestampToEpochMilliseconds(dataPt.Timestamp()),
							Host:       host,
							Source:     source,
							SourceType: sourceType,
							Index:      config.Index,
							Event:      splunk.HecEventMetricType,
							Fields:     fields,
						}
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeDoubleGauge:
					for gi := 0; gi < tm.DoubleGauge().DataPoints().Len(); gi++ {
						fields := map[string]interface{}{}
						if !rm.Resource().IsNil() {
							rm.Resource().Attributes().ForEach(func(k string, v pdata.AttributeValue) {
								fields[k] = v.StringVal()
							})
						}
						dataPt := tm.DoubleGauge().DataPoints().At(gi)
						fields[fmt.Sprintf("%s:%s", splunkMetricValue, tm.Name())] = dataPt.Value()
						dataPt.LabelsMap().ForEach(func(k string, v string) {
							fields[k] = v
						})
						sm := &splunk.Event{
							Time:       timestampToEpochMilliseconds(dataPt.Timestamp()),
							Host:       host,
							Source:     config.Source,
							SourceType: config.SourceType,
							Index:      config.Index,
							Event:      splunk.HecEventMetricType,
							Fields:     fields,
						}
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeDoubleHistogram:
					for gi := 0; gi < tm.DoubleHistogram().DataPoints().Len(); gi++ {
						fields := map[string]interface{}{}
						if !rm.Resource().IsNil() {
							rm.Resource().Attributes().ForEach(func(k string, v pdata.AttributeValue) {
								fields[k] = v.StringVal()
							})
						}
						dataPt := tm.DoubleHistogram().DataPoints().At(gi)
						fields[fmt.Sprintf("%s:%s_%s", splunkMetricValue, tm.Name(), countSuffix)] = dataPt.Count()
						fields[fmt.Sprintf("%s:%s", splunkMetricValue, tm.Name())] = dataPt.Sum()
						for bi := 0; bi < len(dataPt.ExplicitBounds()); bi++ {
							bound := dataPt.ExplicitBounds()[bi]
							fields[fmt.Sprintf("%s:%s_%f", splunkMetricValue, tm.Name(), bound)] = dataPt.BucketCounts()[bi]
						}

						dataPt.LabelsMap().ForEach(func(k string, v string) {
							fields[k] = v
						})
						sm := &splunk.Event{
							Time:       timestampToEpochMilliseconds(dataPt.Timestamp()),
							Host:       host,
							Source:     config.Source,
							SourceType: config.SourceType,
							Index:      config.Index,
							Event:      splunk.HecEventMetricType,
							Fields:     fields,
						}
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeIntHistogram:
					for gi := 0; gi < tm.IntHistogram().DataPoints().Len(); gi++ {
						fields := map[string]interface{}{}
						if !rm.Resource().IsNil() {
							rm.Resource().Attributes().ForEach(func(k string, v pdata.AttributeValue) {
								fields[k] = v.StringVal()
							})
						}
						dataPt := tm.IntHistogram().DataPoints().At(gi)
						fields[fmt.Sprintf("%s:%s_%s", splunkMetricValue, tm.Name(), countSuffix)] = dataPt.Count()
						fields[fmt.Sprintf("%s:%s", splunkMetricValue, tm.Name())] = dataPt.Sum()
						for bi := 0; bi < len(dataPt.ExplicitBounds()); bi++ {
							bound := dataPt.ExplicitBounds()[bi]
							fields[fmt.Sprintf("%s:%s_%f", splunkMetricValue, tm.Name(), bound)] = dataPt.BucketCounts()[bi]
						}

						dataPt.LabelsMap().ForEach(func(k string, v string) {
							fields[k] = v
						})
						sm := &splunk.Event{
							Time:       timestampToEpochMilliseconds(dataPt.Timestamp()),
							Host:       host,
							Source:     config.Source,
							SourceType: config.SourceType,
							Index:      config.Index,
							Event:      splunk.HecEventMetricType,
							Fields:     fields,
						}
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeDoubleSum:
					for gi := 0; gi < tm.DoubleSum().DataPoints().Len(); gi++ {
						fields := map[string]interface{}{}
						dataPt := tm.DoubleSum().DataPoints().At(gi)
						fields[fmt.Sprintf("%s:%s", splunkMetricValue, tm.Name())] = dataPt.Value()

						dataPt.LabelsMap().ForEach(func(k string, v string) {
							fields[k] = v
						})
						sm := &splunk.Event{
							Time:       timestampToEpochMilliseconds(dataPt.Timestamp()),
							Host:       host,
							Source:     config.Source,
							SourceType: config.SourceType,
							Index:      config.Index,
							Event:      splunk.HecEventMetricType,
							Fields:     fields,
						}
						splunkMetrics = append(splunkMetrics, sm)
					}
				case pdata.MetricDataTypeIntSum:
					for gi := 0; gi < tm.IntSum().DataPoints().Len(); gi++ {
						fields := map[string]interface{}{}
						dataPt := tm.IntSum().DataPoints().At(gi)
						fields[fmt.Sprintf("%s:%s", splunkMetricValue, tm.Name())] = dataPt.Value()

						dataPt.LabelsMap().ForEach(func(k string, v string) {
							fields[k] = v
						})
						sm := &splunk.Event{
							Time:       timestampToEpochMilliseconds(dataPt.Timestamp()),
							Host:       host,
							Source:     config.Source,
							SourceType: config.SourceType,
							Index:      config.Index,
							Event:      splunk.HecEventMetricType,
							Fields:     fields,
						}
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

func timestampToEpochMilliseconds(ts pdata.TimestampUnixNano) *float64 {
	if ts == 0 {
		// some telemetry sources send data with timestamps set to 0 by design, as their original target destinations
		// (i.e. before Open Telemetry) are setup with the know-how on how to consume them. In this case,
		// we want to omit the time field when sending data to the Splunk HEC so that the HEC adds a timestamp
		// at indexing time, which will be much more useful than a 0-epoch-time value.
		return nil
	}

	val := math.Round(float64(ts)/1e6)/1e3

	return &val
}

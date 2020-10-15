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

package splunkhecreceiver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

// SplunkHecToMetricsData converts Splunk HEC metric points to
// consumerdata.MetricsData. Returning the converted data and the number of
// dropped time series.
func SplunkHecToMetricsData(logger *zap.Logger, events []*splunk.Event, resourceCustomizer func(pdata.Resource)) (pdata.Metrics, int) {

	// TODO: not optimized at all, basically regenerating everything for each
	// 	data point.
	numDroppedTimeSeries := 0
	md := pdata.NewMetrics()

	for _, event := range events {
		resourceMetrics := pdata.NewResourceMetrics()
		resourceMetrics.InitEmpty()

		metrics := pdata.NewInstrumentationLibraryMetrics()
		metrics.InitEmpty()

		labelKeys, labelValues := buildLabelKeysAndValues(event.Fields)
		if len(labelKeys) > 0 {
			resourceMetrics.Resource().InitEmpty()
		}
		for i, k := range labelKeys {
			resourceMetrics.Resource().Attributes().InsertString(k, labelValues[i])
		}
		resourceCustomizer(resourceMetrics.Resource())

		values := event.GetMetricValues()
		metricNames := make([]string, 0, len(values))
		for k := range values {
			metricNames = append(metricNames, k)
		}
		sort.Strings(metricNames)

		for _, metricName := range metricNames {
			pointTimestamp := convertTimestamp(event.Time)
			metric := pdata.NewMetric()
			metric.InitEmpty()
			// TODO currently mapping all values to unspecified.
			metric.SetDataType(pdata.MetricDataTypeNone)
			metric.SetName(metricName)

			metricValue := values[metricName]
			if i, ok := metricValue.(int64); ok {
				addIntGauge(pointTimestamp, i, metric)
			} else if i, ok := metricValue.(*int64); ok {
				addIntGauge(pointTimestamp, *i, metric)
			} else if f, ok := metricValue.(float64); ok {
				if f == float64(int64(f)) {
					addIntGauge(pointTimestamp, int64(f), metric)
				} else {
					addDoubleGauge(pointTimestamp, f, metric)
				}
			} else if f, ok := metricValue.(*float64); ok {
				if *f == float64(int64(*f)) {
					addIntGauge(pointTimestamp, int64(*f), metric)
				} else {
					addDoubleGauge(pointTimestamp, *f, metric)
				}
			} else if s, ok := metricValue.(*string); ok {
				// best effort, cast to string and turn into a number
				dbl, err := strconv.ParseFloat(*s, 64)
				if err != nil {
					numDroppedTimeSeries++
					logger.Debug("Cannot convert metric",
						zap.String("metric", metricName))
				} else {
					addDoubleGauge(pointTimestamp, dbl, metric)
				}
			} else {
				// drop this point as we do not know how to extract a value from it
				numDroppedTimeSeries++
				logger.Debug("Cannot convert metric",
					zap.String("metric", metricName))
			}

			if metric.DataType() != pdata.MetricDataTypeNone {
				metrics.Metrics().Append(metric)
			}
		}
		if metrics.Metrics().Len() > 0 {
			resourceMetrics.InstrumentationLibraryMetrics().Append(metrics)
			md.ResourceMetrics().Append(resourceMetrics)
		}
	}

	return md, numDroppedTimeSeries
}

func addIntGauge(ts pdata.TimestampUnixNano, value int64, metric pdata.Metric) {
	intPt := pdata.NewIntDataPoint()
	intPt.InitEmpty()
	intPt.SetTimestamp(ts)
	intPt.SetValue(value)
	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	metric.IntGauge().InitEmpty()
	metric.IntGauge().DataPoints().Append(intPt)
}

func addDoubleGauge(ts pdata.TimestampUnixNano, value float64, metric pdata.Metric) {
	doublePt := pdata.NewDoubleDataPoint()
	doublePt.InitEmpty()
	doublePt.SetTimestamp(ts)
	doublePt.SetValue(value)
	metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	metric.DoubleGauge().InitEmpty()
	metric.DoubleGauge().DataPoints().Append(doublePt)
}

func convertTimestamp(sec float64) pdata.TimestampUnixNano {
	if sec == 0 {
		return 0
	}
	return pdata.TimestampUnixNano(sec * 1e9)
}

func buildLabelKeysAndValues(
	dimensions map[string]interface{},
) ([]string, []string) {
	keys := make([]string, 0, len(dimensions))
	values := make([]string, 0, len(dimensions))
	dimensionKeys := make([]string, 0, len(dimensions))
	for key := range dimensions {
		dimensionKeys = append(dimensionKeys, key)
	}
	sort.Strings(dimensionKeys)
	for _, key := range dimensionKeys {

		if strings.HasPrefix(key, "metric_name") {
			continue
		}
		if key == "" || dimensions[key] == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		keys = append(keys, key)
		values = append(values, fmt.Sprintf("%v", dimensions[key]))
	}
	return keys, values
}

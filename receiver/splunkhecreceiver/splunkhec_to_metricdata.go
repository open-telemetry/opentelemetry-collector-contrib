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
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// splunkHecToMetricsData converts Splunk HEC metric points to
// pdata.Metrics. Returning the converted data and the number of
// dropped time series.
func splunkHecToMetricsData(logger *zap.Logger, events []*splunk.Event, resourceCustomizer func(pdata.Resource), config splunk.HECConfiguration) (pdata.Metrics, int) {
	numDroppedTimeSeries := 0
	md := pdata.NewMetrics()

	for _, event := range events {
		resourceMetrics := pdata.NewResourceMetrics()
		attrs := resourceMetrics.Resource().Attributes()
		if event.Host != "" {
			attrs.InsertString(config.GetHostKey(), event.Host)
		}
		if event.Source != "" {
			attrs.InsertString(config.GetSourceKey(), event.Source)
		}
		if event.SourceType != "" {
			attrs.InsertString(config.GetSourceTypeKey(), event.SourceType)
		}
		if event.Index != "" {
			attrs.InsertString(config.GetIndexKey(), event.Index)
		}
		resourceCustomizer(resourceMetrics.Resource())

		values := event.GetMetricValues()

		labels := buildAttributes(event.Fields)

		metrics := resourceMetrics.InstrumentationLibraryMetrics().AppendEmpty().Metrics()
		for metricName, metricValue := range values {
			pointTimestamp := convertTimestamp(event.Time)
			metric := pdata.NewMetric()
			metric.SetName(metricName)

			switch v := metricValue.(type) {
			case int64:
				addIntGauge(metrics, metricName, v, pointTimestamp, labels)
			case *int64:
				addIntGauge(metrics, metricName, *v, pointTimestamp, labels)
			case float64:
				addDoubleGauge(metrics, metricName, v, pointTimestamp, labels)
			case *float64:
				addDoubleGauge(metrics, metricName, *v, pointTimestamp, labels)
			case string:
				convertString(logger, &numDroppedTimeSeries, metrics, metricName, pointTimestamp, v, labels)
			case *string:
				convertString(logger, &numDroppedTimeSeries, metrics, metricName, pointTimestamp, *v, labels)
			default:
				// drop this point as we do not know how to extract a value from it
				numDroppedTimeSeries++
				logger.Debug("Cannot convert metric, unknown input type",
					zap.String("metric", metricName))
			}
		}

		if metrics.Len() > 0 {
			tgt := md.ResourceMetrics().AppendEmpty()
			resourceMetrics.CopyTo(tgt)
		}
	}

	return md, numDroppedTimeSeries
}

func convertString(logger *zap.Logger, numDroppedTimeSeries *int, metrics pdata.MetricSlice, metricName string, pointTimestamp pdata.Timestamp, s string, attributes pdata.AttributeMap) {
	// best effort, cast to string and turn into a number
	dbl, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*numDroppedTimeSeries++
		logger.Debug("Cannot convert metric value from string to number",
			zap.String("metric", metricName))
	} else {
		addDoubleGauge(metrics, metricName, dbl, pointTimestamp, attributes)
	}
}

func addIntGauge(metrics pdata.MetricSlice, metricName string, value int64, ts pdata.Timestamp, attributes pdata.AttributeMap) {
	metric := metrics.AppendEmpty()
	metric.SetName(metricName)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	intPt := metric.Gauge().DataPoints().AppendEmpty()
	intPt.SetTimestamp(ts)
	intPt.SetIntVal(value)
	attributes.CopyTo(intPt.Attributes())
}

func addDoubleGauge(metrics pdata.MetricSlice, metricName string, value float64, ts pdata.Timestamp, attributes pdata.AttributeMap) {
	metric := metrics.AppendEmpty()
	metric.SetName(metricName)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	doublePt := metric.Gauge().DataPoints().AppendEmpty()
	doublePt.SetTimestamp(ts)
	doublePt.SetDoubleVal(value)
	attributes.CopyTo(doublePt.Attributes())
}

func convertTimestamp(sec *float64) pdata.Timestamp {
	if sec == nil {
		return 0
	}

	return pdata.Timestamp(*sec * 1e9)
}

// Extract dimensions from the Splunk event fields to populate metric data point attributes.
func buildAttributes(dimensions map[string]interface{}) pdata.AttributeMap {
	attributes := pdata.NewAttributeMap()
	attributes.EnsureCapacity(len(dimensions))
	for key, val := range dimensions {

		if strings.HasPrefix(key, "metric_name") {
			continue
		}
		if key == "" || val == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		attributes.InsertString(key, fmt.Sprintf("%v", val))
	}
	return attributes
}

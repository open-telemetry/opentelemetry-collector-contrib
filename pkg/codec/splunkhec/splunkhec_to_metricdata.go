// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhec // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/codec/splunkhec"

import (
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// splunkHecToMetricsData converts Splunk HEC metric points to
// pmetric.Metrics. Returning the converted data and the number of
// dropped time series.
func splunkHecToMetricsData(logger *zap.Logger, events []*splunk.Event, config *splunk.HecToOtelAttrs) pmetric.Metrics {
	md := pmetric.NewMetrics()
	scopeMetricsMap := make(map[[4]string]pmetric.ScopeMetrics)
	for _, event := range events {
		values := event.GetMetricValues()

		labels := buildAttributes(event.Fields)

		metrics := pmetric.NewMetricSlice()
		for metricName, metricValue := range values {
			pointTimestamp := convertTimestamp(event.Time)
			metric := pmetric.NewMetric()
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
				convertString(logger, metrics, metricName, pointTimestamp, v, labels)
			case *string:
				convertString(logger, metrics, metricName, pointTimestamp, *v, labels)
			default:
				// drop this point as we do not know how to extract a value from it
				logger.Debug("Cannot convert metric, unknown input type",
					zap.String("metric", metricName))
			}
		}

		if metrics.Len() == 0 {
			continue
		}
		key := [4]string{event.Host, event.Source, event.SourceType, event.Index}
		var sm pmetric.ScopeMetrics
		var found bool
		if sm, found = scopeMetricsMap[key]; !found {
			resourceMetrics := md.ResourceMetrics().AppendEmpty()
			sm = resourceMetrics.ScopeMetrics().AppendEmpty()
			scopeMetricsMap[key] = sm

			attrs := resourceMetrics.Resource().Attributes()
			if event.Host != "" {
				attrs.PutStr(config.Host, event.Host)
			}
			if event.Source != "" {
				attrs.PutStr(config.Source, event.Source)
			}
			if event.SourceType != "" {
				attrs.PutStr(config.SourceType, event.SourceType)
			}
			if event.Index != "" {
				attrs.PutStr(config.Index, event.Index)
			}
		}
		metrics.MoveAndAppendTo(sm.Metrics())
	}

	return md
}

func convertString(logger *zap.Logger, metrics pmetric.MetricSlice, metricName string, pointTimestamp pcommon.Timestamp, s string, attributes pcommon.Map) {
	// best effort, cast to string and turn into a number
	dbl, err := strconv.ParseFloat(s, 64)
	if err != nil {
		logger.Debug("Cannot convert metric value from string to number",
			zap.String("metric", metricName))
	} else {
		addDoubleGauge(metrics, metricName, dbl, pointTimestamp, attributes)
	}
}

func addIntGauge(metrics pmetric.MetricSlice, metricName string, value int64, ts pcommon.Timestamp, attributes pcommon.Map) {
	metric := metrics.AppendEmpty()
	metric.SetName(metricName)
	intPt := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	intPt.SetTimestamp(ts)
	intPt.SetIntValue(value)
	attributes.CopyTo(intPt.Attributes())
}

func addDoubleGauge(metrics pmetric.MetricSlice, metricName string, value float64, ts pcommon.Timestamp, attributes pcommon.Map) {
	metric := metrics.AppendEmpty()
	metric.SetName(metricName)
	doublePt := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	doublePt.SetTimestamp(ts)
	doublePt.SetDoubleValue(value)
	attributes.CopyTo(doublePt.Attributes())
}

func convertTimestamp(sec float64) pcommon.Timestamp {
	return pcommon.Timestamp(sec * 1e9)
}

// Extract dimensions from the Splunk event fields to populate metric data point attributes.
func buildAttributes(dimensions map[string]interface{}) pcommon.Map {
	attributes := pcommon.NewMap()
	attributes.EnsureCapacity(len(dimensions))
	for key, val := range dimensions {

		if strings.HasPrefix(key, "metric_name") {
			continue
		}
		if key == "" || val == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		attributes.PutStr(key, fmt.Sprintf("%v", val))
	}
	return attributes
}

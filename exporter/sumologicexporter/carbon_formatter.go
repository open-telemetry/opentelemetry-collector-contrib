// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// carbon2TagString returns all attributes as space spearated key=value pairs.
// In addition, metric name and unit are also included.
// In case `metric` or `unit` attributes has been set too, they are prefixed
// with underscore `_` to avoid overwriting the metric name and unit.
func carbon2TagString(record metricPair) string {
	length := record.attributes.Len()

	if _, ok := record.attributes.Get("metric"); ok {
		length++
	}

	if _, ok := record.attributes.Get("unit"); ok && len(record.metric.Unit()) > 0 {
		length++
	}

	returnValue := make([]string, 0, length)
	record.attributes.Range(func(k string, v pcommon.Value) bool {
		if k == "name" || k == "unit" {
			k = fmt.Sprintf("_%s", k)
		}
		returnValue = append(returnValue, fmt.Sprintf(
			"%s=%s",
			sanitizeCarbonString(k),
			sanitizeCarbonString(v.AsString()),
		))
		return true
	})

	returnValue = append(returnValue, fmt.Sprintf("metric=%s", sanitizeCarbonString(record.metric.Name())))

	if len(record.metric.Unit()) > 0 {
		returnValue = append(returnValue, fmt.Sprintf("unit=%s", sanitizeCarbonString(record.metric.Unit())))
	}

	return strings.Join(returnValue, " ")
}

// sanitizeCarbonString replaces problematic characters with underscore
func sanitizeCarbonString(text string) string {
	return strings.NewReplacer(" ", "_", "=", ":", "\n", "_").Replace(text)
}

// carbon2NumberRecord converts NumberDataPoint to carbon2 metric string
// with additional information from metricPair.
func carbon2NumberRecord(record metricPair, dataPoint pmetric.NumberDataPoint) string {
	switch dataPoint.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return fmt.Sprintf("%s  %g %d",
			carbon2TagString(record),
			dataPoint.DoubleValue(),
			dataPoint.Timestamp()/1e9,
		)
	case pmetric.NumberDataPointValueTypeInt:
		return fmt.Sprintf("%s  %d %d",
			carbon2TagString(record),
			dataPoint.IntValue(),
			dataPoint.Timestamp()/1e9,
		)
	case pmetric.NumberDataPointValueTypeEmpty:
		return ""
	}
	return ""
}

// carbon2metric2String converts metric to Carbon2 formatted string.
func carbon2Metric2String(record metricPair) string {
	var nextLines []string
	//exhaustive:enforce
	switch record.metric.Type() {
	case pmetric.MetricTypeGauge:
		dps := record.metric.Gauge().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, carbon2NumberRecord(record, dps.At(i)))
		}
	case pmetric.MetricTypeSum:
		dps := record.metric.Sum().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, carbon2NumberRecord(record, dps.At(i)))
		}
	// Skip complex metrics
	case pmetric.MetricTypeHistogram:
	case pmetric.MetricTypeSummary:
	case pmetric.MetricTypeEmpty:
	case pmetric.MetricTypeExponentialHistogram:
	}

	return strings.Join(nextLines, "\n")
}

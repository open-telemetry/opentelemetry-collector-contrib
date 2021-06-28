// Copyright 2021, OpenTelemetry Authors
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

package sumologicexporter

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
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
	record.attributes.Range(func(k string, v pdata.AttributeValue) bool {
		if k == "name" || k == "unit" {
			k = fmt.Sprintf("_%s", k)
		}
		returnValue = append(returnValue, fmt.Sprintf(
			"%s=%s",
			sanitizeCarbonString(k),
			sanitizeCarbonString(tracetranslator.AttributeValueToString(v)),
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

// carbon2IntRecord converts IntDataPoint to carbon2 metric string
// with additional information from metricPair.
func carbon2IntRecord(record metricPair, dataPoint pdata.IntDataPoint) string {
	return fmt.Sprintf("%s  %d %d",
		carbon2TagString(record),
		dataPoint.Value(),
		dataPoint.Timestamp()/1e9,
	)
}

// carbon2DoubleRecord converts DoubleDataPoint to carbon2 metric string
// with additional information from metricPair.
func carbon2DoubleRecord(record metricPair, dataPoint pdata.DoubleDataPoint) string {
	return fmt.Sprintf("%s  %g %d",
		carbon2TagString(record),
		dataPoint.Value(),
		dataPoint.Timestamp()/1e9,
	)
}

// carbon2metric2String converts metric to Carbon2 formatted string.
func carbon2Metric2String(record metricPair) string {
	var nextLines []string

	switch record.metric.DataType() {
	case pdata.MetricDataTypeIntGauge:
		dps := record.metric.IntGauge().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, carbon2IntRecord(record, dps.At(i)))
		}
	case pdata.MetricDataTypeIntSum:
		dps := record.metric.IntSum().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, carbon2IntRecord(record, dps.At(i)))
		}
	case pdata.MetricDataTypeDoubleGauge:
		dps := record.metric.DoubleGauge().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, carbon2DoubleRecord(record, dps.At(i)))
		}
	case pdata.MetricDataTypeDoubleSum:
		dps := record.metric.DoubleSum().DataPoints()
		nextLines = make([]string, 0, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			nextLines = append(nextLines, carbon2DoubleRecord(record, dps.At(i)))
		}
	// Skip complex metrics
	case pdata.MetricDataTypeHistogram:
	case pdata.MetricDataTypeIntHistogram:
	case pdata.MetricDataTypeSummary:
	}

	return strings.Join(nextLines, "\n")
}

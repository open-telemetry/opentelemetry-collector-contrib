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

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type dataPoint interface {
	Timestamp() pcommon.Timestamp
	Attributes() pcommon.Map
}

type prometheusFormatter struct {
	sanitNameRegex *regexp.Regexp
	replacer       *strings.Replacer
}

type prometheusTags string

const (
	prometheusLeTag       string = "le"
	prometheusQuantileTag string = "quantile"
	prometheusInfValue    string = "+Inf"
)

func newPrometheusFormatter() prometheusFormatter {
	sanitNameRegex := regexp.MustCompile(`[^0-9a-zA-Z]`)

	return prometheusFormatter{
		sanitNameRegex: sanitNameRegex,
		replacer:       strings.NewReplacer(`\`, `\\`, `"`, `\"`),
	}
}

// PrometheusLabels returns all attributes as sanitized prometheus labels string
func (f *prometheusFormatter) tags2String(attr pcommon.Map, labels pcommon.Map) prometheusTags {
	mergedAttributes := pcommon.NewMap()
	attr.CopyTo(mergedAttributes)
	labels.Range(func(k string, v pcommon.Value) bool {
		mergedAttributes.UpsertString(k, v.StringVal())
		return true
	})
	length := mergedAttributes.Len()

	if length == 0 {
		return ""
	}

	returnValue := make([]string, 0, length)
	mergedAttributes.Range(func(k string, v pcommon.Value) bool {
		returnValue = append(
			returnValue,
			fmt.Sprintf(
				`%s="%s"`,
				f.sanitizeKey(k),
				f.sanitizeValue(v.AsString()),
			),
		)
		return true
	})

	return prometheusTags(fmt.Sprintf("{%s}", strings.Join(returnValue, ",")))
}

// sanitizeKey returns sanitized key string by replacing
// all non-alphanumeric chars with `_`
func (f *prometheusFormatter) sanitizeKey(s string) string {
	return f.sanitNameRegex.ReplaceAllString(s, "_")
}

// sanitizeKey returns sanitized value string performing the following substitutions:
// `/` -> `//`
// `"` -> `\"`
// `\n` -> `\n`
func (f *prometheusFormatter) sanitizeValue(s string) string {
	return strings.ReplaceAll(f.replacer.Replace(s), `\\n`, `\n`)
}

// doubleLine builds metric based on the given arguments where value is float64
func (f *prometheusFormatter) doubleLine(name string, attributes prometheusTags, value float64, timestamp pcommon.Timestamp) string {
	return fmt.Sprintf(
		"%s%s %g %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pcommon.Timestamp(time.Millisecond),
	)
}

// intLine builds metric based on the given arguments where value is int64
func (f *prometheusFormatter) intLine(name string, attributes prometheusTags, value int64, timestamp pcommon.Timestamp) string {
	return fmt.Sprintf(
		"%s%s %d %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pcommon.Timestamp(time.Millisecond),
	)
}

// uintLine builds metric based on the given arguments where value is uint64
func (f *prometheusFormatter) uintLine(name string, attributes prometheusTags, value uint64, timestamp pcommon.Timestamp) string {
	return fmt.Sprintf(
		"%s%s %d %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pcommon.Timestamp(time.Millisecond),
	)
}

// doubleValueLine returns prometheus line with given value
func (f *prometheusFormatter) doubleValueLine(name string, value float64, dp dataPoint, attributes pcommon.Map) string {
	return f.doubleLine(
		name,
		f.tags2String(attributes, dp.Attributes()),
		value,
		dp.Timestamp(),
	)
}

// uintValueLine returns prometheus line with given value
func (f *prometheusFormatter) uintValueLine(name string, value uint64, dp dataPoint, attributes pcommon.Map) string {
	return f.uintLine(
		name,
		f.tags2String(attributes, dp.Attributes()),
		value,
		dp.Timestamp(),
	)
}

// numberDataPointValueLine returns prometheus line with value from pmetric.NumberDataPoint
func (f *prometheusFormatter) numberDataPointValueLine(name string, dp pmetric.NumberDataPoint, attributes pcommon.Map) string {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return f.doubleValueLine(
			name,
			dp.DoubleVal(),
			dp,
			attributes,
		)
	case pmetric.NumberDataPointValueTypeInt:
		return f.intLine(
			name,
			f.tags2String(attributes, dp.Attributes()),
			dp.IntVal(),
			dp.Timestamp(),
		)
	}
	return ""
}

// sumMetric returns _sum suffixed metric name
func (f *prometheusFormatter) sumMetric(name string) string {
	return fmt.Sprintf("%s_sum", name)
}

// countMetric returns _count suffixed metric name
func (f *prometheusFormatter) countMetric(name string) string {
	return fmt.Sprintf("%s_count", name)
}

// mergeAttributes gets two pcommon.Map and returns new which contains values from both of them
func (f *prometheusFormatter) mergeAttributes(attributes pcommon.Map, additionalAttributes pcommon.Map) pcommon.Map {
	mergedAttributes := pcommon.NewMap()
	attributes.CopyTo(mergedAttributes)
	additionalAttributes.Range(func(k string, v pcommon.Value) bool {
		mergedAttributes.Upsert(k, v)
		return true
	})
	return mergedAttributes
}

// doubleGauge2Strings converts DoubleGauge record to a list of strings (one per dataPoint)
func (f *prometheusFormatter) gauge2Strings(record metricPair) []string {
	dps := record.metric.Gauge().DataPoints()
	lines := make([]string, 0, dps.Len())

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		line := f.numberDataPointValueLine(
			record.metric.Name(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}

	return lines
}

// doubleSum2Strings converts Sum record to a list of strings (one per dataPoint)
func (f *prometheusFormatter) sum2Strings(record metricPair) []string {
	dps := record.metric.Sum().DataPoints()
	lines := make([]string, 0, dps.Len())

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		line := f.numberDataPointValueLine(
			record.metric.Name(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}

	return lines
}

// summary2Strings converts Summary record to a list of strings
// n+2 where n is number of quantiles and 2 stands for sum and count metrics per each data point
func (f *prometheusFormatter) summary2Strings(record metricPair) []string {
	dps := record.metric.Summary().DataPoints()
	var lines []string

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		qs := dp.QuantileValues()
		additionalAttributes := pcommon.NewMap()
		for i := 0; i < qs.Len(); i++ {
			q := qs.At(i)
			additionalAttributes.UpsertDouble(prometheusQuantileTag, q.Quantile())

			line := f.doubleValueLine(
				record.metric.Name(),
				q.Value(),
				dp,
				f.mergeAttributes(record.attributes, additionalAttributes),
			)
			lines = append(lines, line)
		}

		line := f.doubleValueLine(
			f.sumMetric(record.metric.Name()),
			dp.Sum(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)

		line = f.uintValueLine(
			f.countMetric(record.metric.Name()),
			dp.Count(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}
	return lines
}

// histogram2Strings converts Histogram record to a list of strings,
// (n+1) where n is number of bounds plus two for sum and count per each data point
func (f *prometheusFormatter) histogram2Strings(record metricPair) []string {
	dps := record.metric.Histogram().DataPoints()
	var lines []string

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		explicitBounds := dp.ExplicitBounds()
		if explicitBounds.Len() == 0 {
			continue
		}

		var cumulative uint64
		additionalAttributes := pcommon.NewMap()

		for i := 0; i < explicitBounds.Len(); i++ {
			cumulative += dp.BucketCounts().At(i)
			additionalAttributes.UpsertDouble(prometheusLeTag, explicitBounds.At(i))

			line := f.uintValueLine(
				record.metric.Name(),
				cumulative,
				dp,
				f.mergeAttributes(record.attributes, additionalAttributes),
			)
			lines = append(lines, line)
		}

		cumulative += dp.BucketCounts().At(explicitBounds.Len())
		additionalAttributes.UpsertString(prometheusLeTag, prometheusInfValue)
		line := f.uintValueLine(
			record.metric.Name(),
			cumulative,
			dp,
			f.mergeAttributes(record.attributes, additionalAttributes),
		)
		lines = append(lines, line)

		line = f.doubleValueLine(
			f.sumMetric(record.metric.Name()),
			dp.Sum(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)

		line = f.uintValueLine(
			f.countMetric(record.metric.Name()),
			dp.Count(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}

	return lines
}

// metric2String returns stringified metricPair
func (f *prometheusFormatter) metric2String(record metricPair) string {
	var lines []string

	switch record.metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		lines = f.gauge2Strings(record)
	case pmetric.MetricDataTypeSum:
		lines = f.sum2Strings(record)
	case pmetric.MetricDataTypeSummary:
		lines = f.summary2Strings(record)
	case pmetric.MetricDataTypeHistogram:
		lines = f.histogram2Strings(record)
	}
	return strings.Join(lines, "\n")
}

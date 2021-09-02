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

package sumologicexporter

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
)

type dataPoint interface {
	Timestamp() pdata.Timestamp
	Attributes() pdata.AttributeMap
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

func newPrometheusFormatter() (prometheusFormatter, error) {
	sanitNameRegex, err := regexp.Compile(`[^0-9a-zA-Z]`)
	if err != nil {
		return prometheusFormatter{}, err
	}

	return prometheusFormatter{
		sanitNameRegex: sanitNameRegex,
		replacer:       strings.NewReplacer(`\`, `\\`, `"`, `\"`),
	}, nil
}

// PrometheusLabels returns all attributes as sanitized prometheus labels string
func (f *prometheusFormatter) tags2String(attr pdata.AttributeMap, labels pdata.AttributeMap) prometheusTags {
	mergedAttributes := pdata.NewAttributeMap()
	attr.CopyTo(mergedAttributes)
	labels.Range(func(k string, v pdata.AttributeValue) bool {
		mergedAttributes.UpsertString(k, v.StringVal())
		return true
	})
	length := mergedAttributes.Len()

	if length == 0 {
		return ""
	}

	returnValue := make([]string, 0, length)
	mergedAttributes.Range(func(k string, v pdata.AttributeValue) bool {
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
func (f *prometheusFormatter) doubleLine(name string, attributes prometheusTags, value float64, timestamp pdata.Timestamp) string {
	return fmt.Sprintf(
		"%s%s %g %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pdata.Timestamp(time.Millisecond),
	)
}

// intLine builds metric based on the given arguments where value is int64
func (f *prometheusFormatter) intLine(name string, attributes prometheusTags, value int64, timestamp pdata.Timestamp) string {
	return fmt.Sprintf(
		"%s%s %d %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pdata.Timestamp(time.Millisecond),
	)
}

// uintLine builds metric based on the given arguments where value is uint64
func (f *prometheusFormatter) uintLine(name string, attributes prometheusTags, value uint64, timestamp pdata.Timestamp) string {
	return fmt.Sprintf(
		"%s%s %d %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pdata.Timestamp(time.Millisecond),
	)
}

// doubleValueLine returns prometheus line with given value
func (f *prometheusFormatter) doubleValueLine(name string, value float64, dp dataPoint, attributes pdata.AttributeMap) string {
	return f.doubleLine(
		name,
		f.tags2String(attributes, dp.Attributes()),
		value,
		dp.Timestamp(),
	)
}

// uintValueLine returns prometheus line with given value
func (f *prometheusFormatter) uintValueLine(name string, value uint64, dp dataPoint, attributes pdata.AttributeMap) string {
	return f.uintLine(
		name,
		f.tags2String(attributes, dp.Attributes()),
		value,
		dp.Timestamp(),
	)
}

// numberDataPointValueLine returns prometheus line with value from pdata.NumberDataPoint
func (f *prometheusFormatter) numberDataPointValueLine(name string, dp pdata.NumberDataPoint, attributes pdata.AttributeMap) string {
	switch dp.Type() {
	case pdata.MetricValueTypeDouble:
		return f.doubleValueLine(
			name,
			dp.DoubleVal(),
			dp,
			attributes,
		)
	case pdata.MetricValueTypeInt:
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

// mergeAttributes gets two pdata.AttributeMaps and returns new which contains values from both of them
func (f *prometheusFormatter) mergeAttributes(attributes pdata.AttributeMap, additionalAttributes pdata.AttributeMap) pdata.AttributeMap {
	mergedAttributes := pdata.NewAttributeMap()
	attributes.CopyTo(mergedAttributes)
	additionalAttributes.Range(func(k string, v pdata.AttributeValue) bool {
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
		additionalAttributes := pdata.NewAttributeMap()
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
		var cumulative uint64
		additionalAttributes := pdata.NewAttributeMap()

		for i, bound := range explicitBounds {
			cumulative += dp.BucketCounts()[i]
			additionalAttributes.UpsertDouble(prometheusLeTag, bound)

			line := f.uintValueLine(
				record.metric.Name(),
				cumulative,
				dp,
				f.mergeAttributes(record.attributes, additionalAttributes),
			)
			lines = append(lines, line)
		}

		cumulative += dp.BucketCounts()[len(explicitBounds)]
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
	case pdata.MetricDataTypeGauge:
		lines = f.gauge2Strings(record)
	case pdata.MetricDataTypeSum:
		lines = f.sum2Strings(record)
	case pdata.MetricDataTypeSummary:
		lines = f.summary2Strings(record)
	case pdata.MetricDataTypeHistogram:
		lines = f.histogram2Strings(record)
	}
	return strings.Join(lines, "\n")
}

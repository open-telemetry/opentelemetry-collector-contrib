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

	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
)

type dataPoint interface {
	Timestamp() pdata.TimestampUnixNano
	LabelsMap() pdata.StringMap
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
func (f *prometheusFormatter) tags2String(attr pdata.AttributeMap, labels pdata.StringMap) prometheusTags {
	mergedAttributes := pdata.NewAttributeMap()
	attr.CopyTo(mergedAttributes)
	labels.ForEach(func(k string, v string) {
		mergedAttributes.UpsertString(k, v)
	})
	length := mergedAttributes.Len()

	if length == 0 {
		return ""
	}

	returnValue := make([]string, 0, length)
	mergedAttributes.ForEach(func(k string, v pdata.AttributeValue) {
		returnValue = append(
			returnValue,
			fmt.Sprintf(
				`%s="%s"`,
				f.sanitizeKey(k),
				f.sanitizeValue(tracetranslator.AttributeValueToString(v, false)),
			),
		)
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
func (f *prometheusFormatter) doubleLine(name string, attributes prometheusTags, value float64, timestamp pdata.TimestampUnixNano) string {
	return fmt.Sprintf(
		"%s%s %g %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pdata.TimestampUnixNano(time.Millisecond),
	)
}

// intLine builds metric based on the given arguments where value is int64
func (f *prometheusFormatter) intLine(name string, attributes prometheusTags, value int64, timestamp pdata.TimestampUnixNano) string {
	return fmt.Sprintf(
		"%s%s %d %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pdata.TimestampUnixNano(time.Millisecond),
	)
}

// uintLine builds metric based on the given arguments where value is uint64
func (f *prometheusFormatter) uintLine(name string, attributes prometheusTags, value uint64, timestamp pdata.TimestampUnixNano) string {
	return fmt.Sprintf(
		"%s%s %d %d",
		f.sanitizeKey(name),
		attributes,
		value,
		timestamp/pdata.TimestampUnixNano(time.Millisecond),
	)
}

// doubleValueLine returns prometheus line with given value
func (f *prometheusFormatter) doubleValueLine(name string, value float64, dp dataPoint, attributes pdata.AttributeMap) string {
	return f.doubleLine(
		name,
		f.tags2String(attributes, dp.LabelsMap()),
		value,
		dp.Timestamp(),
	)
}

// intValueLine returns prometheus line with given value
func (f *prometheusFormatter) intValueLine(name string, value int64, dp dataPoint, attributes pdata.AttributeMap) string {
	return f.intLine(
		name,
		f.tags2String(attributes, dp.LabelsMap()),
		value,
		dp.Timestamp(),
	)
}

// uintValueLine returns prometheus line with given value
func (f *prometheusFormatter) uintValueLine(name string, value uint64, dp dataPoint, attributes pdata.AttributeMap) string {
	return f.uintLine(
		name,
		f.tags2String(attributes, dp.LabelsMap()),
		value,
		dp.Timestamp(),
	)
}

// doubleDataPointValueLine returns prometheus line with value from pdata.DoubleDataPoint
func (f *prometheusFormatter) doubleDataPointValueLine(name string, dp pdata.DoubleDataPoint, attributes pdata.AttributeMap) string {
	return f.doubleValueLine(
		name,
		dp.Value(),
		dp,
		attributes,
	)
}

// intDataPointValueLine returns prometheus line with value from pdata.IntDataPoint
func (f *prometheusFormatter) intDataPointValueLine(name string, dp pdata.IntDataPoint, attributes pdata.AttributeMap) string {
	return f.intLine(
		name,
		f.tags2String(attributes, dp.LabelsMap()),
		dp.Value(),
		dp.Timestamp(),
	)
}

// sumMetric returns _sum suffixed metric name
func (f *prometheusFormatter) sumMetric(name string) string {
	return fmt.Sprintf("%s_sum", name)
}

// countMetric returns _count suffixed metric name
func (f *prometheusFormatter) countMetric(name string) string {
	return fmt.Sprintf("%s_count", name)
}

// intGauge2Strings converts IntGauge record to list of strings (one per dataPoint)
func (f *prometheusFormatter) intGauge2Strings(record metricPair) []string {
	dps := record.metric.IntGauge().DataPoints()
	lines := make([]string, 0, dps.Len())

	for i := 0; i < dps.Len(); i++ {
		dp := record.metric.IntGauge().DataPoints().At(i)
		line := f.intDataPointValueLine(
			record.metric.Name(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}
	return lines
}

// mergeAttributes gets two pdata.AttributeMaps and returns new which contains values from both of them
func (f *prometheusFormatter) mergeAttributes(attributes pdata.AttributeMap, additionalAttributes pdata.AttributeMap) pdata.AttributeMap {
	mergedAttributes := pdata.NewAttributeMap()
	attributes.CopyTo(mergedAttributes)
	additionalAttributes.ForEach(func(k string, v pdata.AttributeValue) {
		mergedAttributes.Upsert(k, v)
	})
	return mergedAttributes
}

// doubleGauge2Strings converts DoubleGauge record to a list of strings (one per dataPoint)
func (f *prometheusFormatter) doubleGauge2Strings(record metricPair) []string {
	dps := record.metric.DoubleGauge().DataPoints()
	lines := make([]string, 0, dps.Len())

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		line := f.doubleDataPointValueLine(
			record.metric.Name(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}

	return lines
}

// intSum2Strings converts IntSum record to a list of strings (one per dataPoint)
func (f *prometheusFormatter) intSum2Strings(record metricPair) []string {
	dps := record.metric.IntSum().DataPoints()
	lines := make([]string, 0, dps.Len())

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		line := f.intDataPointValueLine(
			record.metric.Name(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}

	return lines
}

// doubleSum2Strings converts DoubleSum record to a list of strings (one per dataPoint)
func (f *prometheusFormatter) doubleSum2Strings(record metricPair) []string {
	dps := record.metric.DoubleSum().DataPoints()
	lines := make([]string, 0, dps.Len())

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		line := f.doubleDataPointValueLine(
			record.metric.Name(),
			dp,
			record.attributes,
		)
		lines = append(lines, line)
	}

	return lines
}

// doubleSummary2Strings converts DoubleSummary record to a list of strings
// n+2 where n is number of quantiles and 2 stands for sum and count metrics per each data point
func (f *prometheusFormatter) doubleSummary2Strings(record metricPair) []string {
	dps := record.metric.DoubleSummary().DataPoints()
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

// intHistogram2Strings converts IntHistogram record to a list of strings,
// (n+1) where n is number of bounds plus two for sum and count per each data point
func (f *prometheusFormatter) intHistogram2Strings(record metricPair) []string {
	dps := record.metric.IntHistogram().DataPoints()
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

		line = f.intValueLine(
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

// doubleHistogram2Strings converts DoubleHistogram record to a list of strings,
// (n+1) where n is number of bounds plus two for sum and count per each data point
func (f *prometheusFormatter) doubleHistogram2Strings(record metricPair) []string {
	dps := record.metric.DoubleHistogram().DataPoints()
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
	case pdata.MetricDataTypeIntGauge:
		lines = f.intGauge2Strings(record)
	case pdata.MetricDataTypeDoubleGauge:
		lines = f.doubleGauge2Strings(record)
	case pdata.MetricDataTypeIntSum:
		lines = f.intSum2Strings(record)
	case pdata.MetricDataTypeDoubleSum:
		lines = f.doubleSum2Strings(record)
	case pdata.MetricDataTypeDoubleSummary:
		lines = f.doubleSummary2Strings(record)
	case pdata.MetricDataTypeIntHistogram:
		lines = f.intHistogram2Strings(record)
	case pdata.MetricDataTypeDoubleHistogram:
		lines = f.doubleHistogram2Strings(record)
	}
	return strings.Join(lines, "\n")
}

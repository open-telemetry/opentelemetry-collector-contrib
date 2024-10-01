// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"bytes"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	// sanitizedRune is used to replace any invalid char per Carbon format.
	sanitizedRune = '_'

	// Tag related constants per Carbon plaintext protocol.
	tagPrefix                = ";"
	tagKeyValueSeparator     = "="
	tagValueEmptyPlaceholder = "<empty>"
	tagLineEmptySpace        = " "
	tagLineNewLine           = "\n"

	// Constants used when converting from distribution metrics to Carbon format.
	distributionBucketSuffix             = ".bucket"
	distributionUpperBoundTagKey         = "upper_bound"
	distributionUpperBoundTagBeforeValue = tagPrefix + distributionUpperBoundTagKey + tagKeyValueSeparator

	// Constants used when converting from summary metrics to Carbon format.
	summaryQuantileSuffix         = ".quantile"
	summaryQuantileTagKey         = "quantile"
	summaryQuantileTagBeforeValue = tagPrefix + summaryQuantileTagKey + tagKeyValueSeparator

	// Suffix to be added to original metric name for a Carbon metric representing
	// a count metric for either distribution or summary metrics.
	countSuffix = ".count"

	// Textual representation for positive infinity valid in Carbon, ie.:
	// positive infinity as represented in Python.
	infinityCarbonValue = "inf"
)

var writerPool = sync.Pool{
	New: func() any {
		// Start with a buffer of 1KB.
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

// metricDataToPlaintext converts internal metrics data to the Carbon plaintext
// format as defined in https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol)
// and https://graphite.readthedocs.io/en/latest/tags.html#carbon. See details
// below.
//
// Each metric point becomes a single string with the following format:
//
//	"<path> <value> <timestamp>"
//
// The <path> contains the metric name and its tags and has the following,
// format:
//
//	<metric_name>[;tag0;...;tagN]
//
// <metric_name> is the name of the metric and terminates either at the first ';'
// or at the end of the path.
//
// <tag> is of the form "key=val", where key can contain any char except ";!^=" and
// val can contain any char except ";~".
//
// The <value> is the textual representation of the metric value.
//
// The <timestamp> is the Unix time text of when the measurement was made.
//
// The returned values are:
//   - a string concatenating all generated "lines" (each single one representing
//     a single Carbon metric.
//   - number of time series successfully converted to carbon.
//   - number of time series that could not be converted to Carbon.
func metricDataToPlaintext(md pmetric.Metrics) string {
	if md.DataPointCount() == 0 {
		return ""
	}

	buf := writerPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer writerPool.Put(buf)

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				if metric.Name() == "" {
					// TODO: log error info
					continue
				}
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					writeNumberDataPoints(buf, metric.Name(), metric.Gauge().DataPoints())
				case pmetric.MetricTypeSum:
					writeNumberDataPoints(buf, metric.Name(), metric.Sum().DataPoints())
				case pmetric.MetricTypeHistogram:
					formatHistogramDataPoints(buf, metric.Name(), metric.Histogram().DataPoints())
				case pmetric.MetricTypeSummary:
					formatSummaryDataPoints(buf, metric.Name(), metric.Summary().DataPoints())
				}
			}
		}
	}

	return buf.String()
}

func writeNumberDataPoints(buf *bytes.Buffer, metricName string, dps pmetric.NumberDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		var valueStr string
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeEmpty:
			continue // skip this data point - otherwise an empty string will be used as the value and the backend will use the timestamp as the metric value
		case pmetric.NumberDataPointValueTypeInt:
			valueStr = formatInt64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			valueStr = formatFloatForValue(dp.DoubleValue())
		}
		writeLine(
			buf,
			buildPath(metricName, dp.Attributes()),
			valueStr,
			formatTimestamp(dp.Timestamp()))
	}
}

// formatHistogramDataPoints transforms a slice of histogram data points into a series
// of Carbon metrics and injects them into the string builder.
//
// Carbon doesn't have direct support to distribution metrics they will be
// translated into a series of Carbon metrics:
//
// 1. The total count will be represented by a metric named "<metricName>.count".
//
// 2. The total sum will be represented by a metric with the original "<metricName>".
//
// 3. Each histogram bucket is represented by a metric named "<metricName>.bucket"
// and will include a dimension "upper_bound" that specifies the maximum value in
// that bucket. This metric specifies the number of events with a value that is
// less than or equal to the upper bound.
func formatHistogramDataPoints(
	buf *bytes.Buffer,
	metricName string,
	dps pmetric.HistogramDataPointSlice,
) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		timestampStr := formatTimestamp(dp.Timestamp())
		formatCountAndSum(buf, metricName, dp.Attributes(), dp.Count(), dp.Sum(), timestampStr)
		if dp.ExplicitBounds().Len() == 0 {
			continue
		}

		bounds := dp.ExplicitBounds().AsRaw()
		carbonBounds := make([]string, len(bounds)+1)
		for i := 0; i < len(bounds); i++ {
			carbonBounds[i] = formatFloatForLabel(bounds[i])
		}
		carbonBounds[len(carbonBounds)-1] = infinityCarbonValue

		bucketPath := buildPath(metricName+distributionBucketSuffix, dp.Attributes())
		for j := 0; j < dp.BucketCounts().Len(); j++ {
			writeLine(
				buf,
				bucketPath+distributionUpperBoundTagBeforeValue+carbonBounds[j],
				formatUint64(dp.BucketCounts().At(j)),
				timestampStr)
		}
	}
}

// formatSummaryDataPoints transforms a slice of summary data points into a series
// of Carbon metrics and injects them into the string builder.
//
// Carbon doesn't have direct support to summary metrics they will be
// translated into a series of Carbon metrics:
//
// 1. The total count will be represented by a metric named "<metricName>.count".
//
// 2. The total sum will be represented by a metric with the original "<metricName>".
//
// 3. Each quantile is represented by a metric named "<metricName>.quantile"
// and will include a tag key "quantile" that specifies the quantile value.
func formatSummaryDataPoints(
	buf *bytes.Buffer,
	metricName string,
	dps pmetric.SummaryDataPointSlice,
) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		timestampStr := formatTimestamp(dp.Timestamp())
		formatCountAndSum(buf, metricName, dp.Attributes(), dp.Count(), dp.Sum(), timestampStr)

		if dp.QuantileValues().Len() == 0 {
			continue
		}

		quantilePath := buildPath(metricName+summaryQuantileSuffix, dp.Attributes())
		for j := 0; j < dp.QuantileValues().Len(); j++ {
			writeLine(
				buf,
				quantilePath+summaryQuantileTagBeforeValue+formatFloatForLabel(dp.QuantileValues().At(j).Quantile()*100),
				formatFloatForValue(dp.QuantileValues().At(j).Value()),
				timestampStr)
		}
	}
}

// Carbon doesn't have direct support to distribution or summary metrics in both
// cases it needs to create a "count" and a "sum" metric. This function creates
// both, as follows:
//
// 1. The total count will be represented by a metric named "<metricName>.count".
//
// 2. The total sum will be represented by a metruc with the original "<metricName>".
func formatCountAndSum(
	buf *bytes.Buffer,
	metricName string,
	attributes pcommon.Map,
	count uint64,
	sum float64,
	timestampStr string,
) {
	// Write count and sum metrics.
	writeLine(
		buf,
		buildPath(metricName+countSuffix, attributes),
		formatUint64(count),
		timestampStr)

	writeLine(
		buf,
		buildPath(metricName, attributes),
		formatFloatForValue(sum),
		timestampStr)
}

// buildPath is used to build the <metric_path> per description above.
func buildPath(name string, attributes pcommon.Map) string {
	if attributes.Len() == 0 {
		return name
	}

	buf := writerPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer writerPool.Put(buf)

	buf.WriteString(name)
	attributes.Range(func(k string, v pcommon.Value) bool {
		value := v.AsString()
		if value == "" {
			value = tagValueEmptyPlaceholder
		}
		buf.WriteString(tagPrefix)
		buf.WriteString(sanitizeTagKey(k))
		buf.WriteString(tagKeyValueSeparator)
		buf.WriteString(value)
		return true
	})

	return buf.String()
}

// writeLine builds a single Carbon metric textual line, ie.: it already adds
// a new-line character at the end of the string.
func writeLine(buf *bytes.Buffer, path, value, timestamp string) {
	buf.WriteString(path)
	buf.WriteString(tagLineEmptySpace)
	buf.WriteString(value)
	buf.WriteString(tagLineEmptySpace)
	buf.WriteString(timestamp)
	buf.WriteString(tagLineNewLine)
}

// sanitizeTagKey removes any invalid character from the tag key, the invalid
// characters are ";!^=".
func sanitizeTagKey(key string) string {
	mapRune := func(r rune) rune {
		switch r {
		case ';', '!', '^', '=':
			return sanitizedRune
		default:
			return r
		}
	}

	return strings.Map(mapRune, key)
}

// sanitizeTagValue removes any invalid character from the tag value, the invalid
// characters are ";~".
func sanitizeTagValue(value string) string {
	mapRune := func(r rune) rune {
		switch r {
		case ';', '~':
			return sanitizedRune
		default:
			return r
		}
	}

	return strings.Map(mapRune, value)
}

// Formats a float64 per Prometheus label value. This is an attempt to keep other
// the label values with different formats of metrics.
func formatFloatForLabel(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// Formats a float64 per Carbon plaintext format.
func formatFloatForValue(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func formatUint64(i uint64) string {
	return strconv.FormatUint(i, 10)
}

func formatInt64(i int64) string {
	return strconv.FormatInt(i, 10)
}

func formatTimestamp(timestamp pcommon.Timestamp) string {
	return formatUint64(uint64(timestamp) / 1e9)
}

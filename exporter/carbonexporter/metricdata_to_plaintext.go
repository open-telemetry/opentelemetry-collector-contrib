// Copyright 2019, OpenTelemetry Authors
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

package carbonexporter

import (
	"fmt"
	"strconv"
	"strings"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

const (
	// sanitizedRune is used to replace any invalid char per Carbon format.
	sanitizedRune = '_'

	// Tag related constants per Carbon plaintext protocol.
	tagPrefix                 = ";"
	tagKeyValueSeparator      = "="
	tagValueEmptyPlaceholder  = "<empty>"
	tagValueNotSetPlaceholder = "<null>"

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

// metricDataToPlaintext converts internal metrics data to the Carbon plaintext
// format as defined in https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol)
// and https://graphite.readthedocs.io/en/latest/tags.html#carbon. See details
// below.
//
// Each metric point becomes a single string with the following format:
//
// 	"<path> <value> <timestamp>"
//
// The <path> contains the metric name and its tags and has the following,
// format:
//
// 	<metric_name>[;tag0;...;tagN]
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
// 	- a string concatenating all generated "lines" (each single one representing
// 	  a single Carbon metric.
//  - number of time series successfully converted to carbon.
// 	- number of time series that could not be converted to Carbon.
func metricDataToPlaintext(mds []*agentmetricspb.ExportMetricsServiceRequest) (string, int, int) {
	if len(mds) == 0 {
		return "", 0, 0
	}
	var sb strings.Builder
	numTimeseriesDropped := 0
	totalTimeseries := 0

	for _, md := range mds {
		for _, metric := range md.Metrics {
			totalTimeseries++
			descriptor := metric.MetricDescriptor
			name := descriptor.GetName()
			if name == "" {
				numTimeseriesDropped += len(metric.Timeseries)
				// TODO: observability for this, debug logging.
				continue
			}

			tagKeys := buildSanitizedTagKeys(metric.MetricDescriptor.LabelKeys)

			for _, ts := range metric.Timeseries {
				if len(tagKeys) != len(ts.LabelValues) {
					numTimeseriesDropped++
					// TODO: observability with debug, something like the message below:
					//	"inconsistent number of labelKeys(%d) and labelValues(%d) for metric %q",
					//	len(tagKeys),
					//	len(labelValues),
					//	name)

					continue
				}

				// From this point on all code below is safe to assume that
				// len(tagKeys) is equal to len(labelValues).

				for _, point := range ts.Points {
					timestampStr := formatInt64(point.GetTimestamp().GetSeconds())

					switch pv := point.Value.(type) {

					case *metricspb.Point_Int64Value:
						path := buildPath(name, tagKeys, ts.LabelValues)
						valueStr := formatInt64(pv.Int64Value)
						sb.WriteString(buildLine(path, valueStr, timestampStr))

					case *metricspb.Point_DoubleValue:
						path := buildPath(name, tagKeys, ts.LabelValues)
						valueStr := formatFloatForValue(pv.DoubleValue)
						sb.WriteString(buildLine(path, valueStr, timestampStr))

					case *metricspb.Point_DistributionValue:
						err := buildDistributionIntoBuilder(
							&sb, name, tagKeys, ts.LabelValues, timestampStr, pv.DistributionValue)
						if err != nil {
							// TODO: log error info
							numTimeseriesDropped++
						}

					case *metricspb.Point_SummaryValue:
						err := buildSummaryIntoBuilder(
							&sb, name, tagKeys, ts.LabelValues, timestampStr, pv.SummaryValue)
						if err != nil {
							// TODO: log error info
							numTimeseriesDropped++
						}
					}
				}
			}
		}
	}

	return sb.String(), totalTimeseries - numTimeseriesDropped, numTimeseriesDropped
}

// buildDistributionIntoBuilder transforms a metric distribution into a series
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
func buildDistributionIntoBuilder(
	sb *strings.Builder,
	metricName string,
	tagKeys []string,
	labelValues []*metricspb.LabelValue,
	timestampStr string,
	distributionValue *metricspb.DistributionValue,
) error {
	buildCountAndSumIntoBuilder(
		sb,
		metricName,
		tagKeys,
		labelValues,
		distributionValue.GetCount(),
		distributionValue.GetSum(),
		timestampStr)

	explicitBuckets := distributionValue.BucketOptions.GetExplicit()
	if explicitBuckets == nil {
		return fmt.Errorf(
			"unknown bucket options type for metric %q",
			metricName)
	}

	bounds := explicitBuckets.Bounds
	carbonBounds := make([]string, len(bounds)+1)
	for i := 0; i < len(bounds); i++ {
		carbonBounds[i] = formatFloatForLabel(bounds[i])
	}
	carbonBounds[len(carbonBounds)-1] = infinityCarbonValue

	bucketPath := buildPath(metricName+distributionBucketSuffix, tagKeys, labelValues)
	for i, bucket := range distributionValue.Buckets {
		sb.WriteString(buildLine(
			bucketPath+distributionUpperBoundTagBeforeValue+carbonBounds[i],
			formatInt64(bucket.Count),
			timestampStr))
	}

	return nil
}

// buildSummaryIntoBuilder transforms a metric summary into a series
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
func buildSummaryIntoBuilder(
	sb *strings.Builder,
	metricName string,
	tagKeys []string,
	labelValues []*metricspb.LabelValue,
	timestampStr string,
	summaryValue *metricspb.SummaryValue,
) error {
	buildCountAndSumIntoBuilder(
		sb,
		metricName,
		tagKeys,
		labelValues,
		summaryValue.GetCount().GetValue(),
		summaryValue.GetSum().GetValue(),
		timestampStr)

	percentiles := summaryValue.GetSnapshot().GetPercentileValues()
	if percentiles == nil {
		return fmt.Errorf(
			"unknown percentiles values for summary metric %q",
			metricName)
	}

	quantilePath := buildPath(metricName+summaryQuantileSuffix, tagKeys, labelValues)
	for _, quantile := range percentiles {
		sb.WriteString(buildLine(
			quantilePath+summaryQuantileTagBeforeValue+formatFloatForLabel(quantile.GetPercentile()),
			formatFloatForValue(quantile.GetValue()),
			timestampStr))
	}

	return nil
}

// Carbon doesn't have direct support to distribution or summary metrics in both
// cases it needs to create a "count" and a "sum" metric. This function creates
// both, as follows:
//
// 1. The total count will be represented by a metric named "<metricName>.count".
//
// 2. The total sum will be represented by a metruc with the original "<metricName>".
//
func buildCountAndSumIntoBuilder(
	sb *strings.Builder,
	metricName string,
	tagKeys []string,
	labelValues []*metricspb.LabelValue,
	count int64,
	sum float64,
	timestampStr string,
) {
	// Build count and sum metrics.
	countPath := buildPath(metricName+countSuffix, tagKeys, labelValues)
	valueStr := formatInt64(count)
	sb.WriteString(buildLine(countPath, valueStr, timestampStr))

	sumPath := buildPath(metricName, tagKeys, labelValues)
	valueStr = formatFloatForValue(sum)
	sb.WriteString(buildLine(sumPath, valueStr, timestampStr))
}

// buildPath is used to build the <metric_path> per description above. It
// assumes that the caller code already checked that len(tagKeys) is equal to
// len(labelValues) and as such cannot fail to build the path.
func buildPath(
	name string,
	tagKeys []string,
	labelValues []*metricspb.LabelValue,
) string {

	if len(tagKeys) == 0 {
		return name
	}

	var sb strings.Builder
	sb.WriteString(name)

	for i, label := range labelValues {
		value := label.Value

		switch value {
		case "":
			// Per Carbon the value must have length > 1 so put a place holder.
			if label.HasValue {
				value = tagValueEmptyPlaceholder
			} else {
				value = tagValueNotSetPlaceholder
			}
		default:
			value = sanitizeTagValue(value)
		}

		sb.WriteString(tagPrefix + tagKeys[i] + tagKeyValueSeparator + value)
	}

	return sb.String()
}

// buildSanitizedTagKeys builds an slice with the sanitized label keys to be
// used as tag keys on the Carbon metric.
func buildSanitizedTagKeys(labelKeys []*metricspb.LabelKey) []string {
	if len(labelKeys) == 0 {
		return nil
	}

	tagKeys := make([]string, 0, len(labelKeys))
	for _, labelKey := range labelKeys {
		tagKey := sanitizeTagKey(labelKey.Key)
		tagKeys = append(tagKeys, tagKey)
	}

	return tagKeys
}

// buildLine builds a single Carbon metric textual line, ie.: it already adds
// a new-line character at the end of the string.
func buildLine(path, value, timestamp string) string {
	return path + " " + value + " " + timestamp + "\n"
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

func formatInt64(i int64) string {
	return strconv.FormatInt(i, 10)
}

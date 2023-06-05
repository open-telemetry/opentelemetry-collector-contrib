// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver"
)

// WavefrontParser converts metrics in the Wavefront format, see
// https://docs.wavefront.com/wavefront_data_format.html#metrics-data-format-syntax,
// into the internal format of the Collector
type WavefrontParser struct {
	ExtractCollectdTags bool `mapstructure:"extract_collectd_tags"`
}

var _ (protocol.Parser) = (*WavefrontParser)(nil)
var _ (protocol.ParserConfig) = (*WavefrontParser)(nil)

// Only two chars can be espcaped per Wavafront SDK, see
// https://github.com/wavefrontHQ/wavefront-sdk-go/blob/2c5891318fcd83c35c93bba2b411640495473333/senders/formatter.go#L20
var escapedCharReplacer = strings.NewReplacer(
	`\"`, `"`, // Replaces escaped double-quotes
	`\n`, "\n", // Repaces escaped new-line.
)

// BuildParser creates a new Parser instance that receives Wavefront metric data.
func (wp *WavefrontParser) BuildParser() (protocol.Parser, error) {
	return wp, nil
}

// Parse receives the string with Wavefront metric data, and transforms it to
// the collector metric format. See
// https://docs.wavefront.com/wavefront_data_format.html#metrics-data-format-syntax.
//
// Each line received represents a Wavefront metric in the following format:
//
//	"<metricName> <metricValue> [<timestamp>] source=<source> [pointTags]"
//
// Detailed description of each element is available on the link above.
func (wp *WavefrontParser) Parse(line string) (*metricspb.Metric, error) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid wavefront metric [%s]", line)
	}

	metricName := unDoubleQuote(parts[0])
	if metricName == "" {
		return nil, fmt.Errorf("empty name for wavefront metric [%s]", line)
	}
	valueStr := parts[1]
	rest := parts[2]

	var metricType metricspb.MetricDescriptor_Type
	var point metricspb.Point
	if intVal, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		metricType = metricspb.MetricDescriptor_GAUGE_INT64
		point.Value = &metricspb.Point_Int64Value{Int64Value: intVal}
	} else {
		dblVal, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid wavefront metric value [%s]: %w", line, err)
		}
		metricType = metricspb.MetricDescriptor_GAUGE_DOUBLE
		point.Value = &metricspb.Point_DoubleValue{DoubleValue: dblVal}
	}

	parts = strings.SplitN(rest, " ", 2)
	timestampStr := parts[0]
	var tags string
	if len(parts) == 2 {
		tags = parts[1]
	}
	var ts timestamppb.Timestamp
	if unixTime, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		ts.Seconds = unixTime
	} else {
		// Timestamp can be omitted so it is only correct if the string was a tag.
		if strings.IndexByte(timestampStr, '=') == -1 {
			return nil, fmt.Errorf(
				"invalid timestamp for wavefront metric [%s]", line)
		}
		// Assume timestamp was omitted, get current time and adjust index.
		ts.Seconds = time.Now().Unix()
		tags = rest
	}
	point.Timestamp = &ts

	var labelKeys []*metricspb.LabelKey
	var labelValues []*metricspb.LabelValue
	if tags != "" {
		// to need for special treatment for source, treat it as a normal tag since
		// tags are separated by space and are optionally double-quoted.
		var err error
		labelKeys, labelValues, err = buildLabels(tags)
		if err != nil {
			return nil, fmt.Errorf("invalid wavefront metric [%s]: %w", line, err)
		}
	}

	if wp.ExtractCollectdTags {
		metricName, labelKeys, labelValues = wp.injectCollectDLabels(metricName, labelKeys, labelValues)
	}

	metric := &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Type:      metricType,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: labelValues,
				Points:      []*metricspb.Point{&point},
			},
		},
	}
	return metric, nil
}

func (wp *WavefrontParser) injectCollectDLabels(
	metricName string,
	labelKeys []*metricspb.LabelKey,
	labelValues []*metricspb.LabelValue,
) (string, []*metricspb.LabelKey, []*metricspb.LabelValue) {
	// This comes from SignalFx Gateway code that has the capability to
	// remove CollectD tags from the name of the metric.
	var toAddDims map[string]string
	index := strings.Index(metricName, "..")
	for {
		metricName, toAddDims = collectdreceiver.LabelsFromName(&metricName)
		if len(toAddDims) == 0 {
			if index == -1 {
				metricName = strings.ReplaceAll(metricName, "..", ".")
			}

			break
		}

		for k, v := range toAddDims {
			labelKeys = append(labelKeys, &metricspb.LabelKey{Key: k})
			labelValues = append(labelValues, &metricspb.LabelValue{
				Value:    v,
				HasValue: true,
			})
		}
	}
	return metricName, labelKeys, labelValues
}

func buildLabels(tags string) (keys []*metricspb.LabelKey, values []*metricspb.LabelValue, err error) {
	if tags == "" {
		return
	}
	for {
		parts := strings.SplitN(tags, "=", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("failed to break key for [%s]", tags)
		}

		key := parts[0]
		rest := parts[1]
		tagLen := len(key) + 1 // Length of key plus separator and yet to be determined length of the value.
		var value string
		if len(rest) > 1 && rest[0] == '"' {
			// Skip until non-escaped double quote.
			foundEscape := false
			i := 1
			for ; i < len(rest); i++ {
				if rest[i] != '"' && rest[i] != 'n' {
					continue
				}
				isPrevCharEscape := rest[i-1] == '\\'
				if rest[i] == '"' && !isPrevCharEscape {
					// Non-escaped double-quote, it is the end of the value.
					break
				}
				foundEscape = foundEscape || isPrevCharEscape
			}

			value = rest[1:i]
			tagLen += len(value) + 2 // plus 2 to account for the double-quotes.
			if foundEscape {
				// Per implementation of Wavefront SDK only double-quotes and
				// newline characters are escaped. See the link below:
				// https://github.com/wavefrontHQ/wavefront-sdk-go/blob/2c5891318fcd83c35c93bba2b411640495473333/senders/formatter.go#L20
				value = escapedCharReplacer.Replace(value)
			}
		} else {
			// Skip until space.
			i := 0
			for ; i < len(rest) && rest[i] != ' '; i++ { // nolint
			}
			value = rest[:i]
			tagLen += i
		}

		keys = append(keys, &metricspb.LabelKey{Key: key})
		values = append(values, &metricspb.LabelValue{
			Value:    value,
			HasValue: true})

		tags = strings.TrimLeft(tags[tagLen:], " ")
		if tags == "" {
			break
		}
	}

	return
}

func unDoubleQuote(s string) string {
	n := len(s)
	if n < 2 {
		return s
	}

	if s[0] == '"' && s[n-1] == '"' {
		return s[1 : n-1]
	}
	return s
}

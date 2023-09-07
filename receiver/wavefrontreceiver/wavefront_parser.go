// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

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
func (wp *WavefrontParser) Parse(line string) (pmetric.Metric, error) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 3 {
		return pmetric.Metric{}, fmt.Errorf("invalid wavefront metric [%s]", line)
	}

	metricName := unDoubleQuote(parts[0])
	if metricName == "" {
		return pmetric.Metric{}, fmt.Errorf("empty name for wavefront metric [%s]", line)
	}
	valueStr := parts[1]
	rest := parts[2]

	parts = strings.SplitN(rest, " ", 2)
	timestampStr := parts[0]
	var tags string
	if len(parts) == 2 {
		tags = parts[1]
	}
	var ts time.Time
	if unixTime, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		ts = time.Unix(unixTime, 0)
	} else {
		// Timestamp can be omitted so it is only correct if the string was a tag.
		if strings.IndexByte(timestampStr, '=') == -1 {
			return pmetric.Metric{}, fmt.Errorf(
				"invalid timestamp for wavefront metric [%s]", line)
		}
		// Assume timestamp was omitted, get current time and adjust index.
		ts = time.Now()
		tags = rest
	}

	attributes := pcommon.NewMap()
	if tags != "" {
		// to need for special treatment for source, treat it as a normal tag since
		// tags are separated by space and are optionally double-quoted.

		if err := buildLabels(attributes, tags); err != nil {
			return pmetric.Metric{}, fmt.Errorf("invalid wavefront metric [%s]: %w", line, err)
		}
	}

	if wp.ExtractCollectdTags {
		metricName = wp.injectCollectDLabels(metricName, attributes)
	}
	metric := pmetric.NewMetric()
	metric.SetName(metricName)
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	attributes.CopyTo(dp.Attributes())
	if intVal, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		dp.SetIntValue(intVal)
	} else {
		dblVal, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return pmetric.Metric{}, fmt.Errorf("invalid wavefront metric value [%s]: %w", line, err)
		}
		dp.SetDoubleValue(dblVal)
	}

	return metric, nil
}

func (wp *WavefrontParser) injectCollectDLabels(
	metricName string,
	attributes pcommon.Map,
) string {
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
			attributes.PutStr(k, v)
		}
	}
	return metricName
}

func buildLabels(attributes pcommon.Map, tags string) (err error) {
	if tags == "" {
		return
	}
	for {
		parts := strings.SplitN(tags, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("failed to break key for [%s]", tags)
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
		attributes.PutStr(key, value)

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

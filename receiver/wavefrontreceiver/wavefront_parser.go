// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package wavefrontreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver"

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/collectd"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

// wavefrontParser converts metrics in the Wavefront format, see
// https://docs.wavefront.com/wavefront_data_format.html#metrics-data-format-syntax,
// into the internal format of the Collector
type wavefrontParser struct {
	ExtractCollectdTags bool `mapstructure:"extract_collectd_tags"`
}

var (
	_ protocol.Parser       = (*wavefrontParser)(nil)
	_ protocol.ParserConfig = (*wavefrontParser)(nil)
)

// Only two chars can be escaped per Wavefront SDK, see
// https://github.com/wavefrontHQ/wavefront-sdk-go/blob/2c5891318fcd83c35c93bba2b411640495473333/senders/formatter.go#L20
var escapedCharReplacer = strings.NewReplacer(
	`\"`, `"`, // Replaces escaped double-quotes
	`\n`, "\n", // Replaces escaped new-line.
)

// BuildParser creates a new Parser instance that receives Wavefront metric data.
func (wp *wavefrontParser) BuildParser() (protocol.Parser, error) {
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
func (wp *wavefrontParser) Parse(line string) (pmetric.Metric, error) {
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
		// no need for special treatment for source, treat it as a normal tag since
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

func (wp *wavefrontParser) injectCollectDLabels(
	metricName string,
	attributes pcommon.Map,
) string {
	// This comes from SignalFx Gateway code that has the capability to
	// remove CollectD tags from the name of the metric.
	var toAddDims map[string]string
	index := strings.Index(metricName, "..")
	for {
		metricName, toAddDims = collectd.LabelsFromName(&metricName)
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

func buildLabels(attributes pcommon.Map, tags string) error {
	for {
		tags = strings.TrimLeft(tags, " ")
		if len(tags) == 0 {
			return nil
		}

		// First we need to find the key, find first '='
		keyEnd := strings.IndexByte(tags, '=')
		if keyEnd == -1 {
			return fmt.Errorf("failed to break key for [%s]", tags)
		}
		key := tags[:keyEnd]

		tags = tags[keyEnd+1:]
		if len(tags) > 1 && tags[0] == '"' {
			// Quoted value, skip until non-escaped double quote.
			foundEnd := false
			foundEscape := false
			valueEnd := 1
			for ; valueEnd < len(tags); valueEnd++ {
				if tags[valueEnd] != '"' && tags[valueEnd] != 'n' {
					continue
				}
				isPrevCharEscape := tags[valueEnd-1] == '\\'
				if tags[valueEnd] == '"' && !isPrevCharEscape {
					// Non-escaped double-quote, it is the end of the value.
					foundEnd = true
					break
				}
				foundEscape = foundEscape || isPrevCharEscape
			}

			// If we didn't find non-escaped double-quote then this is an error.
			if !foundEnd {
				return errors.New("partially quoted tag value")
			}

			value := tags[1:valueEnd]
			tags = tags[valueEnd+1:]
			if foundEscape {
				// Per implementation of Wavefront SDK only double-quotes and
				// newline characters are escaped. See the link below:
				// https://github.com/wavefrontHQ/wavefront-sdk-go/blob/2c5891318fcd83c35c93bba2b411640495473333/senders/formatter.go#L20
				value = escapedCharReplacer.Replace(value)
			}
			attributes.PutStr(key, value)
		} else {
			valueEnd := strings.IndexByte(tags, ' ')
			if valueEnd == -1 {
				// The value is up to the end.
				attributes.PutStr(key, tags)
				return nil
			}
			attributes.PutStr(key, tags[:valueEnd])
			tags = tags[valueEnd+1:]
		}
	}
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

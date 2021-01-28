// Copyright The OpenTelemetry Authors
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

package serialization

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/consumer/pdata"
)

var (
	reNameDisallowedCharList = regexp.MustCompile("[^A-Za-z0-9.-]+")
)

const (
	maxDimKeyLen = 100
)

// SerializeIntDataPoints serializes a slice of integer datapoints to a Dynatrace gauge.
func SerializeIntDataPoints(name string, data pdata.IntDataPointSlice, tags []string) string {
	// {name} {value} {timestamp}
	output := ""
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)
		tagline := serializeTags(p.LabelsMap(), tags)
		valueLine := strconv.FormatInt(p.Value(), 10)

		output += serializeLine(name, tagline, valueLine, p.Timestamp())
	}

	return output
}

// SerializeDoubleDataPoints serializes a slice of double datapoints to a Dynatrace gauge.
func SerializeDoubleDataPoints(name string, data pdata.DoubleDataPointSlice, tags []string) string {
	// {name} {value} {timestamp}
	output := ""
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)
		output += serializeLine(name, serializeTags(p.LabelsMap(), tags), serializeFloat64(p.Value()), p.Timestamp())
	}

	return output
}

// SerializeDoubleHistogramMetrics serializes a slice of double histogram datapoints to a Dynatrace gauge.
//
// IMPORTANT: Min and max are required by Dynatrace but not provided by histogram so they are assumed to be the average.
func SerializeDoubleHistogramMetrics(name string, data pdata.DoubleHistogramDataPointSlice, tags []string) string {
	// {name} gauge,min=9.75,max=9.75,sum=19.5,count=2 {timestamp_unix_ms}
	output := ""
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)
		tagline := serializeTags(p.LabelsMap(), tags)
		if p.Count() == 0 {
			return ""
		}
		avg := p.Sum() / float64(p.Count())

		valueLine := fmt.Sprintf("gauge,min=%[1]s,max=%[1]s,sum=%s,count=%d", serializeFloat64(avg), serializeFloat64(p.Sum()), p.Count())

		output += serializeLine(name, tagline, valueLine, p.Timestamp())
	}

	return output
}

// SerializeIntHistogramMetrics serializes a slice of integer histogram datapoints to a Dynatrace gauge.
//
// IMPORTANT: Min and max are required by Dynatrace but not provided by histogram so they are assumed to be the average.
func SerializeIntHistogramMetrics(name string, data pdata.IntHistogramDataPointSlice, tags []string) string {
	// {name} gauge,min=9.5,max=9.5,sum=19,count=2 {timestamp_unix_ms}
	output := ""
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)
		tagline := serializeTags(p.LabelsMap(), tags)
		count := p.Count()

		if count == 0 {
			return ""
		}

		avg := float64(p.Sum()) / float64(count)

		valueLine := fmt.Sprintf("gauge,min=%[1]s,max=%[1]s,sum=%d,count=%d", serializeFloat64(avg), p.Sum(), count)

		output += serializeLine(name, tagline, valueLine, p.Timestamp())
	}

	return output
}

func serializeLine(name, tagline, valueline string, timestamp pdata.TimestampUnixNano) string {
	// {metric_name} {tags} {value_line} {timestamp}
	output := name

	if tagline != "" {
		output += "," + tagline
	}

	tsMilli := timestamp / 1_000_000

	output += " " + valueline + " " + strconv.FormatUint(uint64(tsMilli), 10)

	return output
}

func serializeTags(labels pdata.StringMap, exporterTags []string) string {
	tags := append([]string{}, exporterTags...)
	labels.ForEach(func(k string, v string) {
		key, err := NormalizeString(strings.ToLower(k), maxDimKeyLen)
		if err != nil {
			return
		}
		value := escapeDimension(v)
		tag := key + "=" + value
		tags = append(tags, tag)
	})

	tagline := ""

	for _, tag := range tags {
		if tagline != "" {
			tagline += ","
		}
		tagline += tag
	}

	return tagline
}

// Escape dimension values based on the specification at https://www.dynatrace.com/support/help/shortlink/metric-ingestion-protocol#dimension-optional
func escapeDimension(dim string) string {
	return fmt.Sprintf("\"%s\"", strings.ReplaceAll(strings.ReplaceAll(dim, "\"", "\\\""), "\\", "\\\\"))
}

// NormalizeString replaces all non-alphanumerical characters to underscore
func NormalizeString(str string, max int) (normalizedString string, err error) {
	normalizedString = reNameDisallowedCharList.ReplaceAllString(str, "_")

	// Strip Digits if they are at the beginning of the string
	normalizedString = strings.TrimLeft(normalizedString, ".0123456789")

	if len(normalizedString) > max {
		normalizedString = normalizedString[:max]
	}

	for strings.HasSuffix(normalizedString, "_") {
		normalizedString = normalizedString[:len(normalizedString)-1]
	}

	if len(normalizedString) == 0 {
		err = fmt.Errorf("error normalizing the string: %s", str)
	}
	return
}

func serializeFloat64(n float64) string {
	str := strings.TrimRight(strconv.FormatFloat(n, 'f', 6, 64), "0.")
	if str == "" {
		// if everything was trimmed away, number was 0.000000
		return "0"
	}
	return str
}

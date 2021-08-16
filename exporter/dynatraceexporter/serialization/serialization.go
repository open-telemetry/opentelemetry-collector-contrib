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

	"go.opentelemetry.io/collector/model/pdata"
)

var (
	reNameDisallowedCharList = regexp.MustCompile("[^A-Za-z0-9.-]+")
)

const (
	maxDimKeyLen = 100
)

// SerializeNumberDataPoints serializes a slice of number datapoints to a Dynatrace gauge.
func SerializeNumberDataPoints(name string, data pdata.NumberDataPointSlice, tags []string) []string {
	// {name} {value} {timestamp}
	output := []string{}
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)
		var val string
		switch p.Type() {
		case pdata.MetricValueTypeDouble:
			val = serializeFloat64(p.DoubleVal())
		case pdata.MetricValueTypeInt:
			val = strconv.FormatInt(p.IntVal(), 10)
		}
		output = append(output, serializeLine(name, serializeTags(p.Attributes(), tags), val, p.Timestamp()))
	}

	return output
}

// SerializeHistogramMetrics serializes a slice of double histogram datapoints to a Dynatrace gauge.
//
// IMPORTANT: Min and max are required by Dynatrace but not provided by histogram so they are assumed to be the average.
func SerializeHistogramMetrics(name string, data pdata.HistogramDataPointSlice, tags []string) []string {
	// {name} gauge,min=9.75,max=9.75,sum=19.5,count=2 {timestamp_unix_ms}
	output := []string{}
	for i := 0; i < data.Len(); i++ {
		p := data.At(i)
		tagline := serializeTags(p.Attributes(), tags)
		if p.Count() == 0 {
			return []string{}
		}
		avg := p.Sum() / float64(p.Count())

		valueLine := fmt.Sprintf("gauge,min=%[1]s,max=%[1]s,sum=%s,count=%d", serializeFloat64(avg), serializeFloat64(p.Sum()), p.Count())

		output = append(output, serializeLine(name, tagline, valueLine, p.Timestamp()))
	}

	return output
}

func serializeLine(name, tagline, valueline string, timestamp pdata.Timestamp) string {
	// {metric_name} {tags} {value_line} {timestamp}
	output := name

	if tagline != "" {
		output += "," + tagline
	}

	tsMilli := timestamp / 1_000_000

	output += " " + valueline + " " + strconv.FormatUint(uint64(tsMilli), 10)

	return output
}

func serializeTags(labels pdata.AttributeMap, exporterTags []string) string {
	tags := append([]string{}, exporterTags...)
	labels.Range(func(k string, v pdata.AttributeValue) bool {
		key, err := NormalizeString(strings.ToLower(k), maxDimKeyLen)
		if err != nil {
			return true
		}
		value := escapeDimension(v.StringVal()) // TODO(codeboten): Fix as part of https://github.com/open-telemetry/opentelemetry-collector/issues/3815
		tag := key + "=" + value
		tags = append(tags, tag)
		return true
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

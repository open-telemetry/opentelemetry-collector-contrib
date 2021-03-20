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

package protocol

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/otel/attribute"
)

var (
	errEmptyMetricName  = errors.New("empty metric name")
	errEmptyMetricValue = errors.New("empty metric value")
)

func getSupportedTypes() []string {
	return []string{"c", "g"}
}

const TagMetricType = "metric_type"

// StatsDParser supports the Parse method for parsing StatsD messages with Tags.
type StatsDParser struct {
	gauges           map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
	counters         map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
	enableMetricType bool
}

type statsDMetric struct {
	description statsDMetricdescription
	value       string
	intvalue    int64
	floatvalue  float64
	addition    bool
	unit        string
	sampleRate  float64
	labelKeys   []string
	labelValues []string
}

type statsDMetricdescription struct {
	name             string
	statsdMetricType string
	labels           attribute.Distinct
}

func (p *StatsDParser) Initialize(enableMetricType bool) error {
	p.gauges = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)
	p.counters = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)
	p.enableMetricType = enableMetricType
	return nil
}

// get the metrics preparing for flushing and reset the state
func (p *StatsDParser) GetMetrics() pdata.Metrics {
	metrics := pdata.NewMetrics()
	metrics.ResourceMetrics().Resize(1)
	metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Resize(0)

	for _, metric := range p.gauges {
		metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Append(metric)
	}

	for _, metric := range p.counters {
		metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Append(metric)
	}

	p.gauges = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)
	p.counters = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)

	return metrics
}

var timeNowFunc = func() time.Time {
	return time.Now()
}

//aggregate for each metric line
func (p *StatsDParser) Aggregate(line string) error {
	parsedMetric, err := parseMessageToMetric(line, p.enableMetricType)
	if err != nil {
		return err
	}
	switch parsedMetric.description.statsdMetricType {
	case "g":
		_, ok := p.gauges[parsedMetric.description]
		if !ok {
			p.gauges[parsedMetric.description] = buildGaugeMetric(parsedMetric, timeNowFunc())
		} else {
			if parsedMetric.addition {
				savedValue := p.gauges[parsedMetric.description].Metrics().At(0).DoubleGauge().DataPoints().At(0).Value()
				parsedMetric.floatvalue = parsedMetric.floatvalue + savedValue
				p.gauges[parsedMetric.description] = buildGaugeMetric(parsedMetric, timeNowFunc())
			} else {
				p.gauges[parsedMetric.description] = buildGaugeMetric(parsedMetric, timeNowFunc())
			}
		}

	case "c":
		_, ok := p.counters[parsedMetric.description]
		if !ok {
			p.counters[parsedMetric.description] = buildCounterMetric(parsedMetric, timeNowFunc())
		} else {
			savedValue := p.counters[parsedMetric.description].Metrics().At(0).IntSum().DataPoints().At(0).Value()
			parsedMetric.intvalue = parsedMetric.intvalue + savedValue
			p.counters[parsedMetric.description] = buildCounterMetric(parsedMetric, timeNowFunc())
		}
	}

	return nil
}

func parseMessageToMetric(line string, enableMetricType bool) (statsDMetric, error) {
	result := statsDMetric{}

	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return result, fmt.Errorf("invalid message format: %s", line)
	}

	separatorIndex := strings.IndexByte(parts[0], ':')
	if separatorIndex < 0 {
		return result, fmt.Errorf("invalid <name>:<value> format: %s", parts[0])
	}

	result.description.name = parts[0][0:separatorIndex]
	if result.description.name == "" {
		return result, errEmptyMetricName
	}
	result.value = parts[0][separatorIndex+1:]
	if result.value == "" {
		return result, errEmptyMetricValue
	}
	if strings.HasPrefix(result.value, "-") || strings.HasPrefix(result.value, "+") {
		result.addition = true
	}

	result.description.statsdMetricType = parts[1]
	if !contains(getSupportedTypes(), result.description.statsdMetricType) {
		return result, fmt.Errorf("unsupported metric type: %s", result.description.statsdMetricType)
	}

	additionalParts := parts[2:]

	var kvs []attribute.KeyValue
	var sortable attribute.Sortable

	for _, part := range additionalParts {
		// TODO: Sample rate doesn't currently have a place to go in the protocol
		if strings.HasPrefix(part, "@") {
			sampleRateStr := strings.TrimPrefix(part, "@")

			f, err := strconv.ParseFloat(sampleRateStr, 64)
			if err != nil {
				return result, fmt.Errorf("parse sample rate: %s", sampleRateStr)
			}

			result.sampleRate = f
		} else if strings.HasPrefix(part, "#") {
			tagsStr := strings.TrimPrefix(part, "#")

			tagSets := strings.Split(tagsStr, ",")

			for _, tagSet := range tagSets {
				tagParts := strings.Split(tagSet, ":")
				if len(tagParts) != 2 {
					return result, fmt.Errorf("invalid tag format: %s", tagParts)
				}
				result.labelKeys = append(result.labelKeys, tagParts[0])
				result.labelValues = append(result.labelValues, tagParts[1])
				kvs = append(kvs, attribute.String(tagParts[0], tagParts[1]))
			}

		} else {
			return result, fmt.Errorf("unrecognized message part: %s", part)
		}
	}
	switch result.description.statsdMetricType {
	case "g":
		f, err := strconv.ParseFloat(result.value, 64)
		if err != nil {
			return result, fmt.Errorf("gauge: parse metric value string: %s", result.value)
		}
		result.floatvalue = f
	case "c":
		f, err := strconv.ParseFloat(result.value, 64)
		if err != nil {
			return result, fmt.Errorf("counter: parse metric value string: %s", result.value)
		}
		i := int64(f)
		if 0 < result.sampleRate && result.sampleRate < 1 {
			i = int64(f / result.sampleRate)
		}
		result.intvalue = i
	}

	// add metric_type dimension for all metrics

	if enableMetricType {
		var metricType = ""
		switch result.description.statsdMetricType {
		case "g":
			metricType = "gauge"
		case "c":
			metricType = "counter"
		}
		result.labelKeys = append(result.labelKeys, TagMetricType)
		result.labelValues = append(result.labelValues, metricType)

		kvs = append(kvs, attribute.String(TagMetricType, metricType))
	}

	if len(kvs) != 0 {
		set := attribute.NewSetWithSortable(kvs, &sortable)
		result.description.labels = set.Equivalent()
	}

	return result, nil
}

func contains(slice []string, element string) bool {
	for _, val := range slice {
		if val == element {
			return true
		}
	}
	return false
}

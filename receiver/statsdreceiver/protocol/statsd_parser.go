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

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/otel/attribute"
)

var (
	errEmptyMetricName  = errors.New("empty metric name")
	errEmptyMetricValue = errors.New("empty metric value")
)

func getSupportedTypes() []string {
	return []string{"c", "g", "h", "ms"}
}

const (
	tagMetricType   = "metric_type"
	statsdCounter   = "c"
	statsdGauge     = "g"
	statsdHistogram = "h"
	statsdTiming    = "ms"
)

type TimerHistogramMapping struct {
	StatsdType   string `mapstructure:"statsd_type"`
	ObserverType string `mapstructure:"observer_type"`
}

// StatsDParser supports the Parse method for parsing StatsD messages with Tags.
type StatsDParser struct {
	gauges                 map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
	counters               map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics
	summaries              map[statsDMetricdescription]summaryMetric
	timersAndDistributions []pdata.InstrumentationLibraryMetrics
	enableMetricType       bool
	isMonotonicCounter     bool
	observeTimer           string
	observeHistogram       string
}

type summaryMetric struct {
	name          string
	summaryPoints []float64
	labelKeys     []string
	labelValues   []string
	timeNow       time.Time
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

func (p *StatsDParser) Initialize(enableMetricType bool, isMonotonicCounter bool, sendTimerHistogram []TimerHistogramMapping) error {
	p.gauges = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)
	p.counters = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)
	p.timersAndDistributions = make([]pdata.InstrumentationLibraryMetrics, 0)
	p.summaries = make(map[statsDMetricdescription]summaryMetric)

	p.enableMetricType = enableMetricType
	p.isMonotonicCounter = isMonotonicCounter
	for _, eachMap := range sendTimerHistogram {
		switch eachMap.StatsdType {
		case "histogram":
			p.observeHistogram = eachMap.ObserverType
		case "timer", "timing":
			p.observeTimer = eachMap.ObserverType
		}
	}
	return nil
}

// GetMetrics gets the metrics preparing for flushing and reset the state.
func (p *StatsDParser) GetMetrics() pdata.Metrics {
	metrics := pdata.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()

	for _, metric := range p.gauges {
		metric.CopyTo(rm.InstrumentationLibraryMetrics().AppendEmpty())
	}

	for _, metric := range p.counters {
		metric.CopyTo(rm.InstrumentationLibraryMetrics().AppendEmpty())
	}

	for _, metric := range p.timersAndDistributions {
		metric.CopyTo(rm.InstrumentationLibraryMetrics().AppendEmpty())
	}

	for _, summaryMetric := range p.summaries {
		tgt := metrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().AppendEmpty()
		buildSummaryMetric(summaryMetric).CopyTo(tgt)
	}

	p.gauges = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)
	p.counters = make(map[statsDMetricdescription]pdata.InstrumentationLibraryMetrics)
	p.timersAndDistributions = make([]pdata.InstrumentationLibraryMetrics, 0)
	p.summaries = make(map[statsDMetricdescription]summaryMetric)
	return metrics
}

var timeNowFunc = func() time.Time {
	return time.Now()
}

// Aggregate for each metric line.
func (p *StatsDParser) Aggregate(line string) error {
	parsedMetric, err := parseMessageToMetric(line, p.enableMetricType)
	if err != nil {
		return err
	}
	switch parsedMetric.description.statsdMetricType {
	case statsdGauge:
		_, ok := p.gauges[parsedMetric.description]
		if !ok {
			p.gauges[parsedMetric.description] = buildGaugeMetric(parsedMetric, timeNowFunc())
		} else {
			if parsedMetric.addition {
				savedValue := p.gauges[parsedMetric.description].Metrics().At(0).Gauge().DataPoints().At(0).DoubleVal()
				parsedMetric.floatvalue = parsedMetric.floatvalue + savedValue
				p.gauges[parsedMetric.description] = buildGaugeMetric(parsedMetric, timeNowFunc())
			} else {
				p.gauges[parsedMetric.description] = buildGaugeMetric(parsedMetric, timeNowFunc())
			}
		}

	case statsdCounter:
		_, ok := p.counters[parsedMetric.description]
		if !ok {
			p.counters[parsedMetric.description] = buildCounterMetric(parsedMetric, p.isMonotonicCounter, timeNowFunc())
		} else {
			savedValue := p.counters[parsedMetric.description].Metrics().At(0).Sum().DataPoints().At(0).IntVal()
			parsedMetric.intvalue = parsedMetric.intvalue + savedValue
			p.counters[parsedMetric.description] = buildCounterMetric(parsedMetric, p.isMonotonicCounter, timeNowFunc())
		}

	case statsdHistogram:
		switch p.observeHistogram {
		case "gauge":
			p.timersAndDistributions = append(p.timersAndDistributions, buildGaugeMetric(parsedMetric, timeNowFunc()))
		case "summary":
			eachSummaryMetric, ok := p.summaries[parsedMetric.description]
			if !ok {
				p.summaries[parsedMetric.description] = summaryMetric{
					name:          parsedMetric.description.name,
					summaryPoints: []float64{parsedMetric.floatvalue},
					labelKeys:     parsedMetric.labelKeys,
					labelValues:   parsedMetric.labelValues,
					timeNow:       timeNowFunc(),
				}
			} else {
				points := eachSummaryMetric.summaryPoints
				p.summaries[parsedMetric.description] = summaryMetric{
					name:          parsedMetric.description.name,
					summaryPoints: append(points, parsedMetric.floatvalue),
					labelKeys:     parsedMetric.labelKeys,
					labelValues:   parsedMetric.labelValues,
					timeNow:       timeNowFunc(),
				}
			}
		}

	case statsdTiming:
		switch p.observeTimer {
		case "gauge":
			p.timersAndDistributions = append(p.timersAndDistributions, buildGaugeMetric(parsedMetric, timeNowFunc()))
		case "summary":
			eachSummaryMetric, ok := p.summaries[parsedMetric.description]
			if !ok {
				p.summaries[parsedMetric.description] = summaryMetric{
					name:          parsedMetric.description.name,
					summaryPoints: []float64{parsedMetric.floatvalue},
					labelKeys:     parsedMetric.labelKeys,
					labelValues:   parsedMetric.labelValues,
					timeNow:       timeNowFunc(),
				}
			} else {
				points := eachSummaryMetric.summaryPoints
				p.summaries[parsedMetric.description] = summaryMetric{
					name:          parsedMetric.description.name,
					summaryPoints: append(points, parsedMetric.floatvalue),
					labelKeys:     parsedMetric.labelKeys,
					labelValues:   parsedMetric.labelValues,
					timeNow:       timeNowFunc(),
				}
			}
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
	if !Contains(getSupportedTypes(), result.description.statsdMetricType) {
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
	case statsdGauge:
		f, err := strconv.ParseFloat(result.value, 64)
		if err != nil {
			return result, fmt.Errorf("gauge: parse metric value string: %s", result.value)
		}
		result.floatvalue = f
	case statsdCounter:
		f, err := strconv.ParseFloat(result.value, 64)
		if err != nil {
			return result, fmt.Errorf("counter: parse metric value string: %s", result.value)
		}
		i := int64(f)
		if 0 < result.sampleRate && result.sampleRate < 1 {
			i = int64(f / result.sampleRate)
		}
		result.intvalue = i
	case statsdHistogram, statsdTiming:
		f, err := strconv.ParseFloat(result.value, 64)
		if err != nil {
			return result, fmt.Errorf("timing/histogram: parse metric value string: %s", result.value)
		}
		if 0 < result.sampleRate && result.sampleRate < 1 {
			f = f / result.sampleRate
		}
		result.floatvalue = f
	}

	// add metric_type dimension for all metrics

	if enableMetricType {
		var metricType = ""
		switch result.description.statsdMetricType {
		case statsdGauge:
			metricType = "gauge"
		case statsdCounter:
			metricType = "counter"
		case statsdTiming:
			metricType = "timing"
		case statsdHistogram:
			metricType = "histogram"
		}
		result.labelKeys = append(result.labelKeys, tagMetricType)
		result.labelValues = append(result.labelValues, metricType)

		kvs = append(kvs, attribute.String(tagMetricType, metricType))
	}

	if len(kvs) != 0 {
		set := attribute.NewSetWithSortable(kvs, &sortable)
		result.description.labels = set.Equivalent()
	}

	return result, nil
}

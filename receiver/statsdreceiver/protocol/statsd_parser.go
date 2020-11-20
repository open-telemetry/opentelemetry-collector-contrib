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

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var gauges map[string]*metricspb.Metric
var counters map[string]*metricspb.Metric

var (
	errEmptyMetricName  = errors.New("empty metric name")
	errEmptyMetricValue = errors.New("empty metric value")
)

func getSupportedTypes() []string {
	return []string{"c", "g"}
}

// StatsDParser supports the Parse method for parsing StatsD messages with Tags.
type StatsDParser struct{}

type statsDMetric struct {
	name             string
	value            string
	intvalue         int64
	floatvalue       float64
	hash             string
	addition         bool
	statsdMetricType string
	unit             string
	metricType       metricspb.MetricDescriptor_Type
	sampleRate       float64
	labelKeys        []*metricspb.LabelKey
	labelValues      []*metricspb.LabelValue
}

func (p *StatsDParser) Initialize() error {
	gauges = make(map[string]*metricspb.Metric)
	counters = make(map[string]*metricspb.Metric)
	return nil
}

// get the metrics preparing for flushing and reset the state
func (p *StatsDParser) GetMetrics() []*metricspb.Metric {
	var metrics []*metricspb.Metric

	for _, metric := range gauges {
		metrics = append(metrics, metric)
	}

	for _, metric := range counters {
		metrics = append(metrics, metric)
	}

	gauges = make(map[string]*metricspb.Metric)
	counters = make(map[string]*metricspb.Metric)

	return metrics
}

var timeNowFunc = func() int64 {
	return time.Now().Unix()
}

//aggregate for each metric line
func (p *StatsDParser) Aggregate(line string) error {
	parsedMetric, err := parseMessageToMetric(line)
	if err != nil {
		return err
	}
	switch parsedMetric.statsdMetricType {
	case "g":
		_, ok := gauges[parsedMetric.hash]
		if !ok {
			metricPoint := buildPoint(parsedMetric)
			gauges[parsedMetric.hash] = buildMetric(parsedMetric, metricPoint)
		} else {
			if parsedMetric.addition {
				savedValue := gauges[parsedMetric.hash].GetTimeseries()[0].Points[0].GetDoubleValue()
				parsedMetric.floatvalue = parsedMetric.floatvalue + savedValue
				metricPoint := buildPoint(parsedMetric)
				gauges[parsedMetric.hash] = buildMetric(parsedMetric, metricPoint)
			} else {
				metricPoint := buildPoint(parsedMetric)
				gauges[parsedMetric.hash] = buildMetric(parsedMetric, metricPoint)
			}
		}

	case "c":
		_, ok := counters[parsedMetric.hash]
		if !ok {
			metricPoint := buildPoint(parsedMetric)
			counters[parsedMetric.hash] = buildMetric(parsedMetric, metricPoint)
		} else {
			savedValue := counters[parsedMetric.hash].GetTimeseries()[0].Points[0].GetInt64Value()
			parsedMetric.intvalue = parsedMetric.intvalue + savedValue
			metricPoint := buildPoint(parsedMetric)
			counters[parsedMetric.hash] = buildMetric(parsedMetric, metricPoint)
		}
	}

	return nil
}

func parseMessageToMetric(line string) (*statsDMetric, error) {
	result := &statsDMetric{}

	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid message format: %s", line)
	}

	separatorIndex := strings.IndexByte(parts[0], ':')
	if separatorIndex < 0 {
		return nil, fmt.Errorf("invalid <name>:<value> format: %s", parts[0])
	}

	result.name = parts[0][0:separatorIndex]
	if result.name == "" {
		return nil, errEmptyMetricName
	}
	result.value = parts[0][separatorIndex+1:]
	if result.value == "" {
		return nil, errEmptyMetricValue
	}
	if strings.HasPrefix(result.value, "-") || strings.HasPrefix(result.value, "+") {
		result.addition = true
	}

	result.statsdMetricType = parts[1]
	if !contains(getSupportedTypes(), result.statsdMetricType) {
		return nil, fmt.Errorf("unsupported metric type: %s", result.statsdMetricType)
	}

	additionalParts := parts[2:]
	for _, part := range additionalParts {
		// TODO: Sample rate doesn't currently have a place to go in the protocol
		if strings.HasPrefix(part, "@") {
			sampleRateStr := strings.TrimPrefix(part, "@")

			f, err := strconv.ParseFloat(sampleRateStr, 64)
			if err != nil {
				return nil, fmt.Errorf("parse sample rate: %s", sampleRateStr)
			}

			result.sampleRate = f
		} else if strings.HasPrefix(part, "#") {
			tagsStr := strings.TrimPrefix(part, "#")

			tagSets := strings.Split(tagsStr, ",")

			result.labelKeys = make([]*metricspb.LabelKey, 0, len(tagSets))
			result.labelValues = make([]*metricspb.LabelValue, 0, len(tagSets))

			for _, tagSet := range tagSets {
				tagParts := strings.Split(tagSet, ":")
				if len(tagParts) != 2 {
					return nil, fmt.Errorf("invalid tag format: %s", tagParts)
				}
				result.labelKeys = append(result.labelKeys, &metricspb.LabelKey{Key: tagParts[0]})
				result.labelValues = append(result.labelValues, &metricspb.LabelValue{
					Value:    tagParts[1],
					HasValue: true,
				})
			}
		} else {
			return nil, fmt.Errorf("unrecognized message part: %s", part)
		}
	}

	switch result.statsdMetricType {
	case "g":
		f, err := strconv.ParseFloat(result.value, 64)
		if err != nil {
			return nil, fmt.Errorf("gauge: parse metric value string: %s", result.value)
		}
		result.floatvalue = f
		result.metricType = metricspb.MetricDescriptor_GAUGE_DOUBLE
	case "c":
		f, err := strconv.ParseFloat(result.value, 64)
		if err != nil {
			return nil, fmt.Errorf("counter: parse metric value string: %s", result.value)
		}
		i := int64(f)
		if 0 < result.sampleRate && result.sampleRate < 1 {
			i = int64(f / result.sampleRate)
		}
		result.intvalue = i
		result.metricType = metricspb.MetricDescriptor_GAUGE_INT64
	}

	var tg []string
	for n, k := range result.labelKeys {
		if result.labelValues[n].HasValue {
			tg = append(tg, k.Key+"="+result.labelValues[n].Value)
		}
	}
	result.hash = result.name + strings.Join(tg, "")
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

func buildMetric(metric *statsDMetric, point *metricspb.Point) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metric.name,
			Type:      metric.metricType,
			LabelKeys: metric.labelKeys,
			Unit:      metric.unit,
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				LabelValues: metric.labelValues,
				Points: []*metricspb.Point{
					point,
				},
			},
		},
	}
}

func buildPoint(parsedMetric *statsDMetric) *metricspb.Point {
	now := &timestamppb.Timestamp{
		Seconds: timeNowFunc(),
	}

	switch parsedMetric.statsdMetricType {
	case "c":
		return buildCounterPoint(parsedMetric, now)
	case "g":
		return buildGaugePoint(parsedMetric, now)
	}

	return nil
}

func buildCounterPoint(parsedMetric *statsDMetric, now *timestamppb.Timestamp) *metricspb.Point {
	point := &metricspb.Point{
		Timestamp: now,
		Value: &metricspb.Point_Int64Value{
			Int64Value: parsedMetric.intvalue,
		},
	}
	return point
}

func buildGaugePoint(parsedMetric *statsDMetric, now *timestamppb.Timestamp) *metricspb.Point {
	point := &metricspb.Point{
		Timestamp: now,
		Value: &metricspb.Point_DoubleValue{
			DoubleValue: parsedMetric.floatvalue,
		},
	}
	return point
}

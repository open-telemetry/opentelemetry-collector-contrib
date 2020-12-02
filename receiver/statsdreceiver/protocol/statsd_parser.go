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

var (
	errEmptyMetricName  = errors.New("empty metric name")
	errEmptyMetricValue = errors.New("empty metric value")
)

func getSupportedTypes() []string {
	return []string{"c", "g", "ms"}
}

// StatsDParser supports the Parse method for parsing StatsD messages with Tags.
type StatsDParser struct{}

type statsDMetric struct {
	name             string
	value            string
	statsdMetricType string
	unit             string
	metricType       metricspb.MetricDescriptor_Type
	sampleRate       float64
	labelKeys        []*metricspb.LabelKey
	labelValues      []*metricspb.LabelValue
}

var timeNowFunc = func() int64 {
	return time.Now().Unix()
}

// Parse returns an OTLP metric representation of the input StatsD string.
func (p *StatsDParser) Parse(line string) (*metricspb.Metric, error) {
	parsedMetric, err := parseMessageToMetric(line)
	if err != nil {
		return nil, err
	}

	metricPoint, err := buildPoint(parsedMetric)
	if err != nil {
		return nil, err
	}

	return buildMetric(parsedMetric, metricPoint), nil
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

func buildPoint(parsedMetric *statsDMetric) (*metricspb.Point, error) {
	now := &timestamppb.Timestamp{
		Seconds: timeNowFunc(),
	}

	switch parsedMetric.statsdMetricType {
	case "c":
		return buildCounterPoint(parsedMetric, now, func(parsedMetric *statsDMetric) {
			parsedMetric.metricType = metricspb.MetricDescriptor_CUMULATIVE_INT64
		})
	case "g":
		return buildGaugePoint(parsedMetric, now, func(parsedMetric *statsDMetric) {
			parsedMetric.metricType = metricspb.MetricDescriptor_GAUGE_DOUBLE
		})
	case "ms":
		return buildTimerPoint(parsedMetric, now, func(parsedMetric *statsDMetric) {
			parsedMetric.metricType = metricspb.MetricDescriptor_GAUGE_DOUBLE
		})
	}

	return nil, fmt.Errorf("unhandled metric type: %s", parsedMetric.statsdMetricType)
}

func buildCounterPoint(parsedMetric *statsDMetric, now *timestamppb.Timestamp,
	metricTypeSetter func(parsedMetric *statsDMetric)) (*metricspb.Point, error) {
	var point *metricspb.Point
	var i int64

	f, err := strconv.ParseFloat(parsedMetric.value, 64)
	if err != nil {
		return nil, fmt.Errorf("counter: parse metric value string: %s", parsedMetric.value)
	}
	i = int64(f)
	if 0 < parsedMetric.sampleRate && parsedMetric.sampleRate < 1 {
		i = int64(f / parsedMetric.sampleRate)
	}
	point = &metricspb.Point{
		Timestamp: now,
		Value: &metricspb.Point_Int64Value{
			Int64Value: i,
		},
	}
	metricTypeSetter(parsedMetric)
	return point, nil
}

func buildGaugePoint(parsedMetric *statsDMetric, now *timestamppb.Timestamp,
	metricTypeSetter func(parsedMetric *statsDMetric)) (*metricspb.Point, error) {
	var point *metricspb.Point

	f, err := strconv.ParseFloat(parsedMetric.value, 64)
	if err != nil {
		return nil, fmt.Errorf("gauge: parse metric value string: %s", parsedMetric.value)
	}
	point = &metricspb.Point{
		Timestamp: now,
		Value: &metricspb.Point_DoubleValue{
			DoubleValue: f,
		},
	}
	metricTypeSetter(parsedMetric)
	return point, nil
}

func buildTimerPoint(parsedMetric *statsDMetric, now *timestamppb.Timestamp, metricTypeSetter func(parsedMetric *statsDMetric)) (*metricspb.Point, error) {
	var point *metricspb.Point
	parsedMetric.unit = "ms"
	f, err := strconv.ParseFloat(parsedMetric.value, 64)
	if err != nil {
		return nil, fmt.Errorf("timer: failed to parse metric value to float: %s", parsedMetric.value)
	}
	point = &metricspb.Point{
		Timestamp: now,
		Value: &metricspb.Point_DoubleValue{
			DoubleValue: f,
		},
	}
	metricTypeSetter(parsedMetric)
	return point, nil
}

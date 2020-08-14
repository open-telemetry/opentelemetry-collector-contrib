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
	"github.com/golang/protobuf/ptypes/timestamp"
)

var (
	errEmptyMetricName  = errors.New("empty metric name")
	errEmptyMetricValue = errors.New("empty metric value")
)

// StatsDParser supports the Parse method for parsing StatsD messages with Tags.
type StatsDParser struct{}

// Parse returns an OTLP metric representation of the input StatsD string.
func (p *StatsDParser) Parse(line string) (*metricspb.Metric, error) {
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid message format: %s", line)
	}

	separatorIndex := strings.IndexByte(parts[0], ':')
	if separatorIndex < 0 {
		return nil, fmt.Errorf("invalid <name>:<value> format: %s", parts[0])
	}

	metricName := parts[0][0:separatorIndex]
	if metricName == "" {
		return nil, errEmptyMetricName
	}
	metricValueString := parts[0][separatorIndex+1:]
	if metricValueString == "" {
		return nil, errEmptyMetricValue
	}

	metricType := parts[1]

	// TODO: add sample rate and tag parsing

	metricPoint, err := buildPoint(metricType, metricValueString)
	if err != nil {
		return nil, err
	}

	return buildMetric(metricName, metricPoint), nil
}

func buildMetric(metricName string, point *metricspb.Point) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: metricName,
		},
		Timeseries: []*metricspb.TimeSeries{
			{
				Points: []*metricspb.Point{
					point,
				},
			},
		},
	}
}

func buildPoint(metricType, metricValue string) (*metricspb.Point, error) {
	point := &metricspb.Point{
		Timestamp: &timestamp.Timestamp{
			Seconds: time.Now().Unix(),
		},
	}

	switch metricType {
	case "c":
		// TODO: support both Int64 and Double values.
		i, err := strconv.ParseInt(metricValue, 10, 64)
		if err != nil {
			f, err := strconv.ParseFloat(metricValue, 64)
			if err != nil {
				return nil, fmt.Errorf("parse metric value string: %s", metricValue)
			}
			i = int64(f)
		}
		point.Value = &metricspb.Point_Int64Value{
			Int64Value: i,
		}
	default:
		return nil, fmt.Errorf("unhandled metric type: %s", metricType)
	}

	return point, nil
}

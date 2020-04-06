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

package redisreceiver

import (
	"fmt"
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// An intermediate data type that allows us to define at startup which metrics to
// convert (from the string-string map we get from redisSvc) and how to convert them.
type redisMetric struct {
	key               string
	name              string
	units             string
	desc              string
	labels            map[string]string
	labelDescriptions map[string]string
	mdType            metricspb.MetricDescriptor_Type
}

// Parse a numeric string to build a proto Metric based on this redisMetric. The
// passed-in time is applied to the Point.
func (m *redisMetric) parseMetric(strVal string, t *timeBundle) (*metricspb.Metric, error) {
	pt, err := m.parsePoint(strVal)
	if err != nil {
		return nil, err
	}
	pbMetric := newProtoMetric(m, pt, t)
	return pbMetric, nil
}

// Parse a numeric string to build a Point.
func (m *redisMetric) parsePoint(strVal string) (*metricspb.Point, error) {
	switch m.mdType {
	case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
		return strToInt64Point(strVal)
	case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
		return strToDoublePoint(strVal)
	}
	// The error below should never happen because types are confined to getDefaultRedisMetrics().
	// If there's a change and it does, this error will cause a test failure in TestAllMetrics
	// which expects no errors/warnings, and in TestRedisRunnable which expects the number of input
	// metrics to equal the number of output metrics. If there's a change to the tests and an
	// unexpected type is found, the only effect will be that that one metric will be missing
	// from the output, in which case the error below is treated as a warning and is logged. Metrics
	// collection shouldn't be adversely affected.
	return nil, fmt.Errorf("unexpected point type %v", m.mdType)
}

// Converts a numeric whole number string to a Point.
func strToInt64Point(s string) (*metricspb.Point, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: i}}, nil
}

// Converts a numeric floating point string to a Point.
func strToDoublePoint(s string) (*metricspb.Point, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return &metricspb.Point{Value: &metricspb.Point_DoubleValue{DoubleValue: f}}, nil
}

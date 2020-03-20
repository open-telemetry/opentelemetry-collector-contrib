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
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

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

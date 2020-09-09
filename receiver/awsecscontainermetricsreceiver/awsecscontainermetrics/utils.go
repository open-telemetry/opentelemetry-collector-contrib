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

package awsecscontainermetrics

import (
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
)

func timestampProto(t time.Time) *timestamp.Timestamp {
	out, _ := ptypes.TimestampProto(t)
	return out
}

func applyTimeStamp(metrics []*metricspb.Metric, t *timestamp.Timestamp) []*metricspb.Metric {
	for _, metric := range metrics {
		if metric != nil {
			metric.Timeseries[0].Points[0].Timestamp = t
		}
	}
	return metrics
}

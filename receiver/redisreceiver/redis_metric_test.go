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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildSingleProtoMetric_PointTimestamp(t *testing.T) {
	now := time.Now()
	uptimeMetric := uptimeInSeconds()
	metric, err := uptimeMetric.parseMetric("42", newTimeBundle(now, 100))
	require.Nil(t, err)
	ts := metric.Timeseries[0]
	pt := ts.Points[0]
	require.Equal(t, now.Unix(), pt.Timestamp.GetSeconds())
	nanosecond := now.Nanosecond()
	require.Equal(t, int32(nanosecond), pt.Timestamp.GetNanos())
}

func TestBuildSingleProtoMetric_Labels(t *testing.T) {
	cpuMetric := usedCPUSys()
	metric, err := cpuMetric.parseMetric("42", newTimeBundle(time.Now(), 100))
	require.Nil(t, err)
	require.Equal(t, 1, len(metric.MetricDescriptor.LabelKeys))
	key := metric.MetricDescriptor.LabelKeys[0].Key
	require.Equal(t, "state", key)
	require.Equal(t, 1, len(metric.Timeseries[0].LabelValues))
	require.Equal(t, "sys", metric.Timeseries[0].LabelValues[0].Value)
}

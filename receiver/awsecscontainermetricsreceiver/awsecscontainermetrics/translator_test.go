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
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
)

func TestConvertToOTMetrics(t *testing.T) {
	timestamp := timestampProto(time.Now())
	m := ECSMetrics{}

	m.MemoryUsage = 100
	m.MemoryMaxUsage = 100
	m.MemoryUtilized = 100
	m.MemoryReserved = 100
	m.CPUTotalUsage = 100

	labelKeys := []*metricspb.LabelKey{
		&metricspb.LabelKey{Key: "label_key_1"},
	}
	labelValues := []*metricspb.LabelValue{
		&metricspb.LabelValue{Value: "label_value_1"},
	}

	metrics := convertToOTMetrics("container.", m, labelKeys, labelValues, timestamp)
	require.EqualValues(t, 25, len(metrics))
}

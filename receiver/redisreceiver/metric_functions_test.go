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
	"strings"
	"testing"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
)

func TestDefaultMetrics(t *testing.T) {
	for _, metric := range getDefaultRedisMetrics() {
		require.True(t, len(metric.key) > 0)
		require.True(t, len(metric.name) > 0)
		require.True(t, strings.HasPrefix(metric.name, "redis/"))
		require.True(
			t,
			metric.mdType == v1.MetricDescriptor_GAUGE_INT64 ||
				metric.mdType == v1.MetricDescriptor_GAUGE_DOUBLE ||
				metric.mdType == v1.MetricDescriptor_CUMULATIVE_INT64 ||
				metric.mdType == v1.MetricDescriptor_CUMULATIVE_DOUBLE,
		)
	}
}

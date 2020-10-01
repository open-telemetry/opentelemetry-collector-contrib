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

func TestGenerateDummyMetrics(t *testing.T) {
	md := GenerateDummyMetrics()

	require.EqualValues(t, 2, len(md.Metrics))
}

func TestCreateGaugeIntMetric(t *testing.T) {
	m := createGaugeIntMetric(100)

	require.EqualValues(t, 1, len(m.Timeseries))
}

func TestTimestampProto(t *testing.T) {
	timestamp := timestampProto(time.Now())

	require.NotNil(t, timestamp)
}

func TestApplyTimestamp(t *testing.T) {
	timestamp := timestampProto(time.Now())
	m := []*metricspb.Metric{
		createGaugeIntMetric(1),
	}

	metrics := applyTimestamp(m, timestamp)

	require.NotNil(t, metrics)
	require.EqualValues(t, timestamp, metrics[0].Timeseries[0].Points[0].Timestamp)
}

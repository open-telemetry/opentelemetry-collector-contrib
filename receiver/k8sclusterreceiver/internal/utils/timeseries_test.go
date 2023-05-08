// Copyright The OpenTelemetry Authors
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

package utils

import (
	"testing"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
)

func TestGetInt64TimeSeries(t *testing.T) {
	dpVal := int64(10)
	ts := GetInt64TimeSeries(dpVal)

	require.Equal(t, dpVal, ts.Points[0].GetInt64Value())
}

func TestGetInt64TimeSeriesWithLabels(t *testing.T) {
	dpVal := int64(10)
	labelVals := []*v1.LabelValue{{Value: "value1"}, {Value: "value2"}}

	ts := GetInt64TimeSeriesWithLabels(dpVal, labelVals)

	require.Equal(t, dpVal, ts.Points[0].GetInt64Value())
	require.Equal(t, labelVals, ts.LabelValues)
}

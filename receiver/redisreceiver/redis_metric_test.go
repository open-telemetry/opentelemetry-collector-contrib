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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestParseMetric_PointTimestamp(t *testing.T) {
	now := time.Now()
	uptimeMetric := uptimeInSeconds()
	pdm, err := uptimeMetric.parseMetric("42", newTimeBundle(now, 100))
	require.NoError(t, err)

	pt := pdm.Sum().DataPoints().At(0)
	ptTime := pt.Timestamp()

	assert.EqualValues(t, now.UnixNano(), int64(ptTime))
}

func TestParseMetric_Labels(t *testing.T) {
	cpuMetric := usedCPUSys()
	pdm, err := cpuMetric.parseMetric("42", newTimeBundle(time.Now(), 100))
	require.NoError(t, err)

	pt := pdm.Sum().DataPoints().At(0)
	attributesMap := pt.Attributes()
	l := attributesMap.Len()
	assert.Equal(t, 1, l)
	state, ok := attributesMap.Get("state")
	assert.True(t, ok)
	assert.Equal(t, "sys", state.StringVal())
}

func TestParseMetric_Errors(t *testing.T) {
	for _, dataType := range []pdata.MetricDataType{
		pdata.MetricDataTypeSum,
		pdata.MetricDataTypeGauge,
	} {
		for _, valueType := range []pdata.MetricValueType{
			pdata.MetricValueTypeDouble,
			pdata.MetricValueTypeInt,
		} {
			m := redisMetric{pdType: dataType, valueType: valueType}
			_, err := m.parseMetric("foo", &timeBundle{})
			assert.Error(t, err)
		}
	}
}

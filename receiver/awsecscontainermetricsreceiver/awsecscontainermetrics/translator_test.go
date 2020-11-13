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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestConvertToOTMetrics(t *testing.T) {
	timestamp := pdata.TimeToUnixNano(time.Now())
	m := ECSMetrics{}

	m.MemoryUsage = 100
	m.MemoryMaxUsage = 100
	m.MemoryUtilized = 100
	m.MemoryReserved = 100
	m.CPUTotalUsage = 100

	resource := pdata.NewResource()
	rms := convertToOTLPMetrics("container.", m, resource, timestamp)
	require.EqualValues(t, 26, rms.At(0).InstrumentationLibraryMetrics().Len())
}

func TestIntGauge(t *testing.T) {
	intValue := int64(100)
	timestamp := pdata.TimeToUnixNano(time.Now())

	ilm := intGauge("cpu_utilized", "Count", intValue, timestamp)
	require.NotNil(t, ilm)
}

func TestDoubleGauge(t *testing.T) {
	timestamp := pdata.TimeToUnixNano(time.Now())
	floatValue := 100.01

	m := doubleGauge("cpu_utilized", "Count", floatValue, timestamp)
	require.NotNil(t, m)
}

func TestIntSum(t *testing.T) {
	timestamp := pdata.TimeToUnixNano(time.Now())
	intValue := int64(100)

	m := intSum("cpu_utilized", "Count", intValue, timestamp)
	require.NotNil(t, m)
}

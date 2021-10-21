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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

func TestConvertToOTMetrics(t *testing.T) {
	timestamp := pdata.NewTimestampFromTime(time.Now())
	m := ECSMetrics{}

	m.MemoryUsage = 100
	m.MemoryMaxUsage = 100
	m.MemoryUtilized = 100
	m.MemoryReserved = 100
	m.CPUTotalUsage = 100

	resource := pdata.NewResource()
	md := convertToOTLPMetrics("container.", m, resource, timestamp)
	require.EqualValues(t, 26, md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Len())
	assert.EqualValues(t, conventions.SchemaURL, md.ResourceMetrics().At(0).SchemaUrl())
}

func TestIntGauge(t *testing.T) {
	intValue := int64(100)
	timestamp := pdata.NewTimestampFromTime(time.Now())

	ilm := pdata.NewInstrumentationLibraryMetrics()
	appendIntGauge("cpu_utilized", "Count", intValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestDoubleGauge(t *testing.T) {
	timestamp := pdata.NewTimestampFromTime(time.Now())
	floatValue := 100.01

	ilm := pdata.NewInstrumentationLibraryMetrics()
	appendDoubleGauge("cpu_utilized", "Count", floatValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestIntSum(t *testing.T) {
	timestamp := pdata.NewTimestampFromTime(time.Now())
	intValue := int64(100)

	ilm := pdata.NewInstrumentationLibraryMetrics()
	appendIntSum("cpu_utilized", "Count", intValue, timestamp, ilm)
	require.NotNil(t, ilm)
}

func TestConvertStoppedContainerDataToOTMetrics(t *testing.T) {
	timestamp := pdata.NewTimestampFromTime(time.Now())
	resource := pdata.NewResource()
	duration := 1200000000.32132
	md := convertStoppedContainerDataToOTMetrics("container.", resource, timestamp, duration)
	require.EqualValues(t, 1, md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Len())
}

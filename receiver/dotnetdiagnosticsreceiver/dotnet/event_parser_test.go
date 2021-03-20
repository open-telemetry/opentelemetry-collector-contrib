// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnet

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

func TestEventParser(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 4)
	require.NoError(t, err)
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err = reader.Seek(1131)
	require.NoError(t, err)
	metrics, err := parseEventBlock(reader, fms())
	require.NoError(t, err)
	assert.Equal(t, 19, len(metrics))
	testCPUUsage(t, metrics[0])
	testAllocRate(t, metrics[16])
}

func testAllocRate(t *testing.T, m Metric) {
	assert.Equal(t, "alloc-rate", m.Name())
	assert.Equal(t, "Allocation Rate", m.DisplayName())
	assert.EqualValues(t, 204392, m.Increment())
	assert.Equal(t, "Sum", m.CounterType())
	assert.Equal(t, "B", m.DisplayUnits())
	assert.Equal(t, "00:00:01", m.DisplayRateTimeScale())
	assert.EqualValues(t, 0.999219, m.IntervalSec())
	assert.Equal(t, "Interval=1000", m.Series())
}

func testCPUUsage(t *testing.T, m Metric) {
	assert.Equal(t, "CPU Usage", m.DisplayName())
	assert.EqualValues(t, 0, m.StandardDeviation())
	assert.EqualValues(t, 1, m.Count())
	assert.EqualValues(t, 0.999219, m.IntervalSec())
	assert.Equal(t, "Interval=1000", m.Series())
	assert.Equal(t, "Mean", m.CounterType())
	assert.Equal(t, "%", m.DisplayUnits())
	assert.Equal(t, "cpu-usage", m.Name())
	assert.EqualValues(t, 0, m.Mean())
	assert.EqualValues(t, 0, m.Min())
	assert.EqualValues(t, 0, m.Max())
}

func TestEventParserErrors(t *testing.T) {
	data, err := network.ReadBlobData(path.Join("..", "testdata"), 4)
	require.NoError(t, err)
	for i := 0; i < 57; i++ {
		testEventParserError(t, data, i)
	}
}

func testEventParserError(t *testing.T, data [][]byte, i int) {
	rw := network.NewBlobReader(data)
	reader := network.NewMultiReader(rw, &network.NopBlobWriter{})
	err := reader.Seek(1131)
	rw.ErrOnRead(i)
	require.NoError(t, err)
	_, err = parseEventBlock(reader, fms())
	require.Error(t, err)
}

func fms() fieldMetadataMap {
	return fieldMetadataMap{
		1: fieldMetadata{
			header: metadataHeader{
				metadataID:    1,
				providerName:  "System.Runtime",
				eventHeaderID: 2,
				eventName:     "EventCounters",
			},
			fields: []field{{
				fieldType: "Struct",
				fields: []field{{
					name:      "Payload",
					fieldType: "Struct",
					fields: []field{
						{name: "Name", fieldType: "String"},
						{name: "DisplayName", fieldType: "String"},
						{name: "Mean", fieldType: "Double"},
						{name: "StandardDeviation", fieldType: "Double"},
						{name: "Count", fieldType: "Int32"},
						{name: "Min", fieldType: "Double"},
						{name: "Max", fieldType: "Double"},
						{name: "IntervalSec", fieldType: "Single"},
						{name: "Series", fieldType: "String"},
						{name: "CounterType", fieldType: "String"},
						{name: "Metadata", fieldType: "String"},
						{name: "DisplayUnits", fieldType: "String"},
					},
				}},
			}},
		},
		2: fieldMetadata{
			header: metadataHeader{
				metadataID:    2,
				providerName:  "System.Runtime",
				eventHeaderID: 3,
				eventName:     "EventCounters",
			},
			fields: []field{{
				fieldType: "Struct",
				fields: []field{{
					name:      "Payload",
					fieldType: "Struct",
					fields: []field{
						{name: "Name", fieldType: "String"},
						{name: "DisplayName", fieldType: "String"},
						{name: "DisplayRateTimeScale", fieldType: "String"},
						{name: "Increment", fieldType: "Double"},
						{name: "IntervalSec", fieldType: "Single"},
						{name: "Metadata", fieldType: "String"},
						{name: "Series", fieldType: "String"},
						{name: "CounterType", fieldType: "String"},
						{name: "DisplayUnits", fieldType: "String"},
					},
				}},
			}}},
	}
}

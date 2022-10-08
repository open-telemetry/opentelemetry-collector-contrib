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

package testutils // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/testutils"

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewGaugeMetrics(t *testing.T) {
	m := NewGaugeMetrics([]TestGauge{
		{
			Name: "metric1",
			DataPoints: []DataPoint{
				{
					Value:      1,
					Attributes: map[string]string{"a": "b", "c": "d", "e": "f"},
				},
			},
		},
		{
			Name: "metric2",
			DataPoints: []DataPoint{
				{
					Value:      2,
					Attributes: map[string]string{"x": "y", "z": "q", "w": "e"},
				},
				{
					Value:      3,
					Attributes: map[string]string{"w": "n"},
				},
			},
		},
	})
	all := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	require.Equal(t, all.Len(), 2)
	require.Equal(t, all.At(0).Name(), "metric1")
	require.Equal(t, all.At(0).Gauge().DataPoints().At(0).DoubleValue(), float64(1))
	require.EqualValues(t, all.At(0).Gauge().DataPoints().At(0).Attributes().AsRaw(), map[string]interface{}{
		"a": "b", "c": "d", "e": "f",
	})
	require.Equal(t, all.At(1).Name(), "metric2")
	require.Equal(t, all.At(1).Gauge().DataPoints().At(0).DoubleValue(), float64(2))
	require.EqualValues(t, all.At(1).Gauge().DataPoints().At(0).Attributes().AsRaw(), map[string]interface{}{
		"x": "y", "z": "q", "w": "e",
	})
	require.Equal(t, all.At(1).Gauge().DataPoints().At(1).DoubleValue(), float64(3))
	require.EqualValues(t, all.At(1).Gauge().DataPoints().At(1).Attributes().AsRaw(), map[string]interface{}{
		"w": "n",
	})
}

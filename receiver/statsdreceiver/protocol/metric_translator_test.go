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

package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestBuildCounterMetric(t *testing.T) {
	timeNow := time.Now()
	metricDescription := statsDMetricdescription{
		name: "testCounter",
	}
	parsedMetric := statsDMetric{
		description: metricDescription,
		intvalue:    32,
		unit:        "meter",
		labelKeys:   []string{"mykey"},
		labelValues: []string{"myvalue"},
	}
	metric := buildCounterMetric(parsedMetric, timeNow)
	expectedMetric := pdata.NewInstrumentationLibraryMetrics()
	expectedMetric.Metrics().Resize(1)
	expectedMetric.Metrics().At(0).SetName("testCounter")
	expectedMetric.Metrics().At(0).SetUnit("meter")
	expectedMetric.Metrics().At(0).SetDataType(pdata.MetricDataTypeIntSum)
	expectedMetric.Metrics().At(0).IntSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
	expectedMetric.Metrics().At(0).IntSum().SetIsMonotonic(true)
	expectedMetric.Metrics().At(0).IntSum().DataPoints().Resize(1)
	expectedMetric.Metrics().At(0).IntSum().DataPoints().At(0).SetValue(32)
	expectedMetric.Metrics().At(0).IntSum().DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timeNow))
	expectedMetric.Metrics().At(0).IntSum().DataPoints().At(0).LabelsMap().Insert("mykey", "myvalue")
	assert.Equal(t, metric, expectedMetric)
}

func TestBuildGaugeMetric(t *testing.T) {
	timeNow := time.Now()
	metricDescription := statsDMetricdescription{
		name: "testGauge",
	}
	parsedMetric := statsDMetric{
		description: metricDescription,
		floatvalue:  32.3,
		unit:        "meter",
		labelKeys:   []string{"mykey", "mykey2"},
		labelValues: []string{"myvalue", "myvalue2"},
	}
	metric := buildGaugeMetric(parsedMetric, timeNow)
	expectedMetric := pdata.NewInstrumentationLibraryMetrics()
	expectedMetric.Metrics().Resize(1)
	expectedMetric.Metrics().At(0).SetName("testGauge")
	expectedMetric.Metrics().At(0).SetUnit("meter")
	expectedMetric.Metrics().At(0).SetDataType(pdata.MetricDataTypeDoubleGauge)
	expectedMetric.Metrics().At(0).DoubleGauge().DataPoints().Resize(1)
	expectedMetric.Metrics().At(0).DoubleGauge().DataPoints().At(0).SetValue(32.3)
	expectedMetric.Metrics().At(0).DoubleGauge().DataPoints().At(0).SetTimestamp(pdata.TimestampFromTime(timeNow))
	expectedMetric.Metrics().At(0).DoubleGauge().DataPoints().At(0).LabelsMap().Insert("mykey", "myvalue")
	expectedMetric.Metrics().At(0).DoubleGauge().DataPoints().At(0).LabelsMap().Insert("mykey2", "myvalue2")
	assert.Equal(t, metric, expectedMetric)
}

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

package icingareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icingareceiver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.opentelemetry.io/otel/attribute"
)

func TestBuildCounterMetric(t *testing.T) {
	var kvs []attribute.KeyValue
	kvs = append(kvs, attribute.String(conventions.AttributeServiceInstanceID, "test"))
	metric := icingaMetric{
		description: icingaMetricDescription{
			name:       "foo",
			metricType: CounterType,
			attrs:      attribute.NewSet(kvs...),
		},
		asFloat: 1.,
		unit:    "bar",
	}
	counter := buildCounterMetric(metric, time.Now())
	require.NotNil(t, counter)
	require.Equal(t, 1, counter.Metrics().Len())
	require.Equal(t, "foo", counter.Metrics().At(0).Name())
	require.Equal(t, pmetric.MetricDataTypeSum, counter.Metrics().At(0).DataType())
	require.Equal(t, "bar", counter.Metrics().At(0).Unit())
	require.Equal(t, 1, counter.Metrics().At(0).Sum().DataPoints().Len())
	require.Equal(t, int64(1), counter.Metrics().At(0).Sum().DataPoints().At(0).IntVal())
	v, _ := counter.Metrics().At(0).Sum().DataPoints().At(0).Attributes().Get(conventions.AttributeServiceInstanceID)
	require.Equal(t, "test", v.AsString())
}

func TestBuildGaugeMetric(t *testing.T) {
	var kvs []attribute.KeyValue
	kvs = append(kvs, attribute.String(conventions.AttributeServiceInstanceID, "test"))
	metric := icingaMetric{
		description: icingaMetricDescription{
			name:       "foo",
			metricType: CounterType,
			attrs:      attribute.NewSet(kvs...),
		},
		asFloat: 1.,
		unit:    "bar",
	}
	counter := buildGaugeMetric(metric, time.Now())
	require.NotNil(t, counter)
	require.Equal(t, 1, counter.Metrics().Len())
	require.Equal(t, "foo", counter.Metrics().At(0).Name())
	require.Equal(t, pmetric.MetricDataTypeGauge, counter.Metrics().At(0).DataType())
	require.Equal(t, "bar", counter.Metrics().At(0).Unit())
	require.Equal(t, 1, counter.Metrics().At(0).Gauge().DataPoints().Len())
	require.Equal(t, 1., counter.Metrics().At(0).Gauge().DataPoints().At(0).DoubleVal())
	v, _ := counter.Metrics().At(0).Gauge().DataPoints().At(0).Attributes().Get(conventions.AttributeServiceInstanceID)
	require.Equal(t, "test", v.AsString())
}

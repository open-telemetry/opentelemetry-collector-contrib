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

package k8sapiserver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type mockNextConsumer struct {
	throwError bool
	t          *testing.T
}

func (m mockNextConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockNextConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if m.throwError {
		return errors.New("throwing an error")
	}

	// verify some of the attributes
	value, found := md.ResourceMetrics().At(0).Resource().Attributes().Get("ClusterName")
	assert.True(m.t, found)
	assert.Equal(m.t, "test-cluster", value.Str())

	value, found = md.ResourceMetrics().At(0).Resource().Attributes().Get("Version")
	assert.Equal(m.t, "0", value.Str())
	assert.True(m.t, found)

	value, found = md.ResourceMetrics().At(0).Resource().Attributes().Get("Type")
	assert.True(m.t, found)
	assert.NotEmpty(m.t, value.Str())

	value, found = md.ResourceMetrics().At(0).Resource().Attributes().Get("Timestamp")
	assert.False(m.t, found)
	assert.Empty(m.t, value.Str())

	value, found = md.ResourceMetrics().At(0).Resource().Attributes().Get("Sources")
	assert.True(m.t, found)
	assert.Equal(m.t, "[\"apiserver\"]", value.Str())

	value, found = md.ResourceMetrics().At(0).Resource().Attributes().Get("NodeName")
	assert.True(m.t, found)
	assert.Equal(m.t, "test-node", value.Str())

	assert.Equal(m.t, len(defaultResourceToType), md.ResourceMetrics().Len())
	assert.Equal(m.t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())

	metric1 := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(m.t, "apiserver_storage_objects", metric1.Name())
	assert.Equal(m.t, 123.4, metric1.Gauge().DataPoints().At(0).DoubleValue())

	assert.Equal(m.t, 1, md.MetricCount())

	return nil
}

func TestPrometheusConsumeMetrics(t *testing.T) {
	nextConsumer := mockNextConsumer{
		throwError: false,
		t:          t,
	}

	consumer := newPrometheusConsumer(zap.NewNop(), nextConsumer, "test-cluster", "test-node")
	consumer.metricsToResource["invalid-metrics-to-resource"] = "invalid"

	cap := consumer.Capabilities()
	assert.True(t, cap.MutatesData)

	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric1 := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	metric1.SetName("apiserver_storage_objects")
	metric1.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(123.4)

	metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
	metric2 := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1)
	metric2.SetName("some_excluded_metric")
	metric2.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(456.7)

	metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty()
	metric3 := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1)
	metric3.SetName("invalid-metrics-to-resource")
	metric3.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(99.9)

	assert.Equal(t, 3, metrics.MetricCount())

	result := consumer.ConsumeMetrics(context.TODO(), metrics)
	assert.NoError(t, result)
}

func TestPrometheusConsumeMetricsForwardsError(t *testing.T) {
	nextConsumer := mockNextConsumer{
		throwError: true,
	}

	consumer := newPrometheusConsumer(zap.NewNop(), nextConsumer, "test-cluster", "test-node")

	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty()

	result := consumer.ConsumeMetrics(context.TODO(), metrics)
	assert.Error(t, result)
}

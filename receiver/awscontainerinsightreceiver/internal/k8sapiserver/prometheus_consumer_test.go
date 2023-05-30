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
	assert.True(m.t, found)
	assert.NotEmpty(m.t, value.Str())

	value, found = md.ResourceMetrics().At(0).Resource().Attributes().Get("Sources")
	assert.True(m.t, found)
	assert.Equal(m.t, "[\"apiserver\"]", value.Str())

	value, found = md.ResourceMetrics().At(0).Resource().Attributes().Get("NodeName")
	assert.True(m.t, found)
	assert.Equal(m.t, "test-node", value.Str())

	return nil
}

func TestPrometheusConsumeMetrics(t *testing.T) {
	nextConsumer := mockNextConsumer{
		throwError: false,
		t:          t,
	}

	consumer := newPrometheusConsumer(zap.NewNop(), nextConsumer, "test-cluster", "test-node")

	cap := consumer.Capabilities()
	assert.True(t, cap.MutatesData)

	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty()

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

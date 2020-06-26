// Copyright 2019, OpenTelemetry Authors
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

package awsxrayreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type mockMetricsConsumer struct {
}

var _ (consumer.MetricsConsumer) = (*mockMetricsConsumer)(nil)

func (m *mockMetricsConsumer) ConsumeMetrics(ctx context.Context, md pdata.Metrics) error {
	return nil
}

type mockTraceConsumer struct {
}

var _ (consumer.TraceConsumer) = (*mockTraceConsumer)(nil)

func (m *mockTraceConsumer) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	return nil
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))

	assert.Equal(t, configmodels.Type(typeStr), factory.Type())
}

func TestCreateTraceReceiver(t *testing.T) {
	// TODO: Create proper tests after CreateTraceReceiver is implemented.
	factory := &Factory{}
	_, err := factory.CreateTraceReceiver(
		context.Background(),
		component.ReceiverCreateParams{
			Logger: zap.NewNop(),
		},
		factory.CreateDefaultConfig().(*Config),
		&mockTraceConsumer{},
	)
	assert.NotNil(t, err, "not implemented yet")
	assert.EqualError(t, err, configerror.ErrDataTypeIsNotSupported.Error())
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := &Factory{}
	_, err := factory.CreateMetricsReceiver(
		context.Background(),
		component.ReceiverCreateParams{
			Logger: zap.NewNop(),
		},
		factory.CreateDefaultConfig().(*Config),
		&mockMetricsConsumer{},
	)
	assert.NotNil(t, err, "a trace receiver factory should not create a metric receiver")
	assert.EqualError(t, err, configerror.ErrDataTypeIsNotSupported.Error())
}

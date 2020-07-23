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

package prometheusexec

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestCreateTraceAndMetricsReceiver(t *testing.T) {
	var (
		traceReceiver  component.TraceReceiver
		metricReceiver component.MetricsReceiver
	)

	factories, err := config.ExampleComponents()
	assert.NoError(t, err)

	factory := &Factory{}
	receiverType := "prometheus_exec"
	factories.Receivers[configmodels.Type(receiverType)] = factory

	config, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	assert.NoError(t, err)
	assert.NotNil(t, config)

	receiver := config.Receivers[receiverType]

	// Test CreateTraceReceiver
	traceReceiver, err = factory.CreateTraceReceiver(context.Background(), zap.NewNop(), receiver, nil)

	assert.Equal(t, nil, traceReceiver)
	assert.Equal(t, configerror.ErrDataTypeIsNotSupported, err)

	// Test CreateMetricsReceiver
	metricReceiver, err = factory.CreateMetricsReceiver(context.Background(), zap.NewNop(), receiver, nil)
	assert.Equal(t, nil, err)
	assert.Equal(t, &prometheusReceiverWrapper{
		logger:             zap.NewNop(),
		config:             receiver.(*Config),
		consumer:           nil,
		receiverConfig:     nil,
		subprocessConfig:   nil,
		prometheusReceiver: nil,
		context:            nil,
		host:               nil,
	}, metricReceiver)
}

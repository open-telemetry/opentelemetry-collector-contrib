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

package prometheusexecreceiver

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configerror"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.uber.org/zap"
)

func TestCreateTraceAndMetricsReceiver(t *testing.T) {
	var (
		traceReceiver  component.TraceReceiver
		metricReceiver component.MetricsReceiver
	)

	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := &Factory{}
	receiverType := "prometheus_exec"
	factories.Receivers[configmodels.Type(receiverType)] = factory

	config, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

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
	assert.Equal(t, &prometheusExecReceiver{
		logger:             zap.NewNop(),
		config:             receiver.(*Config),
		consumer:           nil,
		receiverConfig:     nil,
		subprocessConfig:   nil,
		prometheusReceiver: nil,
	}, metricReceiver)
}

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

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

func Test_loadAndCreateRuntimeReceiver(t *testing.T) {
	run := &receiverRunner{logger: zap.NewNop(), nextConsumer: &mockMetricsConsumer{}, idNamespace: "receiver_creator/1"}
	exampleFactory := &config.ExampleReceiverFactory{}
	template, err := newReceiverTemplate("examplereceiver/1", nil)
	require.NoError(t, err)

	loadedConfig, err := run.loadRuntimeReceiverConfig(exampleFactory, template.receiverConfig, userConfigMap{
		endpointConfigKey: "localhost:12345",
	})
	require.NoError(t, err)
	assert.NotNil(t, loadedConfig)
	exampleConfig := loadedConfig.(*config.ExampleReceiver)
	// Verify that the overridden endpoint is used instead of the one in the config file.
	assert.Equal(t, "localhost:12345", exampleConfig.Endpoint)
	assert.Equal(t, "receiver_creator/1/examplereceiver/1{endpoint=\"localhost:12345\"}", exampleConfig.Name())

	// Test that metric receiver can be created from loaded config.
	t.Run("test create receiver from loaded config", func(t *testing.T) {
		recvr, err := run.createRuntimeReceiver(exampleFactory, loadedConfig)
		require.NoError(t, err)
		assert.NotNil(t, recvr)
		exampleReceiver := recvr.(*config.ExampleReceiverProducer)
		assert.Equal(t, run.nextConsumer, exampleReceiver.MetricsConsumer)
	})
}

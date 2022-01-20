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

package jmxreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestWithInvalidConfig(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, config.Type("jmx"), f.Type())

	cfg := f.CreateDefaultConfig()
	require.NotNil(t, cfg)

	r, err := f.CreateMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg, consumertest.NewNop(),
	)
	require.Error(t, err)
	assert.Equal(t, "jmx missing required fields: `endpoint`, `target_system` or `groovy_script`", err.Error())
	require.Nil(t, r)
}

func TestWithValidConfig(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, config.Type("jmx"), f.Type())

	cfg := f.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "myendpoint:12345"
	cfg.(*Config).GroovyScript = "mygroovyscriptpath"

	params := componenttest.NewNopReceiverCreateSettings()
	r, err := f.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	receiver := r.(*jmxMetricReceiver)
	assert.Same(t, receiver.logger, params.Logger)
	assert.Same(t, receiver.config, cfg)
}

func TestWithSetProperties(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, config.Type("jmx"), f.Type())

	cfg := f.CreateDefaultConfig()
	cfg.(*Config).Endpoint = "myendpoint:12345"
	cfg.(*Config).GroovyScript = "mygroovyscriptpath"
	cfg.(*Config).Properties["org.slf4j.simpleLogger.defaultLogLevel"] = "trace"
	cfg.(*Config).Properties["org.java.fake.property"] = "true"

	params := componenttest.NewNopReceiverCreateSettings()
	r, err := f.CreateMetricsReceiver(context.Background(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, r)
	receiver := r.(*jmxMetricReceiver)
	assert.Same(t, receiver.logger, params.Logger)
	assert.Same(t, receiver.config, cfg)
}

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

package sapmreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	params := receivertest.NewNopCreateSettings()
	tReceiver, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, tReceiver, "receiver creation failed")

	mReceiver, err := factory.CreateMetricsReceiver(context.Background(), params, cfg, nil)
	assert.ErrorIs(t, err, component.ErrDataTypeIsNotSupported)
	assert.Nil(t, mReceiver)
}

func TestCreateInvalidHTTPEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Endpoint = ""
	params := receivertest.NewNopCreateSettings()
	_, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err, "receiver creation with no endpoints must fail")
}

func TestCreateNoPort(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Endpoint = "localhost:"
	params := receivertest.NewNopCreateSettings()
	_, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err, "receiver creation with no port number must fail")
}

func TestCreateLargePort(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	rCfg := cfg.(*Config)

	rCfg.Endpoint = "localhost:65536"
	params := receivertest.NewNopCreateSettings()
	_, err := factory.CreateTracesReceiver(context.Background(), params, cfg, nil)
	assert.Error(t, err, "receiver creation with too large port number must fail")
}

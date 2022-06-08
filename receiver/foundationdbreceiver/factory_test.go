// Copyright 2022 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package foundationdbreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "fauled to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Address = "localhost:0"

	params := componenttest.NewNopReceiverCreateSettings()
	tReceiver, err := createTracesReceiver(context.Background(), params, cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tReceiver, "receiver creation failed")
}

func TestCreateReceiverWithConfigErr(t *testing.T) {
	cfg := &Config{
		ReceiverSettings: config.ReceiverSettings{},
		Address:          "abc",
		MaxPacketSize:    0,
		SocketBufferSize: 0,
	}

	receiver, err := createTracesReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		cfg,
		consumertest.NewNop())

	assert.Error(t, err, "foo")
	assert.Nil(t, receiver)
}

func TestCreateMetricsReceiverWithNilConsumer(t *testing.T) {
	receiver, err := createTracesReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		createDefaultConfig(),
		nil,
	)

	assert.Error(t, err, "nil consumer")
	assert.Nil(t, receiver)
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package foundationdbreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
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
		ReceiverSettings: receiverhelper.ReceiverSettings{},
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

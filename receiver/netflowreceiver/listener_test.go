// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package netflowreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateValidDefaultListener(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings()
	receiver, err := factory.CreateLogsReceiver(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, receiver, "receiver creation failed")
	assert.Equal(t, "sflow", receiver.(*netflowReceiver).config.Listeners[0].Scheme)
	assert.Equal(t, 6343, receiver.(*netflowReceiver).config.Listeners[0].Port)
	assert.Equal(t, 1, receiver.(*netflowReceiver).config.Listeners[0].Sockets)
	assert.Equal(t, 2, receiver.(*netflowReceiver).config.Listeners[0].Workers)
	assert.Equal(t, 1_000_000, receiver.(*netflowReceiver).config.Listeners[0].QueueSize)

}

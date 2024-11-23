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
	receiver, err := factory.CreateLogs(context.Background(), set, cfg, consumertest.NewNop())
	assert.NoError(t, err, "receiver creation failed")
	assert.NotNil(t, receiver, "receiver creation failed")
	assert.Equal(t, "netflow", receiver.(*netflowReceiver).config.Scheme)
	assert.Equal(t, 2055, receiver.(*netflowReceiver).config.Port)
	assert.Equal(t, 1, receiver.(*netflowReceiver).config.Sockets)
	assert.Equal(t, 2, receiver.(*netflowReceiver).config.Workers)
	assert.Equal(t, 1_000_000, receiver.(*netflowReceiver).config.QueueSize)
}

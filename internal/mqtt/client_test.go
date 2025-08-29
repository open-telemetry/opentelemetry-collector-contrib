// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqtt

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewMqttClient(t *testing.T) {
	logger := zap.NewNop()
	client := NewMqttClient(logger)
	assert.NotNil(t, client)
}

func TestDialConfig(t *testing.T) {
	logger := zap.NewNop()
	client := NewMqttClient(logger)

	config := DialConfig{
		BrokerURLs:                 []string{"tcp://localhost:1883"},
		ClientID:                   "test-client",
		Username:                   "test-user",
		Password:                   "test-pass",
		ConnectTimeout:             10 * time.Second,
		KeepAlive:                  30 * time.Second,
		AutoReconnect:              true,
		ConnectRetry:               true,
		ConnectRetryInterval:       2 * time.Second,
		MaxReconnectInterval:       30 * time.Second,
		PingTimeout:                10 * time.Second,
		PublishConfirmationTimeout: 5 * time.Second,
	}

	// This will fail to connect since there's no broker, but we can test the config parsing
	conn, err := client.DialConfig(config)
	// We expect an error since there's no broker running
	assert.Error(t, err)
	assert.Nil(t, conn)
}

func TestConnectionHolder_IsConnected(t *testing.T) {
	holder := &connectionHolder{
		connLock: &sync.Mutex{},
	}

	// Should be false when no client is set
	assert.False(t, holder.IsConnected())
}

func TestConnectionHolder_Close(t *testing.T) {
	holder := &connectionHolder{
		connLock: &sync.Mutex{},
	}

	// Should not panic when no client is set
	err := holder.Close()
	assert.NoError(t, err)
}

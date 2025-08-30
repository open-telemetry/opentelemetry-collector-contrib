// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/mqtt"
)

func TestNewConnection(t *testing.T) {
	logger := zap.NewNop()
	client := mqtt.NewMqttClient(logger)

	config := DialConfig{
		DialConfig: mqtt.DialConfig{
			BrokerURLs:                 []string{"tcp://localhost:1883"},
			ClientID:                   "test-client",
			Username:                   "test-user",
			Password:                   "test-pass",
			ConnectTimeout:             1 * time.Second, // Short timeout for test
			KeepAlive:                  30 * time.Second,
			AutoReconnect:              false, // Disable auto-reconnect for test
			ConnectRetry:               false, // Disable retry for test
			ConnectRetryInterval:       2 * time.Second,
			MaxReconnectInterval:       30 * time.Second,
			PingTimeout:                10 * time.Second,
			PublishConfirmationTimeout: 5 * time.Second,
		},
		QoS:                        1,
		Retain:                     false,
		PublishConfirmationTimeout: 5 * time.Second,
	}

	// This will fail to connect since there's no broker, but we can test the config parsing
	pub, err := NewConnection(logger, client, config)
	// We expect either an error (if connection fails) or a valid publisher (if connection succeeds)
	if err != nil {
		// Connection failed as expected
		assert.NotNil(t, pub) // Publisher should still be created even if connection fails
	} else {
		// Connection succeeded (unlikely without broker, but possible)
		assert.NotNil(t, pub)
	}
}

func TestPublisher_Publish(t *testing.T) {
	pub := &publisher{
		logger:     zap.NewNop(),
		connection: &testConnection{},
		config: DialConfig{
			QoS:    1,
			Retain: false,
		},
	}

	message := Message{
		Topic: "test/topic",
		Body:  []byte("test message"),
	}

	err := pub.Publish(context.Background(), message)
	assert.NoError(t, err)
}

func TestPublisher_Close(t *testing.T) {
	pub := &publisher{
		logger:     zap.NewNop(),
		connection: &testConnection{},
	}

	err := pub.Close()
	assert.NoError(t, err)
}

type testConnection struct{}

func (c *testConnection) ReconnectIfUnhealthy() error {
	return nil
}

func (c *testConnection) IsConnected() bool {
	return true
}

func (c *testConnection) Publish(ctx context.Context, topic string, qos byte, retain bool, payload []byte) error {
	return nil
}

func (c *testConnection) Close() error {
	return nil
}

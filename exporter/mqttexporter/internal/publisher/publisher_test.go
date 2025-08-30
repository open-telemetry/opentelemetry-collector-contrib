// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"

import (
	"context"
<<<<<<< HEAD
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
	// We expect an error since there's no broker running
	assert.Error(t, err)
	assert.NotNil(t, pub)
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

=======
	"net/url"
	"testing"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewConnection(t *testing.T) {
	brokerURL, _ := url.Parse("tcp://localhost:1883")
	config := DialConfig{
		ClientOptions: mqtt.ClientOptions{
			Servers: []*url.URL{brokerURL},
			ClientID: "test-client",
		},
		QoS:    1,
		Retain: false,
	}

	logger := zap.NewNop()
	publisher, err := NewConnection(logger, config)
	
	// This will fail because we can't connect to a real MQTT broker in unit tests
	// but we can test that the function returns an error as expected
	assert.Error(t, err)
	// The publisher should still be created even if connection fails
	assert.NotNil(t, publisher)
}

func TestPublisherPublish(t *testing.T) {
	// Create a mock publisher for testing
	publisher := &mockPublisher{}
	
	ctx := context.Background()
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
	message := Message{
		Topic: "test/topic",
		Body:  []byte("test message"),
	}
<<<<<<< HEAD

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
=======
	
	err := publisher.Publish(ctx, message)
	assert.NoError(t, err)
}

func TestPublisherClose(t *testing.T) {
	// Create a mock publisher for testing
	publisher := &mockPublisher{}
	
	err := publisher.Close()
	assert.NoError(t, err)
}

type mockPublisher struct{}

func (p *mockPublisher) Publish(ctx context.Context, message Message) error {
	return nil
}

func (p *mockPublisher) Close() error {
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
	return nil
}

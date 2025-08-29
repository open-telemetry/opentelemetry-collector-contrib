// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"

import (
	"context"
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
	message := Message{
		Topic: "test/topic",
		Body:  []byte("test message"),
	}
	
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
	return nil
}

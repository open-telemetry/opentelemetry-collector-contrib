// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/mqtt"
)

type DialConfig struct {
	mqtt.DialConfig
	QoS                        byte
	Retain                     bool
	PublishConfirmationTimeout time.Duration
}

type Message struct {
	Topic string
	Body  []byte
}

func NewConnection(logger *zap.Logger, client mqtt.MqttClient, config DialConfig) (Publisher, error) {
	p := publisher{
		logger: logger,
		client: client,
		config: config,
	}

	conn, err := p.client.DialConfig(p.config.DialConfig)
	if err != nil {
		return &p, err
	}
	p.connection = conn

	return &p, nil
}

type Publisher interface {
	Publish(ctx context.Context, message Message) error
	Close() error
}

type publisher struct {
	logger     *zap.Logger
	client     mqtt.MqttClient
	config     DialConfig
	connection mqtt.Connection
}

func (p *publisher) Publish(ctx context.Context, message Message) error {
	err := p.connection.ReconnectIfUnhealthy()
	if err != nil {
		return err
	}

	// Publish message with QoS and retain settings
	return p.connection.Publish(ctx, message.Topic, p.config.QoS, p.config.Retain, message.Body)
}

func (p *publisher) Close() error {
	if p.connection == nil {
		return nil
	}
	return p.connection.Close()
}

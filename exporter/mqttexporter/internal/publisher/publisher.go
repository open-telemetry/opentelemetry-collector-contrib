// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"

import (
	"context"
<<<<<<< HEAD
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/mqtt"
)

type DialConfig struct {
	mqtt.DialConfig
	QoS                        byte
	Retain                     bool
	PublishConfirmationTimeout time.Duration
=======
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

type DialConfig struct {
	mqtt.ClientOptions
	QoS                       byte
	Retain                    bool
	PublishConfirmationTimeout time.Duration
	TLS                       *tls.Config
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
}

type Message struct {
	Topic string
	Body  []byte
}

<<<<<<< HEAD
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

=======
func NewConnection(logger *zap.Logger, config DialConfig) (Publisher, error) {
	p := publisher{
		logger: logger,
		config: config,
	}

	// Set up MQTT client options
	opts := config.ClientOptions
	if config.TLS != nil {
		opts.SetTLSConfig(config.TLS)
	}

	// Create MQTT client
	client := mqtt.NewClient(&opts)
	p.client = client

	// Connect to MQTT broker
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return &p, token.Error()
	}

	logger.Info("Connected to MQTT broker")
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
	return &p, nil
}

type Publisher interface {
	Publish(ctx context.Context, message Message) error
	Close() error
}

type publisher struct {
<<<<<<< HEAD
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
=======
	logger *zap.Logger
	config DialConfig
	client mqtt.Client
}

func (p *publisher) Publish(ctx context.Context, message Message) error {
	if !p.client.IsConnected() {
		return errors.New("MQTT client is not connected")
	}

	// Publish message with QoS and retain settings
	token := p.client.Publish(message.Topic, p.config.QoS, p.config.Retain, message.Body)
	
	// Wait for the token to complete with timeout
	select {
	case <-token.Done():
		if token.Error() != nil {
			p.logger.Error("Error publishing message", zap.Error(token.Error()))
			return token.Error()
		}
		p.logger.Debug("Message published successfully")
		return nil
	case <-time.After(p.config.PublishConfirmationTimeout):
		p.logger.Warn("Timeout waiting for publish confirmation", zap.Duration("timeout", p.config.PublishConfirmationTimeout))
		return fmt.Errorf("timeout waiting for publish confirmation after %s", p.config.PublishConfirmationTimeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *publisher) Close() error {
	if p.client == nil {
		return nil
	}
	
	// Disconnect from MQTT broker
	p.client.Disconnect(250) // 250ms wait time
	return nil
>>>>>>> 2c2e65cb1a (feat(mqttexporter): add MQTT exporter and wire into local build)
}

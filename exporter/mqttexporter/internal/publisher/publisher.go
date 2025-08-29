// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

type DialConfig struct {
	mqtt.ClientOptions
	QoS                        byte
	Retain                     bool
	PublishConfirmationTimeout time.Duration
	TLS                        *tls.Config
}

type Message struct {
	Topic string
	Body  []byte
}

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

	// Improve connection resilience
	if opts.KeepAlive == 0 {
		opts.KeepAlive = int64((30 * time.Second).Seconds())
	}
	if opts.PingTimeout == 0 {
		opts.PingTimeout = 10 * time.Second
	}
	opts.AutoReconnect = true
	opts.ConnectRetry = true
	if opts.ConnectRetryInterval == 0 {
		opts.ConnectRetryInterval = 2 * time.Second
	}
	if opts.MaxReconnectInterval == 0 {
		opts.MaxReconnectInterval = 30 * time.Second
	}
	// Log lifecycle events
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		logger.Info("MQTT connected")
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		logger.Warn("MQTT connection lost", zap.Error(err))
	})

	// Create MQTT client
	client := mqtt.NewClient(&opts)
	p.client = client

	// Connect to MQTT broker
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return &p, token.Error()
	}

	logger.Info("Connected to MQTT broker")
	return &p, nil
}

type Publisher interface {
	Publish(ctx context.Context, message Message) error
	Close() error
}

type publisher struct {
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
}

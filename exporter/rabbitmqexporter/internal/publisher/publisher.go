// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/publisher"

import (
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	otelrabbitmq "github.com/open-telemetry/opentelemetry-collector-contrib/internal/rabbitmq"
)

type DialConfig struct {
	otelrabbitmq.DialConfig
	Durable                    bool
	PublishConfirmationTimeout time.Duration
}

type Message struct {
	Exchange   string
	RoutingKey string
	Body       []byte
}

func NewConnection(logger *zap.Logger, client otelrabbitmq.AmqpClient, config DialConfig) (Publisher, error) {
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
	client     otelrabbitmq.AmqpClient
	config     DialConfig
	connection otelrabbitmq.Connection
}

func (p *publisher) Publish(ctx context.Context, message Message) error {
	err := p.connection.ReconnectIfUnhealthy()
	if err != nil {
		return err
	}

	// Create a new amqp channel for publishing messages and request that the broker confirms delivery.
	// This could later be optimized to re-use channels which avoids repeated network calls to create and close them.
	// Concurrency-control through something like a resource pool would be necessary since aqmp channels are not thread safe.
	channel, err := p.connection.Channel()
	defer func(channel otelrabbitmq.Channel) {
		if channel != nil {
			err2 := channel.Close()
			if err2 != nil {
				p.logger.Warn("Failed closing channel", zap.Error(err2))
			}
		}
	}(channel)
	if err != nil {
		p.logger.Error("Error creating AMQP channel")
		return err
	}
	err = channel.Confirm(false)
	if err != nil {
		p.logger.Error("Error enabling channel confirmation mode")
		return err
	}

	// Send the message
	deliveryMode := amqp.Transient
	if p.config.Durable {
		deliveryMode = amqp.Persistent
	}

	confirmation, err := channel.PublishWithDeferredConfirmWithContext(ctx, message.Exchange, message.RoutingKey, true, false, amqp.Publishing{
		Body:         message.Body,
		DeliveryMode: deliveryMode,
	})
	if err != nil {
		err = errors.Join(errors.New("error publishing message"), err)
		return err
	}

	// Wait for async confirmation of the message
	select {
	case <-confirmation.Done():
		if confirmation.Acked() {
			p.logger.Debug("Received ack")
			return nil
		}
		p.logger.Warn("Received nack from rabbitmq publishing confirmation")
		err := errors.New("received nack from rabbitmq publishing confirmation")
		return err

	case <-time.After(p.config.PublishConfirmationTimeout):
		p.logger.Warn("Timeout waiting for publish confirmation", zap.Duration("timeout", p.config.ConnectionTimeout))
		err := fmt.Errorf("timeout waiting for publish confirmation after %s", p.config.PublishConfirmationTimeout)
		return err
	}
}

func (p *publisher) Close() error {
	if p.connection == nil {
		return nil
	}
	return p.connection.Close()
}

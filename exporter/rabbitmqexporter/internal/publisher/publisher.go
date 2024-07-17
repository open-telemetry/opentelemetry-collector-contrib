// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/publisher"

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type DialConfig struct {
	URL                        string
	Durable                    bool
	Vhost                      string
	Auth                       amqp.Authentication
	ConnectionTimeout          time.Duration
	Heartbeat                  time.Duration
	PublishConfirmationTimeout time.Duration
	TLS                        *tls.Config
	ConnectionName             string
}

type Message struct {
	Exchange   string
	RoutingKey string
	Body       []byte
}

func NewConnection(logger *zap.Logger, client AmqpClient, config DialConfig) (Publisher, error) {
	p := publisher{
		logger:           logger,
		client:           client,
		config:           config,
		connLock:         &sync.Mutex{},
		connectionErrors: make(chan *amqp.Error, 1),
	}

	p.connLock.Lock()
	defer p.connLock.Unlock()
	err := p.connect()

	return &p, err
}

type Publisher interface {
	Publish(ctx context.Context, message Message) error
	Close() error
}

type publisher struct {
	logger           *zap.Logger
	client           AmqpClient
	config           DialConfig
	connLock         *sync.Mutex
	connection       Connection
	connectionErrors chan *amqp.Error
}

func (p *publisher) Publish(ctx context.Context, message Message) error {
	err := p.reconnectIfUnhealthy()
	if err != nil {
		return err
	}

	// Create a new amqp channel for publishing messages and request that the broker confirms delivery.
	// This could later be optimized to re-use channels which avoids repeated network calls to create and close them.
	// Concurrency-control through something like a resource pool would be necessary since aqmp channels are not thread safe.
	channel, err := p.connection.Channel()
	defer func(channel Channel) {
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

func (p *publisher) reconnectIfUnhealthy() error {
	p.connLock.Lock()
	defer p.connLock.Unlock()

	hasConnectionError := false
	select {
	case err := <-p.connectionErrors:
		hasConnectionError = true
		p.logger.Info("Received connection error, will retry restoring unhealthy connection", zap.Error(err))
	default:
		break
	}

	if hasConnectionError || !p.isConnected() {
		if p.isConnected() {
			err := p.connection.Close()
			if err != nil {
				p.logger.Warn("Error closing unhealthy connection", zap.Error(err))
			}
		}

		if err := p.connect(); err != nil {
			return errors.Join(errors.New("failed attempt at restoring unhealthy connection"), err)
		}
		p.logger.Info("Successfully restored unhealthy rabbitmq connection")
	}

	return nil
}

func (p *publisher) connect() error {
	p.logger.Debug("Connecting to rabbitmq")

	properties := amqp.Table{}
	properties.SetClientConnectionName(p.config.ConnectionName)

	connection, err := p.client.DialConfig(p.config.URL, amqp.Config{
		SASL:            []amqp.Authentication{p.config.Auth},
		Vhost:           p.config.Vhost,
		Heartbeat:       p.config.Heartbeat,
		Dial:            amqp.DefaultDial(p.config.ConnectionTimeout),
		Properties:      properties,
		TLSClientConfig: p.config.TLS,
	})
	if connection != nil {
		p.connection = connection
	}
	if err != nil {
		return err
	}

	// Goal is to lazily restore the connection so this needs to be buffered to avoid blocking on asynchronous amqp errors.
	// Also re-create this channel each time because apparently the amqp library can close it
	p.connectionErrors = make(chan *amqp.Error, 1)
	p.connection.NotifyClose(p.connectionErrors)
	return nil
}

func (p *publisher) Close() error {
	if p.isConnected() {
		return p.connection.Close()
	}
	return nil
}

func (p *publisher) isConnected() bool {
	return p.connection != nil && !p.connection.IsClosed()
}

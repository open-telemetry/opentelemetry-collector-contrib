// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmq // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/rabbitmq"

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

type AmqpClient interface {
	DialConfig(config DialConfig) (Connection, error)
}

type Connection interface {
	ReconnectIfUnhealthy() error
	IsClosed() bool
	Channel() (Channel, error)
	NotifyClose(receiver chan *amqp.Error) chan *amqp.Error
	Close() error
}

type Channel interface {
	Confirm(noWait bool) error
	PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (DeferredConfirmation, error)
	IsClosed() bool
	Close() error
}

type DeferredConfirmation interface {
	Done() <-chan struct{}
	Acked() bool
}

type connectionHolder struct {
	url              string
	config           amqp.Config
	connection       *amqp.Connection
	logger           *zap.Logger
	connLock         *sync.Mutex
	connectionErrors chan *amqp.Error
}

type channelHolder struct {
	channel *amqp.Channel
}

type deferredConfirmationHolder struct {
	confirmation *amqp.DeferredConfirmation
}

type DialConfig struct {
	URL               string
	Vhost             string
	Auth              amqp.Authentication
	ConnectionTimeout time.Duration
	Heartbeat         time.Duration
	TLS               *tls.Config
	ConnectionName    string
}

func NewAmqpClient(logger *zap.Logger) AmqpClient {
	return &client{logger: logger}
}

type client struct {
	logger *zap.Logger
}

func (c *client) DialConfig(config DialConfig) (Connection, error) {
	properties := amqp.Table{}
	properties.SetClientConnectionName(config.ConnectionName)
	ch := &connectionHolder{
		url: config.URL,
		config: amqp.Config{
			SASL:            []amqp.Authentication{config.Auth},
			Vhost:           config.Vhost,
			TLSClientConfig: config.TLS,
			Heartbeat:       config.Heartbeat,
			Dial:            amqp.DefaultDial(config.ConnectionTimeout),
			Properties:      properties,
		},
		logger:           c.logger,
		connLock:         &sync.Mutex{},
		connectionErrors: make(chan *amqp.Error, 1),
	}

	ch.connLock.Lock()
	defer ch.connLock.Unlock()

	return ch, ch.connect()
}

func (c *connectionHolder) ReconnectIfUnhealthy() error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	hasConnectionError := false
	select {
	case err := <-c.connectionErrors:
		hasConnectionError = true
		c.logger.Info("Received connection error, will retry restoring unhealthy connection", zap.Error(err))
	default:
		break
	}

	if hasConnectionError || !c.isConnected() {
		if c.isConnected() {
			err := c.connection.Close()
			if err != nil {
				c.logger.Warn("Error closing unhealthy connection", zap.Error(err))
			}
		}

		if err := c.connect(); err != nil {
			return errors.Join(errors.New("failed attempt at restoring unhealthy connection"), err)
		}
		c.logger.Info("Successfully restored unhealthy rabbitmq connection")
	}

	return nil
}

func (c *connectionHolder) connect() error {
	c.logger.Debug("Connecting to rabbitmq")

	connection, err := amqp.DialConfig(c.url, c.config)
	if connection != nil {
		c.connection = connection
	}
	if err != nil {
		return err
	}

	// Goal is to lazily restore the connection so this needs to be buffered to avoid blocking on asynchronous amqp errors.
	// Also re-create this channel each time because apparently the amqp library can close it
	c.connectionErrors = make(chan *amqp.Error, 1)
	c.connection.NotifyClose(c.connectionErrors)
	return nil
}

func (c *connectionHolder) Close() error {
	if c.isConnected() {
		return c.connection.Close()
	}
	return nil
}

func (c *connectionHolder) isConnected() bool {
	return c.connection != nil && !c.IsClosed()
}

func (c *connectionHolder) Channel() (Channel, error) {
	channel, err := c.connection.Channel()
	if err != nil {
		return nil, err
	}
	return &channelHolder{channel: channel}, nil
}

func (c *connectionHolder) IsClosed() bool {
	return c.connection.IsClosed()
}

func (c *connectionHolder) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	return c.connection.NotifyClose(receiver)
}

func (c *channelHolder) Confirm(noWait bool) error {
	return c.channel.Confirm(noWait)
}

func (c *channelHolder) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (DeferredConfirmation, error) {
	confirmation, err := c.channel.PublishWithDeferredConfirmWithContext(ctx, exchange, key, mandatory, immediate, msg)
	if err != nil {
		return nil, err
	}
	return &deferredConfirmationHolder{confirmation: confirmation}, nil
}

func (c *channelHolder) IsClosed() bool {
	return c.channel.IsClosed()
}

func (c *channelHolder) Close() error {
	return c.channel.Close()
}

func (d *deferredConfirmationHolder) Done() <-chan struct{} {
	return d.confirmation.Done()
}

func (d *deferredConfirmationHolder) Acked() bool {
	return d.confirmation.Acked()
}

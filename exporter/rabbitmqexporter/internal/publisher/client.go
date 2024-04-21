// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/publisher"

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AmqpClient interface {
	DialConfig(url string, config amqp.Config) (Connection, error)
}

type Connection interface {
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

func NewAmqpClient() AmqpClient {
	return &client{}
}

type client struct{}

type connectionHolder struct {
	connection *amqp.Connection
}

type channelHolder struct {
	channel *amqp.Channel
}

type deferredConfirmationHolder struct {
	confirmation *amqp.DeferredConfirmation
}

func (*client) DialConfig(url string, config amqp.Config) (Connection, error) {
	con, err := amqp.DialConfig(url, config)
	if err != nil {
		return nil, err
	}

	return &connectionHolder{
		connection: con,
	}, nil
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

func (c *connectionHolder) Close() error {
	return c.connection.Close()
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

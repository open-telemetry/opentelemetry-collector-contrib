package rabbitmqexporter

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
	"net"
	"time"
)

type AmqpClient interface {
	DialConfig(url string, config amqp.Config) (WrappedConnection, error)
	DefaultDial(connectionTimeout time.Duration) func(network, addr string) (net.Conn, error)
}

type WrappedConnection interface {
	Channel() (WrappedChannel, error)
	IsClosed() bool
	NotifyClose(receiver chan *amqp.Error) chan *amqp.Error
	Close() error
}

type WrappedChannel interface {
	Confirm(noWait bool) error
	PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (*amqp.DeferredConfirmation, error)
	Close() error
}

type amqpClient struct{}

func newAmqpClient() AmqpClient {
	return &amqpClient{}
}

type wrappedConnection struct {
	connection *amqp.Connection
}

type wrappedChannel struct {
	channel *amqp.Channel
}

func (*amqpClient) DialConfig(url string, config amqp.Config) (WrappedConnection, error) {
	connection, err := amqp.DialConfig(url, config)
	if err != nil {
		return nil, err
	}

	return &wrappedConnection{
		connection: connection,
	}, nil
}

func (*amqpClient) DefaultDial(connectionTimeout time.Duration) func(network, addr string) (net.Conn, error) {
	return amqp.DefaultDial(connectionTimeout)
}

func (c *wrappedConnection) Channel() (WrappedChannel, error) {
	channel, err := c.connection.Channel()
	if err != nil {
		return nil, err
	}
	return &wrappedChannel{channel: channel}, nil
}

func (c *wrappedConnection) IsClosed() bool {
	return c.connection.IsClosed()
}

func (c *wrappedConnection) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	return c.connection.NotifyClose(receiver)
}

func (c *wrappedConnection) Close() error {
	return c.connection.Close()
}

func (c *wrappedChannel) Confirm(noWait bool) error {
	return c.channel.Confirm(noWait)
}

func (c *wrappedChannel) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (*amqp.DeferredConfirmation, error) {
	return c.channel.PublishWithDeferredConfirmWithContext(ctx, exchange, key, mandatory, immediate, msg)
}

func (c *wrappedChannel) Close() error {
	return c.channel.Close()
}

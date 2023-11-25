package rabbitmqexporter

import (
	"context"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"net"
	"time"
)

type dialFunc func() (WrappedConnection, error)
type mockClient struct {
	dialImpl dialFunc
}

type channelFunc func() (WrappedChannel, error)
type mockConnection struct {
	channelImpl channelFunc
	isClosed    bool
}

type publishFunc func() (*amqp.DeferredConfirmation, error)
type mockChannel struct {
	published   []amqp.Publishing
	publishImpl publishFunc
}

func (c *mockClient) DialConfig(url string, config amqp.Config) (WrappedConnection, error) {
	return c.dialImpl()
}

func (*mockClient) DefaultDial(connectionTimeout time.Duration) func(network, addr string) (net.Conn, error) {
	return nil
}

func (c *mockConnection) Channel() (WrappedChannel, error) {
	return c.channelImpl()
}

func (c *mockConnection) IsClosed() bool {
	return c.isClosed
}

func (c *mockConnection) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	return make(chan *amqp.Error)
}

func (c *mockConnection) Close() error {
	return nil
}

func (c *mockChannel) Confirm(noWait bool) error {
	return nil
}

func (c *mockChannel) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (*amqp.DeferredConfirmation, error) {
	c.published = append(c.published, msg)
	return c.publishImpl()
}

func (c *mockChannel) Close() error {
	return nil
}

func test() {
	var dialImpl dialFunc = func() (WrappedConnection, error) { return nil, nil }
	mockClient := &mockClient{dialImpl: dialImpl}
	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))

	rabbitMqExporter, err := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), mockClient)
	fmt.Println(rabbitMqExporter, err)
}

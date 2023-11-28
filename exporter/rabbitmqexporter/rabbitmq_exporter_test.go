package rabbitmqexporter

import (
	"context"
	"errors"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
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

type publishFunc func() (WrappedDeferredConfirmation, error)
type mockChannel struct {
	published   []amqp.Publishing
	publishImpl publishFunc
	confirmMode bool
}

type mockDeferredConfirmation struct {
	deliveryTag uint64
	done        chan struct{}
	acked       bool
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
	c.confirmMode = true
	return nil
}

func (c *mockChannel) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (WrappedDeferredConfirmation, error) {
	if c.published == nil {
		c.published = make([]amqp.Publishing, 0)
	}
	c.published = append(c.published, msg)
	return c.publishImpl()
}

func (c *mockChannel) Close() error {
	return nil
}

func (c *mockDeferredConfirmation) DeliveryTag() uint64 {
	return c.deliveryTag
}

func (c *mockDeferredConfirmation) Done() chan struct{} {
	return c.done
}

func (c *mockDeferredConfirmation) Acked() bool {
	return c.acked
}

func TestPublishLogsHappyPath(t *testing.T) {
	client, _, channel, confirmation := buildMocks()

	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))
	customConfig.publishConfirmationTimeout = time.Second

	rabbitMqExporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)

	confirmAsynchronously(confirmation, true, time.Millisecond*50)
	err := rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())

	assert.NoError(t, err)
	assert.Equal(t, 1, len(channel.published))
}

func TestPublishLogsConcurrently(t *testing.T) {
	if runtime.NumCPU() <= 1 {
		t.Skip("Requires multiple CPUs")
	}

	client, connection, channel, confirmation := buildMocks()
	_, _, channel2, confirmation2 := buildMocks()
	confirmations := []*mockDeferredConfirmation{confirmation, confirmation2}

	channelIndex := atomic.Uint32{}
	channels := []*mockChannel{channel, channel2}
	connection.channelImpl = func() (WrappedChannel, error) {
		res := channels[channelIndex.Load()]
		channelIndex.Add(1)
		return res, nil
	}

	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))
	customConfig.channelPoolSize = 2
	customConfig.publishConfirmationTimeout = time.Second

	rabbitMqExporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)
	wg := sync.WaitGroup{}
	wg.Add(2)

	for i := 0; i < 2; i++ {
		go func(ii int) {
			confirmAsynchronously(confirmations[ii], true, time.Millisecond*100)
			err := rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())
			assert.NoError(t, err)
			wg.Done()
		}(i)
	}

	start := time.Now()
	wg.Wait()
	duration := time.Since(start)

	assert.Equal(t, 1, len(channel.published))
	assert.Equal(t, 1, len(channel2.published))
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond)
	assert.Less(t, duration, 200*time.Millisecond)
}

func TestPublishLogsWithTimeout(t *testing.T) {
	client, _, channel, _ := buildMocks()
	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))
	customConfig.publishConfirmationTimeout = time.Millisecond * 100

	rabbitMqExporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)
	err := rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())

	assert.ErrorContains(t, err, "timeout waiting for publish confirmation after 100ms")
	assert.Equal(t, 1, len(channel.published))
}

func TestPublishLogsWithError(t *testing.T) {
	client, _, channel, _ := buildMocks()
	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))

	channel.publishImpl = func() (WrappedDeferredConfirmation, error) {
		return nil, errors.New("expected publish error")
	}

	rabbitMqExporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)
	err := rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())

	assert.ErrorContains(t, err, "expected publish error")
}

func TestRecoveryFromBadConnectionAfterPublishError(t *testing.T) {
	client, badConnection, badChannel, _ := buildMocks()
	_, goodConnection, goodChannel, goodConfirm := buildMocks()

	badChannel.publishImpl = func() (WrappedDeferredConfirmation, error) {
		badConnection.isClosed = true
		return nil, errors.New("expected publish error")
	}
	connectionQueue := []*mockConnection{badConnection, goodConnection}
	index := atomic.Uint32{}
	client.dialImpl = func() (WrappedConnection, error) {
		res := connectionQueue[index.Load()]
		index.Add(1)
		return res, nil
	}

	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))
	customConfig.channelPoolSize = 1

	rabbitMqExporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)

	err := rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())
	assert.ErrorContains(t, err, "expected publish error")
	assert.Equal(t, 1, len(badChannel.published))

	confirmAsynchronously(goodConfirm, true, time.Millisecond*50)
	err = rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(goodChannel.published))
}

func TestRecoveryFromBadChannelAfterPublishError(t *testing.T) {
	client, connection, badChannel, _ := buildMocks()
	_, _, goodChannel, goodConfirm := buildMocks()

	badChannel.publishImpl = func() (WrappedDeferredConfirmation, error) {
		return nil, errors.New("expected publish error")
	}
	channelQueue := []*mockChannel{badChannel, goodChannel}
	index := atomic.Uint32{}
	connection.channelImpl = func() (WrappedChannel, error) {
		res := channelQueue[index.Load()]
		index.Add(1)
		return res, nil
	}

	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))
	customConfig.channelPoolSize = 1

	rabbitMqExporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)

	err := rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())
	assert.ErrorContains(t, err, "expected publish error")

	confirmAsynchronously(goodConfirm, true, time.Millisecond*50)
	err = rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())
	assert.NoError(t, err)
	assert.Equal(t, 1, len(goodChannel.published))
}

func TestPublishLogsWithNack(t *testing.T) {
	client, _, channel, confirmation := buildMocks()
	cfg := createDefaultConfig()
	customConfig := *(cfg.(*config))
	customConfig.publishConfirmationTimeout = time.Second

	confirmAsynchronously(confirmation, false, time.Millisecond*50)
	rabbitMqExporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)
	err := rabbitMqExporter.logsDataPusher(context.Background(), plog.NewLogs())

	assert.ErrorContains(t, err, "received nack from rabbitmq publishing confirmation")
	assert.Equal(t, 1, len(channel.published))
}

func TestPublishConfirmationFlag(t *testing.T) {
	tests := []struct {
		confirmMode bool
	}{
		{confirmMode: true},
		{confirmMode: false},
	}

	for _, tt := range tests {
		cfg := createDefaultConfig()
		customConfig := *(cfg.(*config))
		customConfig.confirmMode = tt.confirmMode

		client, _, channel, _ := buildMocks()

		_, _ = newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)

		assert.Equal(t, tt.confirmMode, channel.confirmMode)
	}
}

func TestPublishDurableFlag(t *testing.T) {
	tests := []struct {
		durable      bool
		deliveryMode uint8
	}{
		{durable: true, deliveryMode: amqp.Persistent},
		{durable: false, deliveryMode: amqp.Transient},
	}

	for _, tt := range tests {
		cfg := createDefaultConfig()
		customConfig := *(cfg.(*config))
		customConfig.durable = tt.durable

		client, _, channel, confirmation := buildMocks()

		exporter, _ := newLogsExporter(customConfig, exportertest.NewNopCreateSettings(), client)
		confirmAsynchronously(confirmation, true, time.Millisecond*50)
		err := exporter.logsDataPusher(context.Background(), plog.NewLogs())

		assert.NoError(t, err)
		assert.Equal(t, 1, len(channel.published))
		assert.Equal(t, tt.deliveryMode, channel.published[0].DeliveryMode)
	}
}

func buildMocks() (*mockClient, *mockConnection, *mockChannel, *mockDeferredConfirmation) {
	confirmation := mockDeferredConfirmation{
		deliveryTag: 1,
		done:        make(chan struct{}),
		acked:       false,
	}
	mockChannel := mockChannel{publishImpl: func() (WrappedDeferredConfirmation, error) {
		return &confirmation, nil
	}}
	mockConnection := mockConnection{isClosed: false, channelImpl: func() (WrappedChannel, error) {
		return &mockChannel, nil
	}}
	mockClient := mockClient{dialImpl: func() (WrappedConnection, error) {
		return &mockConnection, nil
	}}

	return &mockClient, &mockConnection, &mockChannel, &confirmation
}

func confirmAsynchronously(confirmation *mockDeferredConfirmation, acked bool, delay time.Duration) {
	time.AfterFunc(delay, func() {
		confirmation.acked = acked
		confirmation.done <- struct{}{}
	})
}

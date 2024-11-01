// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package publisher

import (
	"context"
	"errors"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/rabbitmq"
)

const (
	connectURL = "amqp://localhost"
	exchange   = "amq.direct"
	routingKey = "some_routing_key"
)

func TestConnectAndClose(t *testing.T) {
	connection := mockConnection{}
	client := mockClient{
		conn: &connection,
	}
	dialConfig := DialConfig{
		DialConfig: rabbitmq.DialConfig{
			URL: connectURL,
		},
	}

	// Start the connection successfully
	client.On("DialConfig", mock.Anything).Return(&connection, nil)
	connection.On("NotifyClose", mock.Anything).Return(make(chan *amqp.Error))

	publisher, err := NewConnection(zap.NewNop(), &client, dialConfig)

	require.NoError(t, err)
	client.AssertExpectations(t)

	// Close the connection
	connection.On("Close").Return(nil)

	err = publisher.Close()
	require.NoError(t, err)
	client.AssertExpectations(t)
	connection.AssertExpectations(t)
}

func TestConnectionErrorAndClose(t *testing.T) {
	connection := mockConnection{}
	client := mockClient{
		conn: &connection,
	}
	dialConfig := DialConfig{
		DialConfig: rabbitmq.DialConfig{
			URL: connectURL,
		},
	}

	connection.On("NotifyClose", mock.Anything).Return(make(chan *amqp.Error))
	client.On("DialConfig", mock.Anything).Return(nil, errors.New("simulated connection error"))
	publisher, err := NewConnection(zap.NewNop(), &client, dialConfig)

	assert.EqualError(t, err, "simulated connection error")

	err = publisher.Close()
	require.NoError(t, err)

	client.AssertExpectations(t)
}

func TestPublishAckedWithinTimeout(t *testing.T) {
	client, connection, channel, confirmation := setupMocksForSuccessfulPublish()

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())

	require.NoError(t, err)
	client.AssertExpectations(t)
	connection.AssertExpectations(t)
	channel.AssertExpectations(t)
	confirmation.AssertExpectations(t)
}

func TestPublishNackedWithinTimeout(t *testing.T) {
	client, connection, channel, confirmation := setupMocksForSuccessfulPublish()
	resetCall(t, confirmation.ExpectedCalls, "Acked")
	confirmation.On("Acked").Return(false)

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())

	assert.EqualError(t, err, "received nack from rabbitmq publishing confirmation")
	client.AssertExpectations(t)
	connection.AssertExpectations(t)
	channel.AssertExpectations(t)
	confirmation.AssertExpectations(t)
}

func TestPublishTimeoutBeforeAck(t *testing.T) {
	client, connection, channel, confirmation := setupMocksForSuccessfulPublish()

	resetCall(t, confirmation.ExpectedCalls, "Done")
	resetCall(t, confirmation.ExpectedCalls, "Acked")
	emptyConfirmationChan := make(<-chan struct{})
	confirmation.On("Done").Return(emptyConfirmationChan)

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())

	assert.EqualError(t, err, "timeout waiting for publish confirmation after 20ms")
	client.AssertExpectations(t)
	connection.AssertExpectations(t)
	channel.AssertExpectations(t)
	confirmation.AssertExpectations(t)
}

func TestPublishTwiceReusingSameConnection(t *testing.T) {
	client, connection, channel, confirmation := setupMocksForSuccessfulPublish()

	// Re-use same chan to allow ACKing both publishes
	confirmationChan := make(chan struct{}, 2)
	confirmationChan <- struct{}{}
	confirmationChan <- struct{}{}
	var confirmationChanRet <-chan struct{} = confirmationChan
	resetCall(t, confirmation.ExpectedCalls, "Done")
	confirmation.On("Done").Return(confirmationChanRet)

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())
	require.NoError(t, err)
	err = publisher.Publish(context.Background(), makePublishMessage())
	require.NoError(t, err)

	client.AssertNumberOfCalls(t, "DialConfig", 1)
	client.AssertExpectations(t)
	connection.AssertExpectations(t)
	channel.AssertExpectations(t)
	confirmation.AssertExpectations(t)
}

func TestRestoreUnhealthyConnectionDuringPublish(t *testing.T) {
	client, connection, channel, confirmation := setupMocksForSuccessfulPublish()

	// Capture the channel that the amqp library uses to notify about connection issues so that we can simulate the notification
	resetCall(t, connection.ExpectedCalls, "NotifyClose")
	var connectionErrChan chan *amqp.Error
	connection.On("NotifyClose", mock.Anything).Return(make(chan *amqp.Error)).Run(func(args mock.Arguments) {
		connectionErrChan = args.Get(0).(chan *amqp.Error)
	})

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	connectionErrChan <- amqp.ErrClosed
	connection.On("Close").Return(nil)

	err = publisher.Publish(context.Background(), makePublishMessage())

	require.NoError(t, err)
	connection.AssertNumberOfCalls(t, "ReconnectIfUnhealthy", 1)
	client.AssertExpectations(t)
	resetCall(t, connection.ExpectedCalls, "Close")
	connection.AssertExpectations(t)
	channel.AssertExpectations(t)
	confirmation.AssertExpectations(t)
}

// Tests code path where connection is closed right after checking the connection error channel
func TestRestoreClosedConnectionDuringPublish(t *testing.T) {
	client, connection, channel, confirmation := setupMocksForSuccessfulPublish()

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())
	require.NoError(t, err)
	client.AssertNumberOfCalls(t, "DialConfig", 1)
	client.AssertExpectations(t)
	connection.AssertExpectations(t)
	channel.AssertExpectations(t)
	confirmation.AssertExpectations(t)
}

func TestFailRestoreConnectionDuringPublishing(t *testing.T) {
	client, connection, _, _ := setupMocksForSuccessfulPublish()

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)
	client.AssertNumberOfCalls(t, "DialConfig", 1)

	connection.On("IsClosed").Return(true)

	resetCall(t, client.ExpectedCalls, "DialConfig")
	client.On("DialConfig", connectURL, mock.Anything).Return(nil, errors.New("simulated connection error"))

	_ = publisher.Publish(context.Background(), makePublishMessage())
	client.AssertNumberOfCalls(t, "DialConfig", 1)
}

func TestErrCreatingChannel(t *testing.T) {
	client, connection, _, _ := setupMocksForSuccessfulPublish()

	resetCall(t, connection.ExpectedCalls, "Channel")
	connection.On("Channel").Return(nil, errors.New("simulated error creating channel"))

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())
	assert.EqualError(t, err, "simulated error creating channel")
}

func TestErrSettingChannelConfirmMode(t *testing.T) {
	client, _, channel, _ := setupMocksForSuccessfulPublish()

	resetCall(t, channel.ExpectedCalls, "Confirm")
	channel.On("Confirm", false).Return(errors.New("simulated error setting channel confirm mode"))

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())
	assert.EqualError(t, err, "simulated error setting channel confirm mode")
}

func TestErrPublishing(t *testing.T) {
	client, connection, _, _ := setupMocksForSuccessfulPublish()

	channel := mockChannel{}
	channel.On("Confirm", false).Return(nil)
	channel.On("PublishWithDeferredConfirmWithContext", mock.Anything, exchange, routingKey, true, false, mock.MatchedBy(isPersistentDeliverMode)).Return(nil, errors.New("simulated error publishing"))
	channel.On("Close").Return(nil)
	resetCall(t, connection.ExpectedCalls, "Channel")
	connection.On("Channel").Return(&channel, nil)

	publisher, err := NewConnection(zap.NewNop(), client, makeDialConfig())
	require.NoError(t, err)

	err = publisher.Publish(context.Background(), makePublishMessage())
	assert.EqualError(t, err, "error publishing message\nsimulated error publishing")
}

func setupMocksForSuccessfulPublish() (*mockClient, *mockConnection, *mockChannel, *mockDeferredConfirmation) {
	connection := mockConnection{}
	client := mockClient{
		conn: &connection,
	}
	channel := mockChannel{}
	confirmation := mockDeferredConfirmation{}

	client.On("DialConfig", mock.Anything).Return(&connection, nil)
	connection.On("ReconnectIfUnhealthy").Return(nil)
	connection.On("NotifyClose", mock.Anything).Return(make(chan *amqp.Error))
	connection.On("Channel").Return(&channel, nil)

	channel.On("Confirm", false).Return(nil)
	channel.On("PublishWithDeferredConfirmWithContext", mock.Anything, exchange, routingKey, true, false, mock.MatchedBy(isPersistentDeliverMode)).Return(&confirmation, nil)
	channel.On("Close").Return(nil)

	confirmationChan := make(chan struct{}, 1)
	confirmationChan <- struct{}{}
	var confirmationChanRet <-chan struct{} = confirmationChan
	confirmation.On("Done").Return(confirmationChanRet)
	confirmation.On("Acked").Return(true)

	return &client, &connection, &channel, &confirmation
}

func isPersistentDeliverMode(p amqp.Publishing) bool {
	return p.DeliveryMode == amqp.Persistent
}

func resetCall(t *testing.T, calls []*mock.Call, methodName string) {
	for _, call := range calls {
		if call.Method == methodName {
			call.Unset()
			return
		}
	}
	t.Errorf("Faild to reset method %s", methodName)
	t.FailNow()
}

type mockClient struct {
	mock.Mock
	conn *mockConnection
}

func (m *mockClient) DialConfig(config rabbitmq.DialConfig) (rabbitmq.Connection, error) {
	args := m.Called(config)

	m.conn.NotifyClose(make(chan *amqp.Error, 1))
	if connection := args.Get(0); connection != nil {
		return connection.(rabbitmq.Connection), args.Error(1)
	}
	return nil, args.Error(1)
}

type mockConnection struct {
	mock.Mock
}

func (m *mockConnection) ReconnectIfUnhealthy() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockConnection) IsClosed() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockConnection) Channel() (rabbitmq.Channel, error) {
	args := m.Called()
	if channel := args.Get(0); channel != nil {
		return channel.(rabbitmq.Channel), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockConnection) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	args := m.Called(receiver)
	return args.Get(0).(chan *amqp.Error)
}

func (m *mockConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockChannel struct {
	mock.Mock
}

func (m *mockChannel) Confirm(noWait bool) error {
	args := m.Called(noWait)
	return args.Error(0)
}

func (m *mockChannel) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (rabbitmq.DeferredConfirmation, error) {
	args := m.Called(ctx, exchange, key, mandatory, immediate, msg)
	if confirmation := args.Get(0); confirmation != nil {
		return confirmation.(rabbitmq.DeferredConfirmation), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockChannel) IsClosed() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockDeferredConfirmation struct {
	mock.Mock
}

func (m *mockDeferredConfirmation) Done() <-chan struct{} {
	args := m.Called()
	return args.Get(0).(<-chan struct{})
}

func (m *mockDeferredConfirmation) Acked() bool {
	args := m.Called()
	return args.Bool(0)
}

func makePublishMessage() Message {
	return Message{
		Exchange:   exchange,
		RoutingKey: routingKey,
		Body:       make([]byte, 1),
	}
}

func makeDialConfig() DialConfig {
	return DialConfig{
		DialConfig: rabbitmq.DialConfig{
			URL: connectURL,
		},
		PublishConfirmationTimeout: time.Millisecond * 20,
		Durable:                    true,
	}
}

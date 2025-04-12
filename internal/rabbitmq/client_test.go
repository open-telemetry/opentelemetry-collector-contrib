// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmq

import (
	"context"
	"crypto/tls"
	"runtime"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type MockConnection struct {
	mock.Mock
}

func (m *MockConnection) ReconnectIfUnhealthy() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConnection) IsClosed() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockConnection) Channel() (Channel, error) {
	args := m.Called()
	return args.Get(0).(Channel), args.Error(1)
}

func (m *MockConnection) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	args := m.Called(receiver)
	return args.Get(0).(chan *amqp.Error)
}

func (m *MockConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockChannel struct {
	mock.Mock
}

func (m *MockChannel) Confirm(noWait bool) error {
	args := m.Called(noWait)
	return args.Error(0)
}

func (m *MockChannel) PublishWithDeferredConfirmWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (DeferredConfirmation, error) {
	args := m.Called(ctx, exchange, key, mandatory, immediate, msg)
	return args.Get(0).(DeferredConfirmation), args.Error(1)
}

func (m *MockChannel) IsClosed() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

type MockDeferredConfirmation struct {
	mock.Mock
}

func (m *MockDeferredConfirmation) Done() <-chan struct{} {
	args := m.Called()
	return args.Get(0).(chan struct{})
}

func (m *MockDeferredConfirmation) Acked() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestDialConfig(t *testing.T) {
	logger := zap.NewNop()
	client := NewAmqpClient(logger)

	config := DialConfig{
		URL:               "amqp://guest:guest@localhost:5672/",
		Vhost:             "/",
		Auth:              &amqp.PlainAuth{Username: "guest", Password: "guest"},
		ConnectionTimeout: 10 * time.Second,
		Heartbeat:         10 * time.Second,
		TLS:               &tls.Config{},
		ConnectionName:    "test-connection",
	}

	conn, err := client.DialConfig(config)
	if runtime.GOOS == "windows" {
		assert.ErrorContains(t, err, "No connection could be made")
	} else {
		assert.ErrorContains(t, err, "connect: connection refused")
	}
	assert.NotNil(t, conn)
}

func TestReconnectIfUnhealthy(t *testing.T) {
	connection := &connectionHolder{
		logger:           zap.NewNop(),
		connLock:         &sync.Mutex{},
		connectionErrors: make(chan *amqp.Error, 1),
		url:              "amqp://guest:guest@localhost:5672/",
		config: amqp.Config{
			Vhost: "/",
		},
	}

	connection.connectionErrors <- &amqp.Error{
		Code:    0,
		Reason:  "mock error",
		Server:  false,
		Recover: false,
	}

	err := connection.ReconnectIfUnhealthy()
	if runtime.GOOS == "windows" {
		assert.ErrorContains(t, err, "No connection could be made")
	} else {
		assert.ErrorContains(t, err, "connect: connection refused")
	}
}

func TestIsConnected(t *testing.T) {
	logger := zap.NewNop()
	connection := &connectionHolder{
		logger:   logger,
		connLock: &sync.Mutex{},
	}

	assert.False(t, connection.isConnected())
}

func TestChannel(t *testing.T) {
	mockConn := new(MockConnection)
	mockChan := new(MockChannel)

	mockConn.On("Channel").Return(mockChan, nil)
	mockChan.On("Confirm", false).Return(nil)
	mockChan.On("PublishWithDeferredConfirmWithContext", mock.Anything, "exchange", "key", false, false, mock.Anything).Return(new(MockDeferredConfirmation), nil)
	mockChan.On("IsClosed").Return(false)
	mockChan.On("Close").Return(nil)

	channel, err := mockConn.Channel()
	assert.NoError(t, err)
	assert.NotNil(t, channel)

	err = channel.Confirm(false)
	assert.NoError(t, err)

	ctx := context.Background()
	deferredConf, err := channel.PublishWithDeferredConfirmWithContext(ctx, "exchange", "key", false, false, amqp.Publishing{})
	assert.NoError(t, err)
	assert.NotNil(t, deferredConf)

	assert.False(t, channel.IsClosed())

	err = channel.Close()
	assert.NoError(t, err)

	mockConn.AssertExpectations(t)
	mockChan.AssertExpectations(t)
}

func TestPublishWithDeferredConfirmWithContext(t *testing.T) {
	mockChan := new(MockChannel)
	mockDefConf := new(MockDeferredConfirmation)
	ctx := context.Background()

	mockChan.On("PublishWithDeferredConfirmWithContext", ctx, "exchange", "key", false, false, mock.Anything).Return(mockDefConf, nil)

	deferredConf, err := mockChan.PublishWithDeferredConfirmWithContext(ctx, "exchange", "key", false, false, amqp.Publishing{})
	assert.NoError(t, err)
	assert.NotNil(t, deferredConf)

	mockChan.AssertExpectations(t)
	mockDefConf.AssertExpectations(t)
}

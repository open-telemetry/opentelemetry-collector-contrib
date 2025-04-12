// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	consumer "go.opentelemetry.io/collector/consumer"
)

type mockLogsDataConsumer struct {
	mock.Mock
}

// ConsumeLogsJSON provides a mock function with given fields: ctx, json
func (_m *mockLogsDataConsumer) consumeLogsJSON(ctx context.Context, json []byte) error {
	ret := _m.Called(ctx, json)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte) error); ok {
		r0 = rf(ctx, json)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetNextLogsConsumer provides a mock function with given fields: nextLogsConsumer
func (_m *mockLogsDataConsumer) setNextLogsConsumer(nextLogsConsumer consumer.Logs) {
	_m.Called(nextLogsConsumer)
}

func newMockLogsDataConsumer() *mockLogsDataConsumer {
	logsDataConsumer := &mockLogsDataConsumer{}
	logsDataConsumer.On("consumeLogsJSON", mock.Anything, mock.Anything).Return(nil)
	return logsDataConsumer
}

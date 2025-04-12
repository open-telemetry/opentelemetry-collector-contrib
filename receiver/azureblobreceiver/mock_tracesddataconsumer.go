// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	consumer "go.opentelemetry.io/collector/consumer"
)

type mockTracesDataConsumer struct {
	mock.Mock
}

// ConsumeTracesJSON provides a mock function with given fields: ctx, json
func (_m *mockTracesDataConsumer) consumeTracesJSON(ctx context.Context, json []byte) error {
	ret := _m.Called(ctx, json)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte) error); ok {
		r0 = rf(ctx, json)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetNextTracesConsumer provides a mock function with given fields: nextracesConsumer
func (_m *mockTracesDataConsumer) setNextTracesConsumer(nextracesConsumer consumer.Traces) {
	_m.Called(nextracesConsumer)
}

func newMockTracesDataConsumer() *mockTracesDataConsumer {
	tracesDataConsumer := &mockTracesDataConsumer{}
	tracesDataConsumer.On("consumeTracesJSON", mock.Anything, mock.Anything).Return(nil)
	return tracesDataConsumer
}

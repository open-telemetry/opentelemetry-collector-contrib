// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockCustomEventHandler is a mock of CustomEventHandler interface.
type MockCustomEventHandler[T any] struct {
	ctrl     *gomock.Controller
	recorder *MockCustomEventHandlerMockRecorder[T]
	isgomock struct{}
}

// MockCustomEventHandlerMockRecorder is the mock recorder for MockCustomEventHandler.
type MockCustomEventHandlerMockRecorder[T any] struct {
	mock *MockCustomEventHandler[T]
}

// NewMockCustomEventHandler creates a new mock instance.
func NewMockCustomEventHandler[T any](ctrl *gomock.Controller) *MockCustomEventHandler[T] {
	mock := &MockCustomEventHandler[T]{ctrl: ctrl}
	mock.recorder = &MockCustomEventHandlerMockRecorder[T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCustomEventHandler[T]) EXPECT() *MockCustomEventHandlerMockRecorder[T] {
	return m.recorder
}

// GetNext mocks base method.
func (m *MockCustomEventHandler[T]) GetNext(ctx context.Context) (T, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNext", ctx)
	ret0, _ := ret[0].(T)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNext indicates an expected call of GetNext.
func (mr *MockCustomEventHandlerMockRecorder[T]) GetNext(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNext", reflect.TypeOf((*MockCustomEventHandler[T])(nil).GetNext), ctx)
}

// HasNext mocks base method.
func (m *MockCustomEventHandler[T]) HasNext() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasNext")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasNext indicates an expected call of HasNext.
func (mr *MockCustomEventHandlerMockRecorder[T]) HasNext() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasNext", reflect.TypeOf((*MockCustomEventHandler[T])(nil).HasNext))
}

// IsDryRun mocks base method.
func (m *MockCustomEventHandler[T]) IsDryRun() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsDryRun")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsDryRun indicates an expected call of IsDryRun.
func (mr *MockCustomEventHandlerMockRecorder[T]) IsDryRun() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsDryRun", reflect.TypeOf((*MockCustomEventHandler[T])(nil).IsDryRun))
}

// PostProcess mocks base method.
func (m *MockCustomEventHandler[T]) PostProcess(ctx context.Context, on T) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "PostProcess", ctx, on)
}

// PostProcess indicates an expected call of PostProcess.
func (mr *MockCustomEventHandlerMockRecorder[T]) PostProcess(ctx, on any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PostProcess", reflect.TypeOf((*MockCustomEventHandler[T])(nil).PostProcess), ctx, on)
}

// MockIterator is a mock of Iterator interface.
type MockIterator[T any] struct {
	ctrl     *gomock.Controller
	recorder *MockIteratorMockRecorder[T]
	isgomock struct{}
}

// MockIteratorMockRecorder is the mock recorder for MockIterator.
type MockIteratorMockRecorder[T any] struct {
	mock *MockIterator[T]
}

// NewMockIterator creates a new mock instance.
func NewMockIterator[T any](ctrl *gomock.Controller) *MockIterator[T] {
	mock := &MockIterator[T]{ctrl: ctrl}
	mock.recorder = &MockIteratorMockRecorder[T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockIterator[T]) EXPECT() *MockIteratorMockRecorder[T] {
	return m.recorder
}

// GetNext mocks base method.
func (m *MockIterator[T]) GetNext(ctx context.Context) (T, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetNext", ctx)
	ret0, _ := ret[0].(T)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetNext indicates an expected call of GetNext.
func (mr *MockIteratorMockRecorder[T]) GetNext(ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetNext", reflect.TypeOf((*MockIterator[T])(nil).GetNext), ctx)
}

// HasNext mocks base method.
func (m *MockIterator[T]) HasNext() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasNext")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasNext indicates an expected call of HasNext.
func (mr *MockIteratorMockRecorder[T]) HasNext() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasNext", reflect.TypeOf((*MockIterator[T])(nil).HasNext))
}

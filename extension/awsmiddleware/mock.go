// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"context"
	"net/http"

	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component"
)

// MockMiddlewareExtension mocks the Extension interface.
type MockMiddlewareExtension struct {
	component.StartFunc
	component.ShutdownFunc
	mock.Mock
}

var _ Extension = (*MockMiddlewareExtension)(nil)

func (m *MockMiddlewareExtension) Handlers() ([]RequestHandler, []ResponseHandler) {
	var requestHandlers []RequestHandler
	var responseHandlers []ResponseHandler
	args := m.Called()
	arg := args.Get(0)
	if arg != nil {
		requestHandlers = arg.([]RequestHandler)
	}
	arg = args.Get(1)
	if arg != nil {
		responseHandlers = arg.([]ResponseHandler)
	}
	return requestHandlers, responseHandlers
}

// MockHandler mocks the functions for both
// RequestHandler and ResponseHandler.
type MockHandler struct {
	mock.Mock
}

var (
	_ RequestHandler  = (*MockHandler)(nil)
	_ ResponseHandler = (*MockHandler)(nil)
)

func (m *MockHandler) ID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockHandler) Position() HandlerPosition {
	args := m.Called()
	return args.Get(0).(HandlerPosition)
}

func (m *MockHandler) HandleRequest(ctx context.Context, r *http.Request) {
	m.Called(ctx, r)
}

func (m *MockHandler) HandleResponse(ctx context.Context, r *http.Response) {
	m.Called(ctx, r)
}

// MockExtensionsHost only mocks the GetExtensions function.
// All other functions are ignored and will panic if called.
type MockExtensionsHost struct {
	component.Host
	mock.Mock
}

func (m *MockExtensionsHost) GetExtensions() map[component.ID]component.Component {
	args := m.Called()
	arg := args.Get(0)
	if arg == nil {
		return nil
	}
	return arg.(map[component.ID]component.Component)
}

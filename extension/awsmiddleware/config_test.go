// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

type testMiddlewareExtension struct {
	component.StartFunc
	component.ShutdownFunc
	requestHandlers  []RequestHandler
	responseHandlers []ResponseHandler
}

var _ Extension = (*testMiddlewareExtension)(nil)

func (t *testMiddlewareExtension) RequestHandlers() []RequestHandler {
	return t.requestHandlers
}

func (t *testMiddlewareExtension) ResponseHandlers() []ResponseHandler {
	return t.responseHandlers
}

func TestGetMiddleware(t *testing.T) {
	id := component.NewID("test")
	cfg := &Config{MiddlewareID: id}
	nopExtension, err := extensiontest.NewNopBuilder().Create(context.Background(), extensiontest.NewNopCreateSettings())
	require.Error(t, err)
	testCases := map[string]struct {
		extensions map[component.ID]component.Component
		wantErr    error
	}{
		"WithNoExtensions": {
			extensions: map[component.ID]component.Component{},
			wantErr:    errNotFound,
		},
		"WithNonMiddlewareExtension": {
			extensions: map[component.ID]component.Component{id: nopExtension},
			wantErr:    errNotMiddleware,
		},
		"WithMiddlewareExtension": {
			extensions: map[component.ID]component.Component{id: &testMiddlewareExtension{}},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := cfg.GetMiddleware(testCase.extensions)
			if testCase.wantErr != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, testCase.wantErr)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
			}
		})
	}
}

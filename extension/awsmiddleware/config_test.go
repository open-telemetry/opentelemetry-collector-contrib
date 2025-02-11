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

func TestGetConfigurer(t *testing.T) {
	id := component.MustNewID("test")
	nopFactory := extensiontest.NewNopFactory()
	nopExtension, err := nopFactory.Create(context.Background(), extensiontest.NewNopSettings(), nopFactory.CreateDefaultConfig())
	require.NoError(t, err)
	middlewareExtension := new(MockMiddlewareExtension)
	middlewareExtension.On("Handlers").Return(nil, nil)
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
			extensions: map[component.ID]component.Component{id: middlewareExtension},
		},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := GetConfigurer(testCase.extensions, id)
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

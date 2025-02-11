// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestTryConfigure(t *testing.T) {
	testCases := []SDKVersion{SDKv1(&request.Handlers{}), SDKv2(&aws.Config{})}
	for _, testCase := range testCases {
		id := component.MustNewID("test")
		host := new(MockExtensionsHost)
		host.On("GetExtensions").Return(nil).Once()
		core, observed := observer.New(zap.DebugLevel)
		logger := zap.New(core)
		TryConfigure(logger, host, id, testCase)
		require.Len(t, observed.FilterLevelExact(zap.ErrorLevel).TakeAll(), 1)

		extensions := map[component.ID]component.Component{}
		host.On("GetExtensions").Return(extensions)
		handler := new(MockHandler)
		handler.On("ID").Return("test")
		handler.On("Position").Return(HandlerPosition(-1))
		extension := new(MockMiddlewareExtension)
		extension.On("Handlers").Return([]RequestHandler{handler}, nil).Once()
		extensions[id] = extension
		TryConfigure(logger, host, id, testCase)
		require.Len(t, observed.FilterLevelExact(zap.WarnLevel).TakeAll(), 1)

		extension.On("Handlers").Return(nil, nil).Once()
		TryConfigure(logger, host, id, testCase)
		require.Len(t, observed.FilterLevelExact(zap.DebugLevel).TakeAll(), 1)
	}
}

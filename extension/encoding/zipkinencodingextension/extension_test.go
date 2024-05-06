// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestExtension_Start(t *testing.T) {
	tests := []struct {
		name         string
		getExtension func() (extension.Extension, error)
		expectedErr  string
	}{
		{
			name: "zipkinJSON",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Protocol = "zipkin_json"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "zipkinProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Protocol = "zipkin_proto"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "zipkinProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Protocol = "zipkin_thrift"
				cfg.(*Config).Version = "v1"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "zipkinThriftVersion_invalid",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Protocol = "zipkin_thrift"
				cfg.(*Config).Version = "v2"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
			expectedErr: "unsupported version: \"v2\"",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ext, err := test.getExtension()
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
			err = ext.Start(context.Background(), componenttest.NewNopHost())
			if test.expectedErr != "" && err != nil {
				require.ErrorContains(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

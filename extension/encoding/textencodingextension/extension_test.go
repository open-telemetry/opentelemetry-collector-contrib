// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"

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
			name: "text",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), factory.CreateDefaultConfig())
			},
		},
		{
			name: "text_gbk",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Encoding = "gbk"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
		},
		{
			name: "text_blabla",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				cfg.(*Config).Encoding = "blabla"
				return factory.CreateExtension(context.Background(), extensiontest.NewNopCreateSettings(), cfg)
			},
			expectedErr: "unsupported encoding 'blabla'",
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

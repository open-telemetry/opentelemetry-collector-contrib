// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingencodingextension

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
			name: "skywalkingProtobuf",
			getExtension: func() (extension.Extension, error) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()
				return factory.Create(context.Background(), extensiontest.NewNopSettings(factory.Type()), cfg)
			},
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

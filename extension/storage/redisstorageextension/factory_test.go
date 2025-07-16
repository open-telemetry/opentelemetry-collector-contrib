// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisstorageextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()

	tests := []struct {
		name   string
		config *Config
	}{
		{
			name: "Default",
			config: func() *Config {
				return &Config{
					Endpoint: "localhost:6379",
					TLS: configtls.ClientConfig{
						Insecure: true,
					},
				}
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, err := f.Create(
				context.Background(),
				extensiontest.NewNopSettings(f.Type()),
				test.config,
			)
			require.NoError(t, err)
			require.NotNil(t, e)
			ctx := context.Background()
			require.NoError(t, e.Start(ctx, componenttest.NewNopHost()))
			require.NoError(t, e.Shutdown(ctx))
		})
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarderextension

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	expectType := component.MustNewType("http_forwarder")
	require.Equal(t, expectType, f.Type())

	cfg := f.CreateDefaultConfig().(*Config)
	require.Equal(t, ":6060", cfg.Ingress.Endpoint)
	require.Equal(t, 10*time.Second, cfg.Egress.Timeout)

	tests := []struct {
		name           string
		config         *Config
		wantErr        bool
		wantErrMessage string
	}{
		{
			name:           "Default config",
			config:         cfg,
			wantErr:        true,
			wantErrMessage: "'egress.endpoint' config option cannot be empty",
		},
		{
			name:           "Invalid config",
			config:         &Config{Egress: confighttp.ClientConfig{Endpoint: "123.456.7.89:9090"}},
			wantErr:        true,
			wantErrMessage: "enter a valid URL for 'egress.endpoint': parse \"123.456.7.89:9090\": first path segment in URL cannot",
		},
		{
			name:   "Valid config",
			config: &Config{Egress: confighttp.ClientConfig{Endpoint: "localhost:9090"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, err := f.Create(
				context.Background(),
				extensiontest.NewNopSettings(),
				test.config,
			)
			if test.wantErr {
				if test.wantErrMessage != "" {
					require.Contains(t, err.Error(), test.wantErrMessage)
				}
				require.Error(t, err)
				require.Nil(t, e)
			} else {
				require.NoError(t, err)
				require.NotNil(t, e)
				ctx := context.Background()
				require.NoError(t, e.Start(ctx, componenttest.NewNopHost()))
				require.NoError(t, e.Shutdown(ctx))
			}
		})
	}
}

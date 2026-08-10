// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarderextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	expectType := component.MustNewType("http_forwarder")
	require.Equal(t, expectType, f.Type())

	cfg := f.CreateDefaultConfig().(*Config)
	require.Equal(t, ":6060", cfg.Ingress.NetAddr.Endpoint)
	require.Equal(t, 10*time.Second, cfg.Egress.Timeout)

	invalidEgressConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	invalidEgressConfig.MaxIdleConns = 0
	invalidEgressConfig.IdleConnTimeout = 0
	invalidEgressConfig.ForceAttemptHTTP2 = false
	invalidEgressConfig.Endpoint = "123.456.7.89:9090"

	validIngressConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	validIngressConfig.WriteTimeout = 0
	validIngressConfig.ReadHeaderTimeout = 0
	validIngressConfig.IdleTimeout = 0
	validIngressConfig.KeepAlivesEnabled = false
	validIngressConfig.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  ":0",
	}

	validEgressConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	validEgressConfig.MaxIdleConns = 0
	validEgressConfig.IdleConnTimeout = 0
	validEgressConfig.ForceAttemptHTTP2 = false
	validEgressConfig.Endpoint = "localhost:9090"

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
			config:         &Config{Egress: invalidEgressConfig},
			wantErr:        true,
			wantErrMessage: "enter a valid URL for 'egress.endpoint': parse \"123.456.7.89:9090\": first path segment in URL cannot",
		},
		{
			name: "Valid config",
			config: &Config{
				Ingress: validIngressConfig,
				Egress:  validEgressConfig,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e, err := f.Create(
				t.Context(),
				extensiontest.NewNopSettings(expectType),
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
				ctx := t.Context()
				require.NoError(t, e.Start(ctx, componenttest.NewNopHost()))
				require.NoError(t, e.Shutdown(ctx))
			}
		})
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarderextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarderextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	maxIdleConns := 42
	idleConnTimeout := 80 * time.Second

	egressCfg := confighttp.NewDefaultClientConfig()
	egressCfg.Endpoint = "http://target/"
	egressCfg.Headers = configopaque.MapList{
		{Name: "otel_http_forwarder", Value: "dev"},
	}
	egressCfg.Timeout = 5 * time.Second
	// max_idle_conns and idle_conn_timeout are deprecated keys; unmarshal them
	// through confmap (rather than setting the fields directly) so that
	// egressCfg picks up the same deprecation-warning bookkeeping that
	// loading testdata/config.yaml produces below.
	require.NoError(t, confmap.NewFromStringMap(map[string]any{
		"max_idle_conns":    maxIdleConns,
		"idle_conn_timeout": idleConnTimeout,
	}).Unmarshal(&egressCfg))

	ingressCfg := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	ingressCfg.WriteTimeout = 0
	ingressCfg.ReadHeaderTimeout = 0
	ingressCfg.IdleTimeout = 0           //nolint:staticcheck // SA1019: see TODO above
	ingressCfg.KeepAlivesEnabled = false //nolint:staticcheck // SA1019: see TODO above
	ingressCfg.NetAddr = confignet.AddrConfig{
		Transport: "tcp",
		Endpoint:  "http://localhost:7070",
	}

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				Ingress: ingressCfg,
				Egress:  egressCfg,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			assert.NoError(t, confmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

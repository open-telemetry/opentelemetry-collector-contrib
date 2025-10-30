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
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

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
	egressCfg.MaxIdleConns = maxIdleConns
	egressCfg.IdleConnTimeout = idleConnTimeout
	egressCfg.Timeout = 5 * time.Second

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
				Ingress: confighttp.ServerConfig{
					Endpoint: "http://localhost:7070",
				},
				Egress: egressCfg,
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
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpforwarder

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	maxIdleConns := 42
	idleConnTimeout := 80 * time.Second

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
				Ingress: confighttp.HTTPServerSettings{
					Endpoint: "http://localhost:7070",
				},
				Egress: confighttp.HTTPClientSettings{
					Endpoint: "http://target/",
					Headers: map[string]configopaque.String{
						"otel_http_forwarder": "dev",
					},
					MaxIdleConns:    &maxIdleConns,
					IdleConnTimeout: &idleConnTimeout,
					Timeout:         5 * time.Second,
				},
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

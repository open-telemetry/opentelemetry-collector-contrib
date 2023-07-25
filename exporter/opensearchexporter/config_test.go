// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := newDefaultConfig()
	defaultCfg.(*Config).Endpoint = "https://opensearch.example.com:9200"
	maxIdleConns := 100
	idleConnTimeout := 90 * time.Second

	tests := []struct {
		id                   component.ID
		expected             component.Config
		configValidateAssert assert.ErrorAssertionFunc
	}{
		{
			id:                   component.NewIDWithName(typeStr, ""),
			expected:             defaultCfg,
			configValidateAssert: assert.NoError,
		},
		{
			id: component.NewIDWithName(typeStr, "trace"),
			expected: &Config{
				Dataset:   "ngnix",
				Namespace: "eu",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "https://opensearch.example.com:9200",
					Timeout:  2 * time.Minute,
					Headers: map[string]configopaque.String{
						"myheader": "test",
					},
					MaxIdleConns:    &maxIdleConns,
					IdleConnTimeout: &idleConnTimeout,
					Auth:            &configauth.Authentication{AuthenticatorID: component.NewID("sample_basic_auth")},
				},
				RetrySettings: exporterhelper.RetrySettings{
					Enabled:             true,
					InitialInterval:     100 * time.Millisecond,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
					Multiplier:          1.5,
					RandomizationFactor: 0.5,
				},
			},
			configValidateAssert: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			vv := component.ValidateConfig(cfg)
			tt.configValidateAssert(t, vv)
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

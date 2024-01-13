// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package f5cloudexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	otlphttp "go.opentelemetry.io/collector/exporter/otlphttpexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "allsettings").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	actualCfg := cfg.(*Config)
	expectedCfg := &Config{
		Config: otlphttp.Config{
			RetryConfig: configretry.BackOffConfig{
				Enabled:             true,
				InitialInterval:     10 * time.Second,
				MaxInterval:         1 * time.Minute,
				MaxElapsedTime:      10 * time.Minute,
				RandomizationFactor: backoff.DefaultRandomizationFactor,
				Multiplier:          backoff.DefaultMultiplier,
			},
			QueueConfig: exporterhelper.QueueSettings{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint:        "https://f5cloud",
				ReadBufferSize:  123,
				WriteBufferSize: 345,
				Timeout:         time.Second * 10,
				Headers: map[string]configopaque.String{
					"User-Agent": "opentelemetry-collector-contrib {{version}}",
				},
				Compression: "gzip",
			},
		},
		Source: "dev",
		AuthConfig: AuthConfig{
			CredentialFile: "/etc/creds/key.json",
			Audience:       "exampleaudience",
		},
	}
	// testing function equality is not supported in Go hence these will be ignored for this test
	expectedCfg.HTTPClientSettings.CustomRoundTripper = nil
	actualCfg.HTTPClientSettings.CustomRoundTripper = nil
	assert.Equal(t, expectedCfg, actualCfg)
}

func TestConfig_sanitize(t *testing.T) {
	const validEndpoint = "https://validendpoint.local"
	const validSource = "tests"

	type fields struct {
		Endpoint       string
		Source         string
		CredentialFile string
		Audience       string
	}
	tests := []struct {
		name         string
		fields       fields
		errorMessage string
		shouldError  bool
	}{
		{
			name: "Test missing endpoint",
			fields: fields{
				Endpoint: "",
			},
			errorMessage: "missing required \"endpoint\" setting",
			shouldError:  true,
		},
		{
			name: "Test invalid endpoint",
			fields: fields{
				Endpoint: "this://is:an:invalid:endpoint.com",
			},
			errorMessage: "",
			shouldError:  true,
		},
		{
			name: "Test credential file not provided",
			fields: fields{
				Endpoint:       validEndpoint,
				Source:         validSource,
				CredentialFile: "",
			},
			errorMessage: "missing required \"f5cloud_auth.credential_file\" setting",
			shouldError:  true,
		},
		{
			name: "Test non-existent credential file",
			fields: fields{
				Endpoint:       validEndpoint,
				Source:         validSource,
				CredentialFile: "non-existent cred file",
			},
			errorMessage: "the provided \"f5cloud_auth.credential_file\" does not exist",
			shouldError:  true,
		},
		{
			name: "Test missing source",
			fields: fields{
				Endpoint: validEndpoint,
				Source:   "",
			},
			errorMessage: "missing required \"source\" setting",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Endpoint = tt.fields.Endpoint
			cfg.Source = tt.fields.Source
			cfg.AuthConfig = AuthConfig{
				CredentialFile: tt.fields.CredentialFile,
				Audience:       tt.fields.Audience,
			}

			err := cfg.sanitize()
			if (err != nil) != tt.shouldError {
				t.Errorf("sanitize() error = %v, shouldError %v", err, tt.shouldError)
				return
			}

			if tt.shouldError {
				assert.Error(t, err)
				if len(tt.errorMessage) != 0 {
					assert.Equal(t, tt.errorMessage, err.Error())
				}
			}
		})
	}

	t.Run("Test audience is set from endpoint when not provided", func(t *testing.T) {
		factory := NewFactory()
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Endpoint = validEndpoint
		cfg.Source = validSource
		cfg.AuthConfig = AuthConfig{
			CredentialFile: "testdata/empty_credential_file.json",
			Audience:       "",
		}

		err := cfg.sanitize()
		assert.NoError(t, err)
		assert.Equal(t, validEndpoint, cfg.AuthConfig.Audience)
	})

	t.Run("Test audience is not set from endpoint when provided", func(t *testing.T) {
		factory := NewFactory()
		cfg := factory.CreateDefaultConfig().(*Config)
		cfg.Endpoint = validEndpoint
		cfg.Source = validSource
		cfg.AuthConfig = AuthConfig{
			CredentialFile: "testdata/empty_credential_file.json",
			Audience:       "tests",
		}

		err := cfg.sanitize()
		assert.NoError(t, err)
		assert.Equal(t, "tests", cfg.AuthConfig.Audience)
	})
}

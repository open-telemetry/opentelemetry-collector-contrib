// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.URL = "https://sentry.io"
				cfg.OrgSlug = "my-org"
				cfg.AuthToken = configopaque.String("test-auth-token-12345")
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_routing"),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.URL = "https://sentry.io"
				cfg.OrgSlug = "my-org"
				cfg.AuthToken = configopaque.String("test-auth-token-12345")
				cfg.AutoCreateProjects = true
				cfg.Routing = RoutingConfig{
					ProjectFromAttribute: "service.name",
					AttributeToProjectMapping: map[string]string{
						"api-service": "backend-api",
						"web-service": "frontend-web",
					},
				}
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "full"),
			expected: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.URL = "https://sentry.example.com"
				cfg.OrgSlug = "example-org"
				cfg.AuthToken = configopaque.String("full-test-token")
				cfg.Timeout = 20 * time.Second
				cfg.TLS.Insecure = true
				cfg.TimeoutConfig.Timeout = 45 * time.Second
				cfg.AutoCreateProjects = true
				cfg.Routing = RoutingConfig{
					ProjectFromAttribute: "deployment.environment.name",
					AttributeToProjectMapping: map[string]string{
						"production": "prod-project",
						"staging":    "stage-project",
					},
				}
				return cfg
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
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

func TestValidateRejectsInvalidProjectSlug(t *testing.T) {
	t.Parallel()

	cfg := minimalValidConfig()
	cfg.Routing.AttributeToProjectMapping = map[string]string{
		"api-service": "invalid slug",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project slug")
}

func TestValidateRejectsUpperCaseProjectSlug(t *testing.T) {
	t.Parallel()

	cfg := minimalValidConfig()
	cfg.Routing.AttributeToProjectMapping = map[string]string{
		"api-service": "BackendAPI",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project slug")
}

func TestValidateRejectsNumericOnlyProjectSlug(t *testing.T) {
	t.Parallel()

	cfg := minimalValidConfig()
	cfg.Routing.AttributeToProjectMapping = map[string]string{
		"api-service": "12345",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project slug")
}

func TestValidateRejectsEmptyAttributeValue(t *testing.T) {
	t.Parallel()

	cfg := minimalValidConfig()
	cfg.Routing.AttributeToProjectMapping = map[string]string{
		"": "backend-api",
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty attribute value")
}

func TestValidateRejectsTooManyProjectMappings(t *testing.T) {
	t.Parallel()

	cfg := minimalValidConfig()
	cfg.Routing.AttributeToProjectMapping = make(map[string]string, maxProjects+1)

	for i := range maxProjects + 1 {
		attr := fmt.Sprintf("service-%d", i)
		project := fmt.Sprintf("project-%d", i)
		cfg.Routing.AttributeToProjectMapping[attr] = project
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not define more than")
}

func minimalValidConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.URL = "https://sentry.io"
	cfg.OrgSlug = "my-org"
	cfg.AuthToken = configopaque.String("token")
	return cfg
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gitlabreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	serverConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig.ReadHeaderTimeout = 0
	serverConfig.IdleTimeout = 0
	serverConfig.KeepAlivesEnabled = false
	serverConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  defaultEndpoint,
	}
	serverConfig.ReadTimeout = defaultReadTimeout
	serverConfig.WriteTimeout = defaultWriteTimeout
	expectedConfig := &Config{
		WebHook: WebHook{
			ServerConfig: serverConfig,
			Path:         defaultPath,
			HealthPath:   defaultHealthPath,
			GitlabHeaders: GitlabHeaders{
				Customizable: map[string]string{
					defaultUserAgentHeader:      "",
					defaultGitLabInstanceHeader: "https://gitlab.com",
				},
				Fixed: map[string]string{
					defaultGitLabWebhookUUIDHeader: "",
					defaultGitLabEventHeader:       "Pipeline Hook",
					defaultGitLabEventUUIDHeader:   "",
					defaultIdempotencyKeyHeader:    "",
				},
			},
			IncludeUserAttributes: false,
		},
	}

	assert.Equal(t, expectedConfig, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory

	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Len(t, cfg.Receivers, 2)

	serverConfig := confighttp.NewDefaultServerConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	serverConfig.ReadHeaderTimeout = 0
	serverConfig.IdleTimeout = 0
	serverConfig.KeepAlivesEnabled = false
	serverConfig.NetAddr = confignet.AddrConfig{
		Transport: confignet.TransportTypeTCP,
		Endpoint:  "localhost:8080",
	}
	serverConfig.ReadTimeout = 500 * time.Millisecond
	serverConfig.WriteTimeout = 500 * time.Millisecond
	expectedConfig := &Config{
		WebHook: WebHook{
			ServerConfig: serverConfig,
			Path:         "some/path",
			HealthPath:   "health/path",
			RequiredHeaders: map[string]configopaque.String{
				"key1-present": "value1-present",
			},
			GitlabHeaders: GitlabHeaders{
				Customizable: map[string]string{
					defaultUserAgentHeader:      "",
					defaultGitLabInstanceHeader: "https://gitlab.com",
				},
				Fixed: map[string]string{
					defaultGitLabWebhookUUIDHeader: "",
					defaultGitLabEventHeader:       "Pipeline Hook",
					defaultGitLabEventUUIDHeader:   "",
					defaultIdempotencyKeyHeader:    "",
				},
			},
		},
	}

	r0 := cfg.Receivers[component.NewID(metadata.Type)]

	assert.Equal(t, expectedConfig, r0)

	// r1 requires multiple headers and overwrites gitlab default headers
	expectedConfig.WebHook.RequiredHeaders = map[string]configopaque.String{
		"key1-present":      "value1-present",
		"key2-present":      "value2-present",
		"User-Agent":        "GitLab/1.2.3-custom-version",
		"X-Gitlab-Instance": "https://gitlab.self-hosted.xyz",
	}

	expectedConfig.WebHook.GitlabHeaders = GitlabHeaders{
		Customizable: map[string]string{
			defaultUserAgentHeader:      "GitLab/1.2.3-custom-version",
			defaultGitLabInstanceHeader: "https://gitlab.self-hosted.xyz",
		},
		Fixed: map[string]string{
			defaultGitLabWebhookUUIDHeader: "",
			defaultGitLabEventHeader:       "Pipeline Hook",
			defaultGitLabEventUUIDHeader:   "",
			defaultIdempotencyKeyHeader:    "",
		},
	}

	r1 := cfg.Receivers[component.NewIDWithName(metadata.Type, "customname")].(*Config)

	assert.Equal(t, expectedConfig, r1)
}

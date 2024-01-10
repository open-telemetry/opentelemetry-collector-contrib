// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatraceexporter

import (
	"path/filepath"
	"testing"

	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/apiconstants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	dtconfig "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &dtconfig.Config{
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{
			Enabled: false,
		},

		Tags:              []string{},
		DefaultDimensions: make(map[string]string),
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yml"))
	require.NoError(t, err)

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "defaults"),
			expected: &dtconfig.Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),

				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: apiconstants.GetDefaultOneAgentEndpoint(),
					Headers: map[string]configopaque.String{
						"Content-Type": "text/plain; charset=UTF-8",
						"User-Agent":   "opentelemetry-collector"},
				},
				Tags:              []string{},
				DefaultDimensions: make(map[string]string),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid"),
			expected: &dtconfig.Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),

				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://example.com/api/v2/metrics/ingest",
					Headers: map[string]configopaque.String{
						"Authorization": "Api-Token token",
						"Content-Type":  "text/plain; charset=UTF-8",
						"User-Agent":    "opentelemetry-collector"},
				},
				APIToken: "token",

				Prefix: "myprefix",

				Tags: []string{},
				DefaultDimensions: map[string]string{
					"dimension_example": "dimension_value",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "valid_tags"),
			expected: &dtconfig.Config{
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),

				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://example.com/api/v2/metrics/ingest",
					Headers: map[string]configopaque.String{
						"Authorization": "Api-Token token",
						"Content-Type":  "text/plain; charset=UTF-8",
						"User-Agent":    "opentelemetry-collector"},
				},
				APIToken: "token",

				Prefix: "myprefix",

				Tags:              []string{"tag_example=tag_value"},
				DefaultDimensions: make(map[string]string),
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "bad_endpoint"),
			errorMessage: "endpoint must start with https:// or http://",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_token"),
			errorMessage: "api_token is required if Endpoint is provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

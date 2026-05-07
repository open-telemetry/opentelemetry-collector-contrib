// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func TestInitTelemetrySettingsWithDeclarativeResourceConfig(t *testing.T) {
	schemaURL := "https://opentelemetry.io/schemas/1.38.0"
	settings, err := initTelemetrySettings(t.Context(), zap.NewNop(), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		Resource: config.ResourceConfig{
			Resource: otelconf.Resource{
				SchemaUrl: &schemaURL,
				Attributes: []otelconf.AttributeNameValue{
					{Name: "custom.bool", Value: true},
					{Name: "service.name", Value: "custom-supervisor"},
				},
				Detectors: &otelconf.Detectors{},
			},
		},
	})
	require.NoError(t, err)

	serviceName, ok := settings.Resource.Attributes().Get("service.name")
	require.True(t, ok)
	assert.Equal(t, "custom-supervisor", serviceName.AsString())

	customBool, ok := settings.Resource.Attributes().Get("custom.bool")
	require.True(t, ok)
	assert.True(t, customBool.Bool())

	_, ok = settings.Resource.Attributes().Get("service.instance.id")
	assert.True(t, ok)
}

func TestInitTelemetrySettingsWithLegacyNilResourceOverride(t *testing.T) {
	settings, err := initTelemetrySettings(t.Context(), zap.NewNop(), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		Resource: config.ResourceConfig{
			LegacyAttributes: map[string]any{
				"service.name": nil,
			},
		},
	})
	require.NoError(t, err)

	_, ok := settings.Resource.Attributes().Get("service.name")
	assert.False(t, ok)
	_, ok = settings.Resource.Attributes().Get("service.instance.id")
	assert.True(t, ok)
}

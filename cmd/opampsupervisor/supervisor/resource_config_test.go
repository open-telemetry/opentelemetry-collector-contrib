// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	xotelconf "go.opentelemetry.io/contrib/otelconf/x"
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

func TestBuildSupervisorResourceConfigAppliesDetectionDevelopment(t *testing.T) {
	cfg, resource, err := buildSupervisorResourceConfig(t.Context(), &config.ResourceConfig{
		DetectionDevelopment: &xotelconf.ExperimentalResourceDetection{
			Attributes: &xotelconf.IncludeExclude{
				Included: []string{"host.*"},
			},
			Detectors: []xotelconf.ExperimentalResourceDetector{
				{Host: xotelconf.ExperimentalHostResourceDetector{}},
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, cfg.Detectors)
	require.NotNil(t, cfg.SchemaUrl)
	_, ok := resource.Attributes().Get("host.name")
	assert.True(t, ok)
}

func TestBuildSupervisorResourceConfigAppliesProcessDetectionDevelopment(t *testing.T) {
	cfg, resource, err := buildSupervisorResourceConfig(t.Context(), &config.ResourceConfig{
		DetectionDevelopment: &xotelconf.ExperimentalResourceDetection{
			Attributes: &xotelconf.IncludeExclude{
				Included: []string{"process.command_args"},
			},
			Detectors: []xotelconf.ExperimentalResourceDetector{
				{Process: xotelconf.ExperimentalProcessResourceDetector{}},
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, cfg.Detectors)

	commandArgs, ok := resource.Attributes().Get("process.command_args")
	require.True(t, ok)
	require.Equal(t, pcommon.ValueTypeSlice, commandArgs.Type())
	require.Positive(t, commandArgs.Slice().Len())
}

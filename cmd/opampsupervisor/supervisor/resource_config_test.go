// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/service/telemetry/otelconftelemetry"
	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	xotelconf "go.opentelemetry.io/contrib/otelconf/x"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

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
			ResourceConfig: otelconftelemetry.ResourceConfig{
				Resource: otelconf.Resource{
					SchemaUrl: &schemaURL,
					Attributes: []otelconf.AttributeNameValue{
						{Name: "custom.bool", Value: true},
						{Name: "service.name", Value: "custom-supervisor"},
					},
					Detectors: &otelconf.Detectors{},
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
			ResourceConfig: otelconftelemetry.ResourceConfig{
				LegacyAttributes: map[string]any{
					"service.name": nil,
				},
			},
		},
	})
	require.NoError(t, err)

	_, ok := settings.Resource.Attributes().Get("service.name")
	assert.False(t, ok)
	_, ok = settings.Resource.Attributes().Get("service.instance.id")
	assert.True(t, ok)
}

func TestInitTelemetrySettingsWithHostResourceDetection(t *testing.T) {
	settings, err := initTelemetrySettings(t.Context(), zap.NewNop(), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		Resource: config.ResourceConfig{
			DetectionDevelopment: configoptional.Some(xotelconf.ExperimentalResourceDetection{
				Detectors: []xotelconf.ExperimentalResourceDetector{
					{Host: xotelconf.ExperimentalHostResourceDetector{}},
				},
			}),
		},
	})
	require.NoError(t, err)

	attrs := settings.Resource.Attributes()
	_, hasHostName := attrs.Get("host.name")
	_, hasOSType := attrs.Get("os.type")
	_, hasOSDescription := attrs.Get("os.description")
	assert.True(t, hasHostName || hasOSType || hasOSDescription)
}

func TestInitTelemetrySettingsStableDetectorsIgnored(t *testing.T) {
	settings, err := initTelemetrySettings(t.Context(), zap.NewNop(), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		Resource: config.ResourceConfig{
			ResourceConfig: otelconftelemetry.ResourceConfig{
				Resource: otelconf.Resource{
					Detectors: &otelconf.Detectors{
						Attributes: &otelconf.DetectorsAttributes{
							Included: []string{"host.*", "os.*"},
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	attrs := settings.Resource.Attributes()
	_, hasHostName := attrs.Get("host.name")
	_, hasOSType := attrs.Get("os.type")
	assert.False(t, hasHostName)
	assert.False(t, hasOSType)
}

func TestInitTelemetrySettingsResourceDetectionKeepsSupervisorServiceAttributes(t *testing.T) {
	settings, err := initTelemetrySettings(t.Context(), zap.NewNop(), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		Resource: config.ResourceConfig{
			ResourceConfig: otelconftelemetry.ResourceConfig{
				Resource: otelconf.Resource{
					Attributes: []otelconf.AttributeNameValue{
						{Name: "service.name", Value: "custom-supervisor"},
						{Name: "service.instance.id", Value: "configured-instance"},
					},
				},
			},
			DetectionDevelopment: configoptional.Some(xotelconf.ExperimentalResourceDetection{
				Detectors: []xotelconf.ExperimentalResourceDetector{
					{Service: xotelconf.ExperimentalServiceResourceDetector{}},
				},
			}),
		},
	})
	require.NoError(t, err)

	serviceName, ok := settings.Resource.Attributes().Get("service.name")
	require.True(t, ok)
	assert.Equal(t, "custom-supervisor", serviceName.AsString())

	instanceID, ok := settings.Resource.Attributes().Get("service.instance.id")
	require.True(t, ok)
	assert.Equal(t, "configured-instance", instanceID.AsString())
}

func TestInitTelemetrySettingsReturnsResourceDetectionErrors(t *testing.T) {
	originalNewExperimentalSDK := newExperimentalSDK
	t.Cleanup(func() {
		newExperimentalSDK = originalNewExperimentalSDK
	})
	newExperimentalSDK = func(...xotelconf.ConfigurationOption) (xotelconf.SDK, error) {
		return xotelconf.SDK{}, assert.AnError
	}

	_, err := initTelemetrySettings(t.Context(), zap.NewNop(), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		Resource: config.ResourceConfig{
			DetectionDevelopment: configoptional.Some(xotelconf.ExperimentalResourceDetection{
				Detectors: []xotelconf.ExperimentalResourceDetector{
					{Host: xotelconf.ExperimentalHostResourceDetector{}},
				},
			}),
		},
	})
	assert.ErrorIs(t, err, assert.AnError)
}

func TestInitTelemetrySettingsWarnsOnExperimentalConfigFileEnvVar(t *testing.T) {
	t.Setenv(experimentalConfigFileEnvVar, writeSDKConfigFile(t))

	core, logs := observer.New(zap.WarnLevel)
	_, err := initTelemetrySettings(t.Context(), zap.New(core), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
		Resource: config.ResourceConfig{
			DetectionDevelopment: configoptional.Some(xotelconf.ExperimentalResourceDetection{
				Detectors: []xotelconf.ExperimentalResourceDetector{
					{Host: xotelconf.ExperimentalHostResourceDetector{}},
				},
			}),
		},
	})
	require.NoError(t, err)

	warnings := logs.FilterFieldKey("env_var").All()
	require.Len(t, warnings, 1)
	assert.Equal(t, experimentalConfigFileEnvVar, warnings[0].ContextMap()["env_var"])
}

func TestInitTelemetrySettingsNoWarningWithoutResourceDetection(t *testing.T) {
	t.Setenv(experimentalConfigFileEnvVar, writeSDKConfigFile(t))

	core, logs := observer.New(zap.WarnLevel)
	_, err := initTelemetrySettings(t.Context(), zap.New(core), config.Telemetry{
		Logs: config.Logs{
			Level:            zap.InfoLevel,
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		},
	})
	require.NoError(t, err)

	assert.Zero(t, logs.Len())
}

// writeSDKConfigFile writes a minimal declarative configuration file and returns its path.
func writeSDKConfigFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sdk-config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("file_format: \"0.4\"\n"), 0o600))
	return path
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package agentcomponents // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"

import (
	"testing"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	implgzip "github.com/DataDog/datadog-agent/pkg/util/compression/impl-gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

func TestNewConfigComponent_WithOptions(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	set.Logger = zap.NewNop()

	tests := []struct {
		name    string
		options []ConfigOption
		verify  func(t *testing.T, config pkgconfigmodel.Config)
	}{
		{
			name:    "empty options",
			options: []ConfigOption{},
			verify: func(t *testing.T, config pkgconfigmodel.Config) {
				// Should just have basic viper config
				assert.NotNil(t, config)
			},
		},
		{
			name: "with API config",
			options: []ConfigOption{
				WithAPIConfig(&datadogconfig.Config{
					API: datadogconfig.APIConfig{
						Key:  configopaque.String("test-api-key"),
						Site: "datadoghq.eu",
					},
				}),
			},
			verify: func(t *testing.T, config pkgconfigmodel.Config) {
				assert.Equal(t, "test-api-key", config.GetString("api_key"))
				assert.Equal(t, "datadoghq.eu", config.GetString("site"))
			},
		},
		{
			name: "with log level",
			options: []ConfigOption{
				WithLogLevel(set),
			},
			verify: func(t *testing.T, config pkgconfigmodel.Config) {
				// The actual log level string format depends on zap's implementation
				logLevel := config.GetString("log_level")
				assert.NotEmpty(t, logLevel)
				assert.Contains(t, []string{"info", "INFO", "Level(6)"}, logLevel)
			},
		},
		{
			name: "with logs defaults",
			options: []ConfigOption{
				WithLogsDefaults(),
			},
			verify: func(t *testing.T, config pkgconfigmodel.Config) {
				assert.False(t, config.IsConfigured("logs_config.auditor_ttl"))
				assert.False(t, config.IsConfigured("logs_config.batch_max_content_size"))
				assert.False(t, config.IsConfigured("logs_config.use_v2_api"))
				assert.True(t, config.GetBool("logs_config.use_v2_api"))
			},
		},
		{
			name: "with custom config",
			options: []ConfigOption{
				WithCustomConfig("custom.setting", "custom-value", pkgconfigmodel.SourceFile),
				WithCustomConfig("custom.number", 42, pkgconfigmodel.SourceDefault),
			},
			verify: func(t *testing.T, config pkgconfigmodel.Config) {
				assert.Equal(t, "custom-value", config.GetString("custom.setting"))
				assert.Equal(t, 42, config.GetInt("custom.number"))
			},
		},
		{
			name: "multiple options combined",
			options: []ConfigOption{
				WithAPIConfig(&datadogconfig.Config{
					API: datadogconfig.APIConfig{
						Key:  configopaque.String("combined-api-key"),
						Site: "datadoghq.com",
					},
				}),
				WithLogLevel(set),
				WithLogsDefaults(),
				WithCustomConfig("module.name", "test-module", pkgconfigmodel.SourceFile),
			},
			verify: func(t *testing.T, config pkgconfigmodel.Config) {
				// Verify API config
				assert.Equal(t, "combined-api-key", config.GetString("api_key"))
				assert.Equal(t, "datadoghq.com", config.GetString("site"))

				// Verify log level
				logLevel := config.GetString("log_level")
				assert.NotEmpty(t, logLevel)
				assert.Contains(t, []string{"info", "INFO", "Level(6)"}, logLevel)

				// Verify logs defaults
				assert.True(t, config.GetBool("logs_config.use_v2_api"))

				// Verify custom config
				assert.Equal(t, "test-module", config.GetString("module.name"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configComponent := NewConfigComponent(tt.options...)
			require.NotNil(t, configComponent)

			// Get the underlying config for verification
			config := configComponent.(pkgconfigmodel.Config)
			tt.verify(t, config)
		})
	}
}

func TestConfigOptions_ModularUsage(t *testing.T) {
	// Example: A metrics-only module that only needs API config
	metricsConfig := NewConfigComponent(
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("metrics-api-key"),
				Site: "datadoghq.com",
			},
		}),
		WithCustomConfig("metrics.enabled", true, pkgconfigmodel.SourceFile),
	)

	config := metricsConfig.(pkgconfigmodel.Config)
	assert.Equal(t, "metrics-api-key", config.GetString("api_key"))
	assert.True(t, config.GetBool("metrics.enabled"))
	// Should not have logs defaults
	assert.False(t, config.IsConfigured("logs_config.use_v2_api"))

	// Example: A traces-only module with custom settings
	tracesConfig := NewConfigComponent(
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("traces-api-key"),
				Site: "datadoghq.eu",
			},
		}),
		WithCustomConfig("traces.enabled", true, pkgconfigmodel.SourceFile),
		WithCustomConfig("traces.sample_rate", 0.1, pkgconfigmodel.SourceDefault),
	)

	tracesConfigModel := tracesConfig.(pkgconfigmodel.Config)
	assert.Equal(t, "traces-api-key", tracesConfigModel.GetString("api_key"))
	assert.Equal(t, "datadoghq.eu", tracesConfigModel.GetString("site"))
	assert.True(t, tracesConfigModel.GetBool("traces.enabled"))
	assert.Equal(t, 0.1, tracesConfigModel.GetFloat64("traces.sample_rate"))
}

func TestConfigOptions_OrderMatters(t *testing.T) {
	// Test that later options can override earlier ones
	config := NewConfigComponent(
		WithCustomConfig("test.value", "first", pkgconfigmodel.SourceFile),
		WithCustomConfig("test.value", "second", pkgconfigmodel.SourceFile), // This should win
	)

	configModel := config.(pkgconfigmodel.Config)
	assert.Equal(t, "second", configModel.GetString("test.value"))
}

func TestAgentComponents_NewSerializer(t *testing.T) {
	// Create a zap logger for testing
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := config.Build()
	if err != nil {
		t.Fatalf("Failed to build logger: %v", err)
	}

	// Create a TelemetrySettings with the test logger
	telemetrySettings := component.TelemetrySettings{
		Logger: logger,
	}
	zlog := &ZapLogger{
		Logger: logger,
	}

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("abcdef1234567890"),
				Site: "test-site",
			},
		}),
		WithLogsEnabled(),
		WithLogLevel(telemetrySettings),
		WithForwarderConfig(),
		WithPayloadsConfig(),
	}
	// Create a config component
	configComponent := NewConfigComponent(configOptions...)

	// Create a log component
	logComponent := NewLogComponent(telemetrySettings)

	// Create a forwarder
	forwarder := NewForwarderComponent(configComponent, logComponent)

	// Create a compressor
	compressor := NewCompressorComponent()

	// Call NewSerializer
	serial := NewSerializerComponent(forwarder, compressor, configComponent, zlog, "test-hostname")

	// Assert that the returned serializer is not nil
	assert.NotNil(t, serial)

	// Assert that the serializer has the correct configuration
	assert.Equal(t, forwarder, serial.Forwarder)
	assert.Equal(t, compressor, serial.Strategy)
}

func TestAgentComponents_NewCompressor(t *testing.T) {
	// Call NewCompressor
	compressor := NewCompressorComponent()

	// Assert that the returned compressor is not nil
	assert.NotNil(t, compressor)

	// Assert that the returned compressor is of type *compression.GzipCompressor
	_, ok := compressor.(*implgzip.GzipStrategy)
	assert.True(t, ok, "Expected compressor to be of type *compression.GzipCompressor")
}

func TestAgentComponents_NewForwarder(t *testing.T) {
	// Create a zap logger for testing
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := config.Build()
	if err != nil {
		t.Fatalf("Failed to build logger: %v", err)
	}

	// Create a TelemetrySettings with the test logger
	telemetrySettings := component.TelemetrySettings{
		Logger: logger,
	}

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("abcdef1234567890"),
				Site: "test-site",
			},
		}),
		WithLogsEnabled(),
		WithLogLevel(telemetrySettings),
		WithForwarderConfig(),
		WithPayloadsConfig(),
	}

	// Create a config component
	configComponent := NewConfigComponent(configOptions...)

	// Create a log component
	logComponent := NewLogComponent(telemetrySettings)

	// Call NewForwarder
	forwarder := NewForwarderComponent(configComponent, logComponent)

	// Assert that the returned forwarder is not nil
	assert.NotNil(t, forwarder)

	// Assert that the returned forwarder is of type *defaultforwarder.DefaultForwarder
	_, ok := forwarder.(*defaultforwarder.DefaultForwarder)
	assert.True(t, ok, "Expected forwarder to be of type *defaultforwarder.DefaultForwarder")
}

func TestAgentComponents_NewLogComponent(t *testing.T) {
	// Create a zap logger for testing
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	logger, err := config.Build()
	if err != nil {
		t.Fatalf("Failed to build logger: %v", err)
	}

	// Create a TelemetrySettings with the test logger
	telemetrySettings := component.TelemetrySettings{
		Logger: logger,
	}

	// Call NewLogComponent
	logComponent := NewLogComponent(telemetrySettings)

	// Assert that the returned component is not nil
	assert.NotNil(t, logComponent)

	// Assert that the returned component is of type *datadog.Zaplogger
	zlog, ok := logComponent.(*ZapLogger)
	assert.True(t, ok, "Expected logComponent to be of type *datadog.Zaplogger")

	// Assert that the logger is correctly set
	assert.Equal(t, logger, zlog.Logger)
}

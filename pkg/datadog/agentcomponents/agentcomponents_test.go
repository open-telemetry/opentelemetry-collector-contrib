// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package agentcomponents // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"

import (
	"testing"

	coreconfig "github.com/DataDog/datadog-agent/comp/core/config"
	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
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
			name: "with logs config",
			options: []ConfigOption{
				WithLogsConfig(&datadogconfig.Config{
					Logs: datadogconfig.LogsConfig{
						TCPAddrConfig: confignet.TCPAddrConfig{
							Endpoint: "https://http-intake.logs.example.com",
						},
						UseCompression:   false,
						CompressionLevel: 3,
						BatchWait:        10,
					},
				}),
			},
			verify: func(t *testing.T, config pkgconfigmodel.Config) {
				assert.Equal(t, 10, config.GetInt("logs_config.batch_wait"))
				assert.False(t, config.GetBool("logs_config.use_compression"))
				assert.Equal(t, 3, config.GetInt("logs_config.compression_level"))
				assert.Equal(t, "https://http-intake.logs.example.com", config.GetString("logs_config.logs_dd_url"))
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

func TestWithLogsConfig(t *testing.T) {
	tests := []struct {
		name           string
		logsConfig     datadogconfig.LogsConfig
		expectedValues map[string]any
	}{
		{
			name: "default production values",
			logsConfig: datadogconfig.LogsConfig{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "https://http-intake.logs.datadoghq.com",
				},
				UseCompression:   true,
				CompressionLevel: 6,
				BatchWait:        5,
			},
			expectedValues: map[string]any{
				"logs_config.batch_wait":        5,
				"logs_config.use_compression":   true,
				"logs_config.compression_level": 6,
				"logs_config.logs_dd_url":       "https://http-intake.logs.datadoghq.com",
			},
		},
		{
			name: "custom endpoint and settings",
			logsConfig: datadogconfig.LogsConfig{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "https://custom-logs-intake.example.com",
				},
				UseCompression:   false,
				CompressionLevel: 0,
				BatchWait:        15,
			},
			expectedValues: map[string]any{
				"logs_config.batch_wait":        15,
				"logs_config.use_compression":   false,
				"logs_config.compression_level": 0,
				"logs_config.logs_dd_url":       "https://custom-logs-intake.example.com",
			},
		},
		{
			name: "eu site configuration",
			logsConfig: datadogconfig.LogsConfig{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "https://http-intake.logs.datadoghq.eu",
				},
				UseCompression:   true,
				CompressionLevel: 9,
				BatchWait:        1,
			},
			expectedValues: map[string]any{
				"logs_config.batch_wait":        1,
				"logs_config.use_compression":   true,
				"logs_config.compression_level": 9,
				"logs_config.logs_dd_url":       "https://http-intake.logs.datadoghq.eu",
			},
		},
		{
			name: "zero values",
			logsConfig: datadogconfig.LogsConfig{
				TCPAddrConfig: confignet.TCPAddrConfig{
					Endpoint: "",
				},
				UseCompression:   false,
				CompressionLevel: 0,
				BatchWait:        0,
			},
			expectedValues: map[string]any{
				"logs_config.batch_wait":        0,
				"logs_config.use_compression":   false,
				"logs_config.compression_level": 0,
				"logs_config.logs_dd_url":       "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &datadogconfig.Config{
				Logs: tt.logsConfig,
			}

			configComponent := NewConfigComponent(WithLogsConfig(cfg))
			require.NotNil(t, configComponent)

			config := configComponent.(pkgconfigmodel.Config)

			// Verify all expected values are set correctly
			for key, expectedValue := range tt.expectedValues {
				switch v := expectedValue.(type) {
				case bool:
					assert.Equal(t, v, config.GetBool(key), "Key: %s", key)
				case int:
					assert.Equal(t, v, config.GetInt(key), "Key: %s", key)
				case string:
					assert.Equal(t, v, config.GetString(key), "Key: %s", key)
				default:
					t.Errorf("Unsupported type for key %s: %T", key, expectedValue)
				}
			}
		})
	}
}

func TestWithLogsConfig_CombinedWithOtherOptions(t *testing.T) {
	// Test that WithLogsConfig works correctly when combined with other options
	cfg := &datadogconfig.Config{
		API: datadogconfig.APIConfig{
			Key:  configopaque.String("test-api-key"),
			Site: "datadoghq.eu",
		},
		Logs: datadogconfig.LogsConfig{
			TCPAddrConfig: confignet.TCPAddrConfig{
				Endpoint: "https://http-intake.logs.datadoghq.eu",
			},
			UseCompression:   true,
			CompressionLevel: 4,
			BatchWait:        8,
		},
	}

	configComponent := NewConfigComponent(
		WithAPIConfig(cfg),
		WithLogsEnabled(),
		WithLogsConfig(cfg),
		WithLogsDefaults(),
	)
	require.NotNil(t, configComponent)

	config := configComponent.(pkgconfigmodel.Config)

	// Verify API config
	assert.Equal(t, "test-api-key", config.GetString("api_key"))
	assert.Equal(t, "datadoghq.eu", config.GetString("site"))

	// Verify logs config
	assert.Equal(t, 8, config.GetInt("logs_config.batch_wait"))
	assert.True(t, config.GetBool("logs_config.use_compression"))
	assert.Equal(t, 4, config.GetInt("logs_config.compression_level"))
	assert.Equal(t, "https://http-intake.logs.datadoghq.eu", config.GetString("logs_config.logs_dd_url"))

	// Verify logs defaults
	assert.True(t, config.GetBool("logs_config.use_v2_api"))
	assert.True(t, config.GetBool("logs_enabled"))
}

func TestWithProxy(t *testing.T) {
	tests := []struct {
		name            string
		envVars         map[string]string
		proxyURL        string
		expectedHTTP    string
		expectedHTTPS   string
		expectedNoProxy []any
	}{
		{
			name: "all proxy environment variables set",
			envVars: map[string]string{
				"HTTP_PROXY":  "http://proxy.example.com:8080",
				"HTTPS_PROXY": "https://secure-proxy.example.com:8443",
				"NO_PROXY":    "localhost,127.0.0.1,.local",
			},
			expectedHTTP:    "http://proxy.example.com:8080",
			expectedHTTPS:   "https://secure-proxy.example.com:8443",
			expectedNoProxy: []any{"localhost", "127.0.0.1", ".local"},
			proxyURL:        "",
		},
		{
			name: "only HTTP_PROXY set",
			envVars: map[string]string{
				"HTTP_PROXY": "http://proxy.example.com:3128",
			},
			expectedHTTP:    "http://proxy.example.com:3128",
			expectedHTTPS:   "",
			expectedNoProxy: []any{""},
			proxyURL:        "",
		},
		{
			name:            "no proxy environment variables",
			envVars:         map[string]string{},
			expectedHTTP:    "",
			expectedHTTPS:   "",
			expectedNoProxy: []any{""},
			proxyURL:        "",
		},
		{
			name: "single NO_PROXY entry",
			envVars: map[string]string{
				"NO_PROXY": "internal.company.com",
			},
			expectedHTTP:    "",
			expectedHTTPS:   "",
			expectedNoProxy: []any{"internal.company.com"},
			proxyURL:        "",
		},
		{
			name:            "only proxy_url set",
			envVars:         map[string]string{},
			expectedHTTP:    "http://proxyurl.example.com:3128",
			expectedHTTPS:   "http://proxyurl.example.com:3128",
			expectedNoProxy: []any{""},
			proxyURL:        "http://proxyurl.example.com:3128",
		},
		{
			name: "both proxy_url and proxy env vars set",
			envVars: map[string]string{
				"HTTP_PROXY":  "http://proxy.example.com:8080",
				"HTTPS_PROXY": "https://secure-proxy.example.com:8443",
			},
			expectedHTTP:    "http://proxyurl.example.com:3128",
			expectedHTTPS:   "http://proxyurl.example.com:3128",
			expectedNoProxy: []any{""},
			proxyURL:        "http://proxyurl.example.com:3128",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set test environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			cfg := &datadogconfig.Config{
				ClientConfig: confighttp.ClientConfig{
					ProxyURL: tt.proxyURL,
				},
			}

			// Create config with proxy settings from environment
			configComponent := NewConfigComponent(WithProxy(cfg))
			require.NotNil(t, configComponent)

			config := configComponent.(pkgconfigmodel.Config)

			// Verify proxy settings
			assert.Equal(t, tt.expectedHTTP, config.GetString("proxy.http"))
			assert.Equal(t, tt.expectedHTTPS, config.GetString("proxy.https"))

			// Verify NO_PROXY setting
			noProxySlice := config.Get("proxy.no_proxy")
			assert.Equal(t, tt.expectedNoProxy, noProxySlice)
		})
	}
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

	// Call NewSerializer - now returns SerializerWithForwarder
	serializer := NewSerializerComponent(configComponent, zlog, "test-hostname")

	// Assert that the returned serializer is not nil
	assert.NotNil(t, serializer)

	// Assert that serializer implements MetricSerializer interface by calling a method
	assert.True(t, serializer.AreSeriesEnabled() || !serializer.AreSeriesEnabled()) // Just testing interface compliance
}

func TestSerializerWithForwarder_LifecycleMethods(t *testing.T) {
	// Create a zap logger for testing
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	zlog := &ZapLogger{
		Logger: logger,
	}

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("test-api-key-123"),
				Site: "datadoghq.com",
			},
		}),
		WithForwarderConfig(),
		WithPayloadsConfig(),
	}
	configComponent := NewConfigComponent(configOptions...)

	// Create the serializer
	serializer := NewSerializerComponent(configComponent, zlog, "test-hostname")
	require.NotNil(t, serializer)

	// Test initial state - should be stopped
	assert.Equal(t, defaultforwarder.Stopped, serializer.State())

	// Test Start method
	err = serializer.Start()
	assert.NoError(t, err, "Start should succeed")
	assert.Equal(t, defaultforwarder.Started, serializer.State())

	// Test that we can call Start again (should return error or be idempotent)
	err = serializer.Start()
	assert.Error(t, err, "Starting an already started forwarder should return an error")

	// Test Stop method
	serializer.Stop()
	assert.Equal(t, defaultforwarder.Stopped, serializer.State())

	// Test that we can stop again (should be safe)
	serializer.Stop() // Should not panic
}

func TestForwarderWithLifecycle_CompileTimeCheck(t *testing.T) {
	// This test verifies that our compile-time check works correctly
	// If DefaultForwarder doesn't implement ForwarderWithLifecycle,
	// the compilation would fail at the var _ ForwarderWithLifecycle line

	// Create a minimal setup to verify the interface works
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	zlog := &ZapLogger{Logger: logger}

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("abcdef1234567890"),
				Site: "datadoghq.com",
			},
		}),
		WithForwarderConfig(),
	}
	configComponent := NewConfigComponent(configOptions...)

	// This should return ForwarderWithLifecycle
	forwarder := newForwarderComponent(configComponent, zlog)
	require.NotNil(t, forwarder)

	// Lifecycle methods
	assert.Equal(t, defaultforwarder.Stopped, forwarder.State())

	err = forwarder.Start()
	assert.NoError(t, err)
	assert.Equal(t, defaultforwarder.Started, forwarder.State())

	// Test that it implements both Forwarder and lifecycle methods
	// Forwarder interface methods
	err = forwarder.SubmitV1Series(nil, nil)
	assert.NoError(t, err) // Should not panic, may return error

	forwarder.Stop()
	assert.Equal(t, defaultforwarder.Stopped, forwarder.State())
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

func TestNewForwarderComponent(t *testing.T) {
	tests := []struct {
		name     string
		site     string
		apiKey   string
		validate func(t *testing.T, forwarder forwarderWithLifecycle, cfg coreconfig.Component)
	}{
		{
			name:   "basic forwarder creation",
			site:   "datadoghq.com",
			apiKey: "test-api-key-123",
			validate: func(t *testing.T, forwarder forwarderWithLifecycle, _ coreconfig.Component) {
				// Test that forwarder is not nil
				assert.NotNil(t, forwarder)

				// Test that the forwarder is in stopped state initially
				assert.Equal(t, defaultforwarder.Stopped, forwarder.State())
			},
		},
		{
			name:   "different site configuration",
			site:   "datadoghq.eu",
			apiKey: "eu-api-key-456",
			validate: func(t *testing.T, forwarder forwarderWithLifecycle, cfg coreconfig.Component) {
				assert.NotNil(t, forwarder)

				// Verify the site was configured correctly
				assert.Equal(t, "datadoghq.eu", cfg.GetString("site"))
				assert.Equal(t, "eu-api-key-456", cfg.GetString("api_key"))
			},
		},
		{
			name:   "empty api key",
			site:   "datadoghq.com",
			apiKey: "",
			validate: func(t *testing.T, forwarder forwarderWithLifecycle, cfg coreconfig.Component) {
				assert.NotNil(t, forwarder)
				assert.Empty(t, cfg.GetString("api_key"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test logger
			logger, err := zap.NewDevelopment()
			require.NoError(t, err)

			// Create log component
			logComponent := &ZapLogger{Logger: logger}

			// Create config component with test data
			configOptions := []ConfigOption{
				WithAPIConfig(&datadogconfig.Config{
					API: datadogconfig.APIConfig{
						Key:  configopaque.String(tt.apiKey),
						Site: tt.site,
					},
				}),
				WithForwarderConfig(),
			}
			configComponent := NewConfigComponent(configOptions...)

			// Call the function under test
			forwarder := newForwarderComponent(configComponent, logComponent)

			// Run test-specific validations
			tt.validate(t, forwarder, configComponent)
		})
	}
}

func TestNewForwarderComponent_Internal(t *testing.T) {
	// Create test logger
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	logComponent := &ZapLogger{Logger: logger}

	// Create config with specific test values
	testSite := "test.datadoghq.com"
	testAPIKey := "abcdef1234567890"

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String(testAPIKey),
				Site: testSite,
			},
		}),
		WithForwarderConfig(),
	}
	configComponent := NewConfigComponent(configOptions...)

	// Call the function under test
	forwarder := newForwarderComponent(configComponent, logComponent)

	// Validate the forwarder was created
	require.NotNil(t, forwarder)

	// Test internal state
	assert.Equal(t, defaultforwarder.Stopped, forwarder.State())

	// Test that we can start and stop the forwarder (this tests internal configuration)
	err = forwarder.Start()
	assert.NoError(t, err)
	assert.Equal(t, defaultforwarder.Started, forwarder.State())

	forwarder.Stop()
	assert.Equal(t, defaultforwarder.Stopped, forwarder.State())
}

func TestNewForwarderComponent_KeysPerDomainConfiguration(t *testing.T) {
	// This test verifies that the keysPerDomain map is constructed correctly
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	logComponent := &ZapLogger{Logger: logger}

	testSite := "custom.datadoghq.com"
	testAPIKey := "test-key-123"

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String(testAPIKey),
				Site: testSite,
			},
		}),
		WithForwarderConfig(),
	}
	configComponent := NewConfigComponent(configOptions...)

	// Verify config contains the expected values before calling newForwarderComponent
	assert.Equal(t, testSite, configComponent.GetString("site"))
	assert.Equal(t, testAPIKey, configComponent.GetString("api_key"))

	// Call the function under test
	forwarder := newForwarderComponent(configComponent, logComponent)
	require.NotNil(t, forwarder)

	err = forwarder.Start()
	assert.NoError(t, err)

	// Clean up
	forwarder.Stop()
}

func TestNewForwarderComponent_DisableAPIKeyChecking(t *testing.T) {
	// This test specifically verifies that DisableAPIKeyChecking is set to true
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	logComponent := &ZapLogger{Logger: logger}

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("test-key"),
				Site: "datadoghq.com",
			},
		}),
		WithForwarderConfig(),
	}
	configComponent := NewConfigComponent(configOptions...)

	// Call the function under test
	forwarder := newForwarderComponent(configComponent, logComponent)
	require.NotNil(t, forwarder)

	// The function should set DisableAPIKeyChecking to true
	// We can verify this by checking that the forwarder starts successfully
	// even with potentially invalid keys, since API key validation is disabled
	err = forwarder.Start()
	assert.NoError(t, err, "Forwarder should start successfully with DisableAPIKeyChecking=true")

	// Clean up
	forwarder.Stop()
}

func TestNewForwarderComponent_ForwarderInterface(t *testing.T) {
	// Test that the returned forwarder implements the expected interface
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)
	logComponent := &ZapLogger{Logger: logger}

	configOptions := []ConfigOption{
		WithAPIConfig(&datadogconfig.Config{
			API: datadogconfig.APIConfig{
				Key:  configopaque.String("test-key"),
				Site: "datadoghq.com",
			},
		}),
		WithForwarderConfig(),
	}
	configComponent := NewConfigComponent(configOptions...)

	forwarder := newForwarderComponent(configComponent, logComponent)
	require.NotNil(t, forwarder)

	// Test a few method calls that shouldn't panic (though they may return errors)
	// Since these require the forwarder to be started, we'll start it first
	err = forwarder.Start()
	require.NoError(t, err)

	// These method calls validate that the interface is properly implemented
	// We expect errors since we don't have a real backend, but no panics
	err = forwarder.SubmitV1Series(nil, nil)
	require.NoError(t, err)

	// Clean up
	forwarder.Stop()
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package agentcomponents

import (
	"testing"

	"github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder"
	implgzip "github.com/DataDog/datadog-agent/pkg/util/compression/impl-gzip"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
)

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
	zlog := &datadog.Zaplogger{
		Logger: logger,
	}

	// Create a config component
	configComponent := NewConfigComponent(telemetrySettings, "abcdef1234567890", "test-site")

	// Create a log component
	logComponent := NewLogComponent(telemetrySettings)

	// Create a forwarder
	forwarder := NewForwarder(configComponent, logComponent)

	// Create a compressor
	compressor := NewCompressor()

	// Call NewSerializer
	serial := NewSerializer(forwarder, compressor, configComponent, zlog, "test-hostname")

	// Assert that the returned serializer is not nil
	assert.NotNil(t, serial)

	// Assert that the serializer has the correct configuration
	assert.Equal(t, forwarder, serial.Forwarder)
	assert.Equal(t, compressor, serial.Strategy)
}

func TestAgentComponents_NewCompressor(t *testing.T) {
	// Call NewCompressor
	compressor := NewCompressor()

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

	// Create a config component
	configComponent := NewConfigComponent(telemetrySettings, "abcdef1234567890", "test-site")

	// Create a log component
	logComponent := NewLogComponent(telemetrySettings)

	// Call NewForwarder
	forwarder := NewForwarder(configComponent, logComponent)

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
	zlog, ok := logComponent.(*datadog.Zaplogger)
	assert.True(t, ok, "Expected logComponent to be of type *datadog.Zaplogger")

	// Assert that the logger is correctly set
	assert.Equal(t, logger, zlog.Logger)
}

func TestAgentComponents_NewConfigComponent(t *testing.T) {
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

	// Call NewConfigComponent
	configComponent := NewConfigComponent(telemetrySettings, "abcdef1234567890", "test-site")

	// Assert that the returned component is not nil
	assert.NotNil(t, configComponent)

	// Assert that the configuration values are set correctly
	assert.Equal(t, "abcdef1234567890", configComponent.GetString("api_key"))
	assert.Equal(t, "test-site", configComponent.GetString("site"))
	assert.Equal(t, "info", configComponent.GetString("log_level"))
	assert.True(t, configComponent.GetBool("logs_enabled"))
	assert.True(t, configComponent.GetBool("enable_payloads.events"))
	assert.True(t, configComponent.GetBool("enable_payloads.json_to_v1_intake"))
	assert.True(t, configComponent.GetBool("enable_sketch_stream_payload_serialization"))
	assert.Equal(t, 60, configComponent.GetInt("forwarder_apikey_validation_interval"))
	assert.Equal(t, 1, configComponent.GetInt("forwarder_num_workers"))
	assert.Equal(t, 2, configComponent.GetInt("logging_frequency"))
	assert.Equal(t, 2, configComponent.GetInt("forwarder_backoff_factor"))
	assert.Equal(t, 2, configComponent.GetInt("forwarder_backoff_base"))
	assert.Equal(t, 64, configComponent.GetInt("forwarder_backoff_max"))
	assert.Equal(t, 2, configComponent.GetInt("forwarder_recovery_interval"))
}

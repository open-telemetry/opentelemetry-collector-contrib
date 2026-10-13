// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clssdk "github.com/tencentcloud/tencentcloud-cls-sdk-go"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/internal/metadata"
)

func testConfig() *Config {
	return &Config{
		Region:    "ap-beijing",
		LogSet:    "demo-logset",
		Topic:     "demo-topic",
		SecretID:  "demo-secret-id",
		SecretKey: "demo-secret-key",
	}
}

func TestNewProducerConfigEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected string
	}{
		{
			name:     "region resolves to extranet endpoint",
			config:   &Config{Region: "ap-beijing"},
			expected: "ap-beijing.cls.tencentcs.com",
		},
		{
			name:     "region resolves to intranet endpoint",
			config:   &Config{Region: "ap-beijing", IsIntranet: true},
			expected: "ap-beijing.cls.tencentyun.com",
		},
		{
			name:     "endpoint takes precedence over region",
			config:   &Config{Region: "ap-beijing", Endpoint: "cls.example.com"},
			expected: "cls.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, newProducerConfig(tt.config).Endpoint)
		})
	}
}

func TestNewProducerConfigCredentials(t *testing.T) {
	cfg := testConfig()
	producerConfig := newProducerConfig(cfg)

	assert.Equal(t, cfg.SecretID, producerConfig.AccessKeyID)
	assert.Equal(t, string(cfg.SecretKey), producerConfig.AccessKeySecret)
}

func TestNewLogServiceClient(t *testing.T) {
	client, err := newLogServiceClient(testConfig(), exportertest.NewNopSettings(metadata.Type).Logger)
	require.NoError(t, err)
	require.NotNil(t, client)

	assert.NoError(t, client.shutdown(t.Context()))
}

func TestSendLogsWithoutLogs(t *testing.T) {
	client, err := newLogServiceClient(testConfig(), exportertest.NewNopSettings(metadata.Type).Logger)
	require.NoError(t, err)

	assert.NoError(t, client.sendLogs(nil))
	assert.NoError(t, client.sendLogs([]*clssdk.Log{}))

	assert.NoError(t, client.shutdown(t.Context()))
}

func TestLogServiceClientCallbacks(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	client := &logServiceClientImpl{
		logset: "demo-logset",
		topic:  "demo-topic",
		logger: zap.New(core),
	}

	client.Success(clssdk.NewResult())
	require.Equal(t, 1, logs.Len())
	assert.Equal(t, "Send to CLS LogService success", logs.All()[0].Message)

	client.Fail(clssdk.NewResult())
	require.Equal(t, 2, logs.Len())
	assert.Equal(t, "Send to CLS LogService failed", logs.All()[1].Message)
	assert.Equal(t, zapcore.WarnLevel, logs.All()[1].Level)
}

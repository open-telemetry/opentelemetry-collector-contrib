// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestNewLogsExporter(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		shouldError bool
	}{
		{
			name: "Valid domain config",
			cfg: &Config{
				Domain:     "test.domain.com",
				PrivateKey: "test-key",
			},
			shouldError: false,
		},
		{
			name: "Valid logs endpoint config",
			cfg: &Config{
				Logs: configgrpc.ClientConfig{
					Endpoint: "localhost:4317",
				},
				PrivateKey: "test-key",
			},
			shouldError: false,
		},
		{
			name: "Missing both domain and endpoint",
			cfg: &Config{
				PrivateKey: "test-key",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := newLogsExporter(tt.cfg, exportertest.NewNopSettings(exportertest.NopType))
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, exp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exp)
			}
		})
	}
}

func TestLogsExporter_Start(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Logs: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.logExporter)
	assert.Contains(t, exp.config.Logs.Headers, "Authorization")

	// Test shutdown
	err = exp.shutdown(context.Background())
	require.NoError(t, err)
}

func TestLogsExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Logs: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{
				"test-header": "test-value",
			},
		},
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := context.Background()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestLogsExporter_PushLogs(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Logs: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newLogsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	// Initialize the exporter by calling start
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Create test logs
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs()
	rl := resourceLogs.AppendEmpty()

	resource := rl.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushLogs(context.Background(), logs)
	assert.Error(t, err)
}

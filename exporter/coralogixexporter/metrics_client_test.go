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
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNewMetricsExporter(t *testing.T) {
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
			name: "Valid metrics endpoint config",
			cfg: &Config{
				Metrics: configgrpc.ClientConfig{
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
			exp, err := newMetricsExporter(tt.cfg, exportertest.NewNopSettings(exportertest.NopType))
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

func TestMetricsExporter_Start(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Metrics: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	assert.NotNil(t, exp.clientConn)
	assert.NotNil(t, exp.metricExporter)
	assert.Contains(t, exp.config.Metrics.Headers, "Authorization")

	// Test shutdown
	err = exp.shutdown(context.Background())
	require.NoError(t, err)
}

func TestMetricsExporter_EnhanceContext(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Metrics: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{
				"test-header": "test-value",
			},
		},
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	ctx := context.Background()
	enhancedCtx := exp.enhanceContext(ctx)
	assert.NotEqual(t, ctx, enhancedCtx)
}

func TestMetricsExporter_PushMetrics(t *testing.T) {
	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
		Metrics: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
		},
	}

	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	// Initialize the exporter by calling start
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Create test metrics
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics()
	rm := resourceMetrics.AppendEmpty()

	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	err = exp.pushMetrics(context.Background(), metrics)
	assert.Error(t, err)
}

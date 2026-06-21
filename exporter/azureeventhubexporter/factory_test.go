// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubexporter/internal/metadata"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Connection = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key;SharedAccessKey=dGVzdA==;EntityPath=hub"

	f := NewFactory()
	exp, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Connection = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key;SharedAccessKey=dGVzdA==;EntityPath=hub"

	f := NewFactory()
	exp, err := f.CreateMetrics(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Connection = "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=key;SharedAccessKey=dGVzdA==;EntityPath=hub"

	f := NewFactory()
	exp, err := f.CreateTraces(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	assert.NotNil(t, exp)
}

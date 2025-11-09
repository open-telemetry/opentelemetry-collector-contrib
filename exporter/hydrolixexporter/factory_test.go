// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "hydrolix", factory.Type().String())
}

func TestCreateTracesExporter(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "https://example.com/ingest",
			Timeout:  30 * time.Second,
		},
		HDXTable:     "traces_table",
		HDXTransform: "traces_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	set := exportertest.NewNopSettings(factory.Type())
	exporter, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)
}

func TestCreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "https://example.com/ingest",
			Timeout:  30 * time.Second,
		},
		HDXTable:     "metrics_table",
		HDXTransform: "metrics_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	set := exportertest.NewNopSettings(factory.Type())
	exporter, err := factory.CreateMetrics(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "https://example.com/ingest",
			Timeout:  30 * time.Second,
		},
		HDXTable:     "logs_table",
		HDXTransform: "logs_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	set := exportertest.NewNopSettings(factory.Type())
	exporter, err := factory.CreateLogs(context.Background(), set, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)
}

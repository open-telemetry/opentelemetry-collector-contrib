// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func Test_createDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, cfg, &Config{
		TimeoutSettings:         exporterhelper.NewDefaultTimeoutSettings(),
		RetrySettings:           exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:           exporterhelper.NewDefaultQueueSettings(),
		Trace:                   ExporterOption{Encoding: defaultEncoding},
		Log:                     ExporterOption{Encoding: defaultEncoding},
		Metric:                  ExporterOption{Encoding: defaultEncoding},
		Endpoint:                defaultBroker,
		Authentication:          Authentication{},
		MaxConnectionsPerBroker: 1,
		ConnectionTimeout:       5 * time.Second,
		OperationTimeout:        30 * time.Second,
	})
}

func TestWithTracesMarshalers_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""
	cfg.Trace.Encoding = "custom"
	f := NewFactory()
	r, err := f.CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	// no available broker
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateTracesExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	r, err := createTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	// no available broker
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	mr, err := createMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	require.Error(t, err)
	assert.Nil(t, mr)
}

func TestCreateLogsExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	mr, err := createLogsExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	require.Error(t, err)
	assert.Nil(t, mr)
}

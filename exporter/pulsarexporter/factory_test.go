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
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		Endpoint:        defaultBroker,
		// using an empty topic to track when it has not been set by user, default is based on traces or metrics.
		Topic:                   "",
		Encoding:                defaultEncoding,
		Authentication:          Authentication{},
		MaxConnectionsPerBroker: 1,
		ConnectionTimeout:       5 * time.Second,
		OperationTimeout:        30 * time.Second,
	})
}

func TestCreateTracesExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	f := pulsarExporterFactory{tracesMarshalers: tracesMarshalers()}
	r, err := f.createTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	// no available broker
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	mf := pulsarExporterFactory{metricsMarshalers: metricsMarshalers()}
	mr, err := mf.createMetricsExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	require.Error(t, err)
	assert.Nil(t, mr)
}

func TestCreateLogsExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	mf := pulsarExporterFactory{logsMarshalers: logsMarshalers()}
	mr, err := mf.createLogsExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	require.Error(t, err)
	assert.Nil(t, mr)
}

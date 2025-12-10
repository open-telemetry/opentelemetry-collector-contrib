// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter/internal/metadata"
)

func Test_createDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		Endpoint:        defaultBroker,

		Topic:                   "",
		Encoding:                defaultEncoding,
		Authentication:          Authentication{},
		MaxConnectionsPerBroker: 1,
		ConnectionTimeout:       5 * time.Second,
		OperationTimeout:        30 * time.Second,
	}, cfg)
}

func TestWithTracesMarshalers_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	tracesMarshaler := &customTraceMarshaler{encoding: "unknown"}
	f := NewFactory(withTracesMarshalers(tracesMarshaler))
	r, err := f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	// no available broker
	require.Error(t, err)
}

func TestCreateTraces_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	f := pulsarExporterFactory{tracesMarshalers: tracesMarshalers()}
	r, err := f.createTracesExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	// no available broker
	require.Error(t, err)
}

func TestCreateMetrics_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	mf := pulsarExporterFactory{metricsMarshalers: metricsMarshalers()}
	r, err := mf.createMetricsExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
}

func TestCreateLogs_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = ""

	mf := pulsarExporterFactory{logsMarshalers: logsMarshalers()}
	r, err := mf.createLogsExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
}

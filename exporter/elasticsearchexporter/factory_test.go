// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"http://test:9200"}
	})
	params := exportertest.NewNopSettings()
	exporter, err := factory.CreateLogs(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.Background()))
}

func TestFactory_CreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"http://test:9200"}
	})
	params := exportertest.NewNopSettings()
	exporter, err := factory.CreateMetrics(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.Background()))
}

func TestFactory_CreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoints = []string{"http://test:9200"}
	})
	params := exportertest.NewNopSettings()
	exporter, err := factory.CreateTraces(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.Background()))
}

func TestFactory_DedotDeprecated(t *testing.T) {
	loggerCore, logObserver := observer.New(zap.WarnLevel)
	set := exportertest.NewNopSettings()
	set.Logger = zap.New(loggerCore)

	cfgNoDedotECS := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoint = "http://testing.invalid:9200"
		cfg.Mapping.Dedot = false
		cfg.Mapping.Mode = "ecs"
	})

	cfgDedotRaw := withDefaultConfig(func(cfg *Config) {
		cfg.Endpoint = "http://testing.invalid:9200"
		cfg.Mapping.Dedot = true
		cfg.Mapping.Mode = "raw"
	})

	for _, cfg := range []*Config{cfgNoDedotECS, cfgDedotRaw} {
		factory := NewFactory()
		logsExporter, err := factory.CreateLogs(context.Background(), set, cfg)
		require.NoError(t, err)
		require.NoError(t, logsExporter.Shutdown(context.Background()))

		tracesExporter, err := factory.CreateTraces(context.Background(), set, cfg)
		require.NoError(t, err)
		require.NoError(t, tracesExporter.Shutdown(context.Background()))

		metricsExporter, err := factory.CreateMetrics(context.Background(), set, cfg)
		require.NoError(t, err)
		require.NoError(t, metricsExporter.Shutdown(context.Background()))
	}

	records := logObserver.AllUntimed()
	assert.Len(t, records, 6)
	for _, record := range records {
		assert.Equal(t, "dedot has been deprecated: in the future, dedotting will always be performed in ECS mode only", record.Message)
	}
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateLogs(t *testing.T) {
	for _, tc := range signalTestCases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			params := exportertest.NewNopSettings(metadata.Type)

			observedZapCore, observedLogs := observer.New(zap.InfoLevel)
			params.Logger = zap.New(observedZapCore)

			cfg := createDefaultConfig()
			cm := confmap.NewFromStringMap(tc.cfg)
			require.NoError(t, cm.Unmarshal(cfg))

			exporter, err := factory.CreateLogs(context.Background(), params, cfg.(*Config))
			require.NoError(t, err)
			require.NotNil(t, exporter)
			require.Equal(t, len(tc.expectedLogs), observedLogs.Len())
			actualLogs := observedLogs.All()
			for i, expectedLog := range tc.expectedLogs {
				assert.Contains(t, actualLogs[i].Message, expectedLog)
			}

			require.NoError(t, exporter.Shutdown(context.Background()))
		})
	}
}

func TestFactory_CreateMetrics(t *testing.T) {
	for _, tc := range signalTestCases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			params := exportertest.NewNopSettings(metadata.Type)

			observedZapCore, observedLogs := observer.New(zap.InfoLevel)
			params.Logger = zap.New(observedZapCore)

			cfg := createDefaultConfig()
			cm := confmap.NewFromStringMap(tc.cfg)
			require.NoError(t, cm.Unmarshal(cfg))

			exporter, err := factory.CreateMetrics(context.Background(), params, cfg.(*Config))
			require.NoError(t, err)
			require.NotNil(t, exporter)
			require.Equal(t, len(tc.expectedLogs), observedLogs.Len())
			actualLogs := observedLogs.All()
			for i, expectedLog := range tc.expectedLogs {
				assert.Contains(t, actualLogs[i].Message, expectedLog)
			}

			require.NoError(t, exporter.Shutdown(context.Background()))
		})
	}
}

func TestFactory_CreateTraces(t *testing.T) {
	for _, tc := range signalTestCases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			params := exportertest.NewNopSettings(metadata.Type)

			observedZapCore, observedLogs := observer.New(zap.InfoLevel)
			params.Logger = zap.New(observedZapCore)

			cfg := createDefaultConfig()
			cm := confmap.NewFromStringMap(tc.cfg)
			require.NoError(t, cm.Unmarshal(cfg))

			exporter, err := factory.CreateTraces(context.Background(), params, cfg.(*Config))
			require.NoError(t, err)
			require.NotNil(t, exporter)
			require.Equal(t, len(tc.expectedLogs), observedLogs.Len())
			actualLogs := observedLogs.All()
			for i, expectedLog := range tc.expectedLogs {
				assert.Contains(t, actualLogs[i].Message, expectedLog)
			}

			require.NoError(t, exporter.Shutdown(context.Background()))
		})
	}
}

var signalTestCases = []struct {
	name         string
	cfg          map[string]any
	expectedLogs []string
}{
	{
		name: "default",
		cfg: map[string]any{
			"endpoints": []string{"http://test:9200"},
		},
	},
	{
		name: "with_deprecated_batcher",
		cfg: map[string]any{
			"batcher": map[string]any{
				"enabled": true,
			},
		},
		expectedLogs: []string{
			"batcher has been deprecated",
		},
	},
	{
		name: "with_sending_queue",
		cfg: map[string]any{
			"sending_queue": map[string]any{
				"enabled": true,
				"batch":   map[string]any{},
			},
		},
	},
	{
		name: "with_sending_queue_disabled_and_deprecated_batcher",
		cfg: map[string]any{
			"sending_queue": map[string]any{
				"batch": map[string]any{},
			},
			"batcher": map[string]any{
				"enabled": true,
			},
		},
		expectedLogs: []string{
			"sending_queue::batch will take preference",
		},
	},
	{
		name: "with_sending_queue_enabled_and_deprecated_batcher",
		cfg: map[string]any{
			"sending_queue": map[string]any{
				"enabled": true,
				"batch":   map[string]any{},
			},
			"batcher": map[string]any{
				"enabled": true,
			},
		},
		expectedLogs: []string{
			"sending_queue::batch will take preference",
		},
	},
}

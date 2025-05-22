// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudexporter

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/internal/resourcemapping"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExporter(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Default credentials not set, skip creating Google Cloud exporter")
	}
	ctx := context.Background()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.ProjectID = "test"

	te, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}

func TestCreateLegacyExporter(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Default credentials not set, skip creating Google Cloud exporter")
	}
	ctx := context.Background()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.ProjectID = "test"

	te, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}

func TestCustomMonitoredResourceMapping(t *testing.T) {
	_ = featuregate.GlobalRegistry().Set("exporter.googlecloud.CustomMonitoredResources", true)
	ctx := context.Background()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.ProjectID = "test"

	te, err := factory.CreateLogs(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create logs exporter")

	actualLogFuncPointer := reflect.ValueOf(eCfg.LogConfig.MapMonitoredResource).Pointer()
	expectedLogFuncPointer := reflect.ValueOf(resourcemapping.CustomLoggingMonitoredResourceMapping).Pointer()
	assert.Equal(t, expectedLogFuncPointer, actualLogFuncPointer)

	me, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")

	actualMetricsFuncPointer := reflect.ValueOf(eCfg.LogConfig.MapMonitoredResource).Pointer()
	expectedMetricsFuncPointer := reflect.ValueOf(resourcemapping.CustomLoggingMonitoredResourceMapping).Pointer()
	assert.Equal(t, expectedMetricsFuncPointer, actualMetricsFuncPointer)
}

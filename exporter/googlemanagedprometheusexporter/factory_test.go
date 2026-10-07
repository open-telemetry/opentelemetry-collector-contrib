// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"google.golang.org/api/option"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter/internal/metadata"
)

func TestFactoryTypeAlias(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, component.MustNewType("google_managed_prometheus"), factory.Type())

	for _, typ := range []component.Type{metadata.Type, component.MustNewType("googlemanagedprometheus")} {
		t.Run(typ.String(), func(t *testing.T) {
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.GMPConfig.ProjectID = "test-project"
			cfg.GMPConfig.MetricConfig.ClientConfig.Endpoint = "localhost:4317"
			cfg.GMPConfig.MetricConfig.ClientConfig.GetClientOptions = func() []option.ClientOption {
				return []option.ClientOption{option.WithoutAuthentication()}
			}
			exp, err := factory.CreateMetrics(t.Context(), exportertest.NewNopSettings(typ), cfg)
			require.NoError(t, err)
			require.NotNil(t, exp)
			require.NoError(t, exp.Shutdown(t.Context()))
		})
	}
}

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
	ctx := t.Context()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.GMPConfig.ProjectID = "test"

	te, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}

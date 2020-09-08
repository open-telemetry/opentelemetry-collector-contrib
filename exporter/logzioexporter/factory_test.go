package logzioexporter

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateTraceExporter(t *testing.T) {

	factories, err := componenttest.ExampleComponents()
	require.NoError(t, err)
	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exporter, err := factory.CreateTraceExporter(context.Background(), params, cfg.Exporters["logzio/2"])
	assert.Nil(t, err)
	assert.NotNil(t, exporter)
}

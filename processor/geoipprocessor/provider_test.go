package geoipprocessor

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
)

type ProviderMock struct {
	LocationF func(context.Context, net.IP) (attribute.Set, error)
}

type ProviderFactoryMock struct {
	CreateDefaultConfigF func() provider.Config
	CreateGeoIPProviderF func(ctx context.Context, settings processor.CreateSettings, cfg provider.Config) (provider.GeoIPProvider, error)
}

var (
	_ provider.GeoIPProvider        = (*ProviderMock)(nil)
	_ provider.GeoIPProviderFactory = (*ProviderFactoryMock)(nil)
)

func (pm *ProviderMock) Location(ctx context.Context, ip net.IP) (attribute.Set, error) {
	return pm.LocationF(ctx, ip)
}

func (fm *ProviderFactoryMock) CreateDefaultConfig() provider.Config {
	return fm.CreateDefaultConfigF()
}

func (fm *ProviderFactoryMock) CreateGeoIPProvider(ctx context.Context, settings processor.CreateSettings, cfg provider.Config) (provider.GeoIPProvider, error) {
	return fm.CreateGeoIPProviderF(ctx, settings, cfg)
}

func TestLoadConfig_MockProvider(t *testing.T) {
	mockProvider := ProviderMock{
		LocationF: func(context.Context, net.IP) (attribute.Set, error) {
			return attribute.Set{}, nil
		},
	}
	mockFactory := ProviderFactoryMock{
		CreateDefaultConfigF: func() provider.Config {
			type SampleConfig struct {
				Database string `mapstructure:"database"`
			}
			return &SampleConfig{}
		},
		CreateGeoIPProviderF: func(ctx context.Context, settings processor.CreateSettings, cfg provider.Config) (provider.GeoIPProvider, error) {
			return &mockProvider, nil
		},
	}

	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	providerFactories["mock"] = &mockFactory
	factory := NewFactory()
	factories.Processors[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-mockProvider.yaml"), factories)
	assert.NoError(t, err)
}

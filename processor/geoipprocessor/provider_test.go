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
	"go.opentelemetry.io/collector/processor/processortest"
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

var baseMockProvider = ProviderMock{
	LocationF: func(context.Context, net.IP) (attribute.Set, error) {
		return attribute.Set{}, nil
	},
}

var baseMockFactory = ProviderFactoryMock{
	CreateDefaultConfigF: func() provider.Config {
		type emptyConfig struct{}
		return &emptyConfig{}
	},
	CreateGeoIPProviderF: func(ctx context.Context, settings processor.CreateSettings, cfg provider.Config) (provider.GeoIPProvider, error) {
		return &baseMockProvider, nil
	},
}

func TestLoadConfig_MockProvider(t *testing.T) {
	baseMockFactory.CreateDefaultConfigF = func() provider.Config {
		type SampleConfig struct {
			Database string `mapstructure:"database"`
		}
		return &SampleConfig{}
	}

	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	providerFactories["mock"] = &baseMockFactory
	factory := NewFactory()
	factories.Processors[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-mockProvider.yaml"), factories)
	assert.NoError(t, err)
}

func TestGeoProviderLocation(t *testing.T) {
	exampleIP := net.IPv4(240, 0, 0, 0)
	baseMockProvider.LocationF = func(ctx context.Context, ip net.IP) (attribute.Set, error) {
		// dummy provider that only returns data if the IP is 240.0.0.0
		if ip.Equal(exampleIP) {
			return attribute.NewSet(
				attribute.String("geo.city_name", "Barcelona"),
				attribute.String("geo.country_name", "Spain"),
			), nil
		}
		return attribute.NewSet(), nil
	}
	factory := NewFactory()
	config := factory.CreateDefaultConfig()
	geoCfg := config.(*Config)
	geoCfg.Providers = make(map[string]provider.Config, 1)
	geoCfg.Providers["mock"] = &baseMockFactory

	providers, err := createGeoIPProviders(context.Background(), processortest.NewNopCreateSettings(), geoCfg, providerFactories)
	if err != nil {
		t.Fatal(err)
	}

	processor := newGeoIPProcessor(providers)
	assert.Equal(t, 1, len(processor.providers))

	attributes, err := processor.providers[0].Location(context.Background(), exampleIP)
	assert.NoError(t, err)
	value, has := attributes.Value("geo.city_name")
	require.True(t, has)
	require.Equal(t, "Barcelona", value.AsString())
}

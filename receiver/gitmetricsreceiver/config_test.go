package gitmetricsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitmetricsreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[component.NewID(metadata.Type)]
	defaultConfigGitHubScraper := factory.CreateDefaultConfig()
	defaultConfigGitHubScraper.(*Config).Scrapers = map[string]internal.Config{
		githubscraper.TypeStr: (&githubscraper.Factory{}).CreateDefaultConfig(),
	}

	assert.Equal(t, defaultConfigGitHubScraper, r0)

	r1 := cfg.Receivers[component.NewIDWithName(metadata.Type, "customname")].(*Config)
	expectedConfig := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 30 * time.Second,
		},
		Scrapers: map[string]internal.Config{
			githubscraper.TypeStr:  (&githubscraper.Factory{}).CreateDefaultConfig(),
		},
	}

	assert.Equal(t, expectedConfig, r1)
}

func TestLoadInvalidConfig_NoScrapers(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-noscrapers.yaml"), factories)

	require.Contains(t, err.Error(), "must specify at least one scraper when using hostmetrics receiver")
}

func TestLoadInvalidConfig_InvalidScraperKey(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalidscraperkey.yaml"), factories)

	require.Contains(t, err.Error(), "error reading configuration for \"hostmetrics\": invalid scraper key: invalidscraperkey")
}

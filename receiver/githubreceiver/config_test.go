// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	// nolint:staticcheck
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Len(t, cfg.Receivers, 2)

	r0 := cfg.Receivers[component.NewID(metadata.Type)]
	defaultConfigGitHubScraper := factory.CreateDefaultConfig()
	defaultConfigGitHubScraper.(*Config).Scrapers = map[string]internal.Config{
		metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
	}
	defaultConfigGitHubScraper.(*Config).AccessToken = "my_token"
    defaultConfigGitHubScraper.(*Config).LogType = "user"
    defaultConfigGitHubScraper.(*Config).Name = "github"
    defaultConfigGitHubScraper.(*Config).PollInterval = 60 * time.Second

	assert.Equal(t, defaultConfigGitHubScraper, r0)

	r1 := cfg.Receivers[component.NewIDWithName(metadata.Type, "customname")].(*Config)
	expectedConfig := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 30 * time.Second,
			InitialDelay:       1 * time.Second,
		},
		Scrapers: map[string]internal.Config{
			metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
		},
		AccessToken:  "my_token",
		LogType:      "user",
		Name:         "github",
		PollInterval: time.Second * 60,
	}

	assert.Equal(t, expectedConfig, r1)
}

func TestLoadInvalidConfig_NoScrapers(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	// nolint:staticcheck
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-noscrapers.yaml"), factories)

	require.Contains(t, err.Error(), "must specify at least one scraper")
}

func TestLoadInvalidConfig_InvalidScraperKey(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
	// nolint:staticcheck
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalidscraperkey.yaml"), factories)

	require.Contains(t, err.Error(), "error reading configuration for \"github\": invalid scraper key: \"invalidscraperkey\"")
}

func TestConfig_Unmarshal(t *testing.T) {
	type fields struct {
		ControllerConfig     scraperhelper.ControllerConfig
		Scrapers             map[string]internal.Config
		MetricsBuilderConfig metadata.MetricsBuilderConfig
	}

	type args struct {
		componentParser *confmap.Conf
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "Empty Component Parser",
			fields:  fields{},
			args:    args{componentParser: nil},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{
				ControllerConfig:     test.fields.ControllerConfig,
				Scrapers:             test.fields.Scrapers,
				MetricsBuilderConfig: test.fields.MetricsBuilderConfig,
			}
			if err := cfg.Unmarshal(test.args.componentParser); (err != nil) != test.wantErr {
				t.Errorf("Config.Unmarshal() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		errExpected bool
		errText     string
		config      Config
	}{
		{
			desc:        "pass simple",
			errExpected: false,
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
				},
				Scrapers: map[string]internal.Config{
					metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
				},
				AccessToken:  "AccessToken",
				LogType:      "user",
				Name:         "testName",
				PollInterval: 60 * time.Second,
			},
		},
		{
			desc:        "fail no access token",
			errExpected: true,
			errText:     "missing access_token; required",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
				},
				Scrapers: map[string]internal.Config{
					metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
				},
				LogType:      "user",
				Name:         "testName",
				PollInterval: 60 * time.Second,
			},
		},
		{
			desc:        "fail no log type",
			errExpected: true,
			errText:     "missing log_type; required",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
				},
				Scrapers: map[string]internal.Config{
					metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
				},
				AccessToken:  "AccessToken",
				Name:         "testName",
				PollInterval: 60 * time.Second,
			},
		},
		{
			desc:        "fail no name",
			errExpected: true,
			errText:     "missing name; required",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
				},
				Scrapers: map[string]internal.Config{
					metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
				},
				AccessToken:  "AccessToken",
				LogType:      "user",
				PollInterval: 60 * time.Second,
			},
		},
		{
			desc:        "fail no name",
			errExpected: true,
			errText:     "missing name; required",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
				},
				Scrapers: map[string]internal.Config{
					metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
				},
				AccessToken:  "AccessToken",
				LogType:      "user",
				PollInterval: 60 * time.Second,
			},
		},
		{
			desc:        "fail with no poll interval",
			errExpected: true,
			errText:     "missing poll_interval; required",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
				},
				Scrapers: map[string]internal.Config{
					metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
				},
				AccessToken: "AccessToken",
				LogType:     "user",
				Name:        "testName",
			},
		},
		{
			desc:        "fail invalid poll interval short",
			errExpected: true,
			errText:     "invalid poll_interval; must be at least 0.72 seconds",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       1 * time.Second,
				},
				Scrapers: map[string]internal.Config{
					metadata.Type.String(): (&githubscraper.Factory{}).CreateDefaultConfig(),
				},
				AccessToken:  "AccessToken",
				LogType:      "user",
				Name:         "testName",
				PollInterval: 700 * time.Millisecond,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {

			err := component.ValidateConfig(tc.config)

			if tc.errExpected {
				require.EqualError(t, err, tc.errText)
				return
			}

			require.NoError(t, err)
		})
	}
}

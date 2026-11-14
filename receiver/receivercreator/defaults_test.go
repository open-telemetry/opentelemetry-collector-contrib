// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/component/xcomponentdiscovery"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/xscraper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var fakeScraperType = component.MustNewType("fake_scraper")

type fakeScraperConfig struct {
	Endpoint string `mapstructure:"endpoint"`
	Username string `mapstructure:"username"`
}

func (c *fakeScraperConfig) Validate() error {
	if c.Username == "" {
		return errors.New("username must be set")
	}
	return nil
}

// newFakeScraperFactory returns a scraper factory with the given discovery rule.
// Configs of created scrapers are sent to created.
func newFakeScraperFactory(rule xcomponentdiscovery.Rule, created chan<- *fakeScraperConfig) scraper.Factory {
	return xscraper.NewFactory(
		fakeScraperType,
		func() component.Config { return &fakeScraperConfig{} },
		xscraper.WithMetrics(func(_ context.Context, _ scraper.Settings, cfg component.Config) (scraper.Metrics, error) {
			if created != nil {
				created <- cfg.(*fakeScraperConfig)
			}
			return scraper.NewMetrics(func(context.Context) (pmetric.Metrics, error) {
				return pmetric.NewMetrics(), nil
			})
		}, component.StabilityLevelAlpha),
		xscraper.WithDiscoveryRule(rule),
	)
}

func fakeDiscoveryRule() xcomponentdiscovery.Rule {
	return xcomponentdiscovery.Rule{
		DefaultConfig: map[string]any{"username": "default-user"},
		Endpoints: map[xcomponentdiscovery.EndpointType]xcomponentdiscovery.EndpointRule{
			"port":     {Rule: `container_image matches "(?i)redis"`},
			"hostport": {Rule: `process_name == "redis-server"`},
		},
	}
}

type scraperFactoryHost struct {
	component.Host
	scrapers map[component.Type]scraper.Factory
}

func (h *scraperFactoryHost) GetScraperFactory(typ component.Type) scraper.Factory {
	return h.scrapers[typ]
}

func TestDefaultsConfigUnmarshal(t *testing.T) {
	cm := confmap.NewFromStringMap(map[string]any{
		"discovery": map[string]any{
			"defaults": map[string]any{
				"scrapers": map[string]any{
					"redis": nil,
					"mysql": map[string]any{"config": map[string]any{"username": "u"}},
				},
			},
		},
	})
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t, map[string]DefaultComponentConfig{
		"redis": {},
		"mysql": {Config: userConfigMap{"username": "u"}},
	}, cfg.Discovery.Defaults.Scrapers)
}

func TestResolveDefaultScrapers(t *testing.T) {
	noRuleFactory := xscraper.NewFactory(fakeScraperType, func() component.Config { return &fakeScraperConfig{} })
	invalidRuleFactory := newFakeScraperFactory(xcomponentdiscovery.Rule{
		Endpoints: map[xcomponentdiscovery.EndpointType]xcomponentdiscovery.EndpointRule{"port": {Rule: "port =="}},
	}, nil)

	tests := []struct {
		name          string
		scrapers      map[string]DefaultComponentConfig
		templates     map[string]receiverTemplate
		host          component.Host
		wantErr       string
		wantTemplates int
	}{
		{
			name: "nothing listed",
			host: componenttest.NewNopHost(),
		},
		{
			name:     "host without scraper factories",
			scrapers: map[string]DefaultComponentConfig{"fake_scraper": {}},
			host:     componenttest.NewNopHost(),
			wantErr:  "discovery::defaults::scrapers requires a host that provides scraper factories",
		},
		{
			name:     "invalid type",
			scrapers: map[string]DefaultComponentConfig{"Not A Type": {}},
			host:     &scraperFactoryHost{Host: componenttest.NewNopHost()},
			wantErr:  `invalid scraper type "Not A Type"`,
		},
		{
			name:     "not in the build",
			scrapers: map[string]DefaultComponentConfig{"fake_scraper": {}},
			host:     &scraperFactoryHost{Host: componenttest.NewNopHost()},
			wantErr:  "discovery::defaults::scrapers::fake_scraper: scraper is not in the build",
		},
		{
			name:     "no discovery rule",
			scrapers: map[string]DefaultComponentConfig{"fake_scraper": {}},
			host:     &scraperFactoryHost{Host: componenttest.NewNopHost(), scrapers: map[component.Type]scraper.Factory{fakeScraperType: noRuleFactory}},
			wantErr:  "discovery::defaults::scrapers::fake_scraper: scraper has no default discovery rule",
		},
		{
			name:     "invalid rule",
			scrapers: map[string]DefaultComponentConfig{"fake_scraper": {}},
			host:     &scraperFactoryHost{Host: componenttest.NewNopHost(), scrapers: map[component.Type]scraper.Factory{fakeScraperType: invalidRuleFactory}},
			wantErr:  `discovery::defaults::scrapers::fake_scraper: invalid rule for endpoint type "port"`,
		},
		{
			name:      "explicit template of the same type",
			scrapers:  map[string]DefaultComponentConfig{"fake_scraper": {}},
			templates: map[string]receiverTemplate{"fake_scraper/1": mustTemplate(t, "fake_scraper/1")},
			host:      &scraperFactoryHost{Host: componenttest.NewNopHost(), scrapers: map[component.Type]scraper.Factory{fakeScraperType: newFakeScraperFactory(fakeDiscoveryRule(), nil)}},
		},
		{
			name:          "resolved",
			scrapers:      map[string]DefaultComponentConfig{"fake_scraper": {}},
			host:          &scraperFactoryHost{Host: componenttest.NewNopHost(), scrapers: map[component.Type]scraper.Factory{fakeScraperType: newFakeScraperFactory(fakeDiscoveryRule(), nil)}},
			wantTemplates: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Discovery.Defaults.Scrapers = tt.scrapers
			maps.Copy(cfg.receiverTemplates, tt.templates)
			templates, err := resolveDefaultScrapers(cfg, tt.host, zap.NewNop())
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Len(t, templates, tt.wantTemplates)
		})
	}
}

func TestResolveDefaultScrapersMergesConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Discovery.Defaults.Scrapers = map[string]DefaultComponentConfig{
		"fake_scraper": {Config: userConfigMap{"username": "user-override"}},
	}
	host := &scraperFactoryHost{Host: componenttest.NewNopHost(), scrapers: map[component.Type]scraper.Factory{
		fakeScraperType: newFakeScraperFactory(fakeDiscoveryRule(), nil),
	}}
	templates, err := resolveDefaultScrapers(cfg, host, zap.NewNop())
	require.NoError(t, err)
	require.Len(t, templates, 1)
	tmpl := templates[0]
	assert.Equal(t, component.NewID(fakeScraperType), tmpl.receiverConfig.id)
	assert.Equal(t, userConfigMap{"username": "user-override"}, tmpl.receiverConfig.config)
	assert.Equal(t, receiverSignals{metrics: true}, tmpl.signals)
	assert.NotNil(t, tmpl.receiverConfig.scraperFactory)
	assert.Len(t, tmpl.endpointRules, 2)

	matches, err := tmpl.matches(portEndpoint, mustEnv(t, portEndpoint))
	require.NoError(t, err)
	assert.True(t, matches)
	matches, err = tmpl.matches(hostportEndpoint, mustEnv(t, hostportEndpoint))
	require.NoError(t, err)
	assert.False(t, matches)
	matches, err = tmpl.matches(podEndpoint, mustEnv(t, podEndpoint))
	require.NoError(t, err)
	assert.False(t, matches, "endpoint types without a rule never match")
}

func TestClaimedByMetricsHints(t *testing.T) {
	withAnnotations := func(annotations map[string]string) observer.Endpoint {
		p := *portEndpoint.Details.(*observer.Port)
		p.Pod.Annotations = annotations
		return observer.Endpoint{ID: "port-hints", Target: portEndpoint.Target, Details: &p}
	}
	tests := []struct {
		name               string
		endpoint           observer.Endpoint
		hintsEnabled       bool
		defaultAnnotations map[string]string
		want               bool
	}{
		{name: "no annotations", endpoint: portEndpoint, want: false},
		{name: "not a port endpoint", endpoint: hostportEndpoint, want: false},
		{name: "opted out", endpoint: withAnnotations(map[string]string{otelMetricsHints + "/enabled": "false"}), want: true},
		{name: "opted out per port", endpoint: withAnnotations(map[string]string{otelMetricsHints + ".1234/enabled": "false"}), want: true},
		{name: "hinted scraper", endpoint: withAnnotations(map[string]string{otelMetricsHints + "/enabled": "true", otelMetricsHints + "/scraper": "redis"}), want: true},
		{name: "hinted scraper with hints enabled", endpoint: withAnnotations(map[string]string{otelMetricsHints + "/enabled": "true", otelMetricsHints + "/scraper": "redis"}), hintsEnabled: true, want: true},
		{name: "enabled without scraper", endpoint: withAnnotations(map[string]string{otelMetricsHints + "/enabled": "true"}), want: false},
		{
			name:               "default annotations count when hints are enabled",
			endpoint:           portEndpoint,
			hintsEnabled:       true,
			defaultAnnotations: map[string]string{otelMetricsHints + "/enabled": "false"},
			want:               true,
		},
		{
			name:               "default annotations ignored when hints are disabled",
			endpoint:           portEndpoint,
			defaultAnnotations: map[string]string{otelMetricsHints + "/enabled": "false"},
			want:               false,
		},
		{
			name:               "default enabled without scraper doesn't claim",
			endpoint:           portEndpoint,
			hintsEnabled:       true,
			defaultAnnotations: map[string]string{otelMetricsHints + "/enabled": "true"},
			want:               false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DiscoveryConfig{Enabled: tt.hintsEnabled, DefaultAnnotations: tt.defaultAnnotations}
			assert.Equal(t, tt.want, claimedByMetricsHints(cfg, mustEnv(t, tt.endpoint)))
		})
	}
}

func TestOnAddDefaults(t *testing.T) {
	optedOut := *portEndpoint.Details.(*observer.Port)
	optedOut.Pod.Annotations = map[string]string{otelMetricsHints + "/enabled": "false"}
	optedOutEndpoint := observer.Endpoint{ID: "port-opted-out", Target: "localhost:1234", Details: &optedOut}

	tests := []struct {
		name        string
		endpoint    observer.Endpoint
		overrides   userConfigMap
		wantStarted bool
		wantConfig  *fakeScraperConfig
	}{
		{
			name:        "matching endpoint",
			endpoint:    portEndpoint,
			wantStarted: true,
			wantConfig:  &fakeScraperConfig{Endpoint: "localhost:1234", Username: "default-user"},
		},
		{
			name:        "user override",
			endpoint:    portEndpoint,
			overrides:   userConfigMap{"username": "u"},
			wantStarted: true,
			wantConfig:  &fakeScraperConfig{Endpoint: "localhost:1234", Username: "u"},
		},
		{
			name:     "missing credentials skip the endpoint",
			endpoint: portEndpoint,
			// An empty username overrides the default and fails Validate.
			overrides: userConfigMap{"username": ""},
		},
		{name: "rule doesn't match", endpoint: hostportEndpoint},
		{name: "claimed by hints", endpoint: portEndpointWithHints},
		{name: "opted out", endpoint: optedOutEndpoint},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created := make(chan *fakeScraperConfig, 1)
			cfg := createDefaultConfig().(*Config)
			cfg.Discovery.Defaults.Scrapers = map[string]DefaultComponentConfig{"fake_scraper": {Config: tt.overrides}}
			host := &scraperFactoryHost{Host: componenttest.NewNopHost(), scrapers: map[component.Type]scraper.Factory{
				fakeScraperType: newFakeScraperFactory(fakeDiscoveryRule(), created),
			}}
			defaultTemplates, err := resolveDefaultScrapers(cfg, host, zap.NewNop())
			require.NoError(t, err)

			handler, mr := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil, nil)
			handler.defaultTemplates = defaultTemplates
			handler.OnAdd([]observer.Endpoint{tt.endpoint})

			if !tt.wantStarted {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				return
			}
			require.NoError(t, mr.lastError)
			require.Equal(t, 1, handler.receiversByEndpointID.Size())
			assert.Equal(t, tt.wantConfig, <-created)

			handler.OnRemove([]observer.Endpoint{tt.endpoint})
			require.NoError(t, mr.lastError)
			assert.Equal(t, 0, handler.receiversByEndpointID.Size())
		})
	}
}

func mustTemplate(t *testing.T, name string) receiverTemplate {
	tmpl, err := newReceiverTemplate(name, nil)
	require.NoError(t, err)
	return tmpl
}

func mustEnv(t *testing.T, e observer.Endpoint) observer.EndpointEnv {
	env, err := e.Env()
	require.NoError(t, err)
	return env
}

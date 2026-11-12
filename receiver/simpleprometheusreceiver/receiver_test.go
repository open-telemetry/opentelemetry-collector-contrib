// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package simpleprometheusreceiver

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/client-go/rest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver/internal/metadata"
)

func TestReceiver(t *testing.T) {
	f := NewFactory()
	tests := []struct {
		name              string
		useServiceAccount bool
		wantError         bool
	}{
		{
			name: "success",
		},
		{
			name:              "fails to get prometheus config",
			useServiceAccount: true,
			wantError:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.UseServiceAccount = tt.useServiceAccount

			r, err := f.CreateMetrics(
				t.Context(),
				receivertest.NewNopSettings(metadata.Type),
				cfg,
				consumertest.NewNop(),
			)

			if !tt.wantError {
				require.NoError(t, err)
				require.NotNil(t, r)

				require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
				require.NoError(t, r.Shutdown(t.Context()))
				return
			}

			require.Error(t, r.Start(t.Context(), componenttest.NewNopHost()))
		})
	}
}

func TestGetPrometheusConfigServiceAccount(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("token"), 0o600))

	tests := []struct {
		name       string
		restConfig *rest.Config
		wantErr    string
	}{
		{
			name:       "uses token file",
			restConfig: &rest.Config{BearerToken: "token", BearerTokenFile: tokenFile},
		},
		{
			name:       "empty token",
			restConfig: &rest.Config{BearerTokenFile: tokenFile},
			wantErr:    "bearer token was empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := inClusterConfig
			t.Cleanup(func() { inClusterConfig = orig })
			inClusterConfig = func() (*rest.Config, error) { return tt.restConfig, nil }

			cfg := createDefaultConfig().(*Config)
			cfg.UseServiceAccount = true
			got, err := getPrometheusConfig(cfg, zap.NewNop())
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			wantAuth := &configutil.Authorization{Type: "Bearer", CredentialsFile: tokenFile}
			assertAuth := func(httpConfig configutil.HTTPClientConfig) {
				t.Helper()
				assert.Equal(t, wantAuth, httpConfig.Authorization)
				assert.Empty(t, httpConfig.BearerToken)
				assert.Empty(t, httpConfig.BearerTokenFile)
			}
			assertAuth(got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig)

			// The prometheus receiver re-marshals its config through YAML on
			// start; the token file must survive that round trip so the token
			// is re-read on each scrape instead of being fixed at start.
			require.NoError(t, got.PrometheusConfig.Reload())
			require.Len(t, got.PrometheusConfig.ScrapeConfigs, 1)
			assertAuth(got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig)
		})
	}
}

func TestGetPrometheusConfig(t *testing.T) {
	// Expected scrape configs carry the Prometheus scrape defaults (compression,
	// fallback protocol, redirects, HTTP/2) that the wrapper now starts from.
	clientConfigTLS := confighttp.NewDefaultClientConfig()
	clientConfigTLS.Endpoint = "localhost:1234"
	clientConfigTLS.TLS = configtls.ClientConfig{
		Insecure: true,
	}

	clientConfigCA := confighttp.NewDefaultClientConfig()
	clientConfigCA.Endpoint = "localhost:1234"
	clientConfigCA.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile: "./testdata/test_cert.pem",
		},
		InsecureSkipVerify: true,
	}

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "localhost:1234"

	clientConfigJobName := confighttp.NewDefaultClientConfig()
	clientConfigJobName.Endpoint = "localhost:1234"

	clientConfigWithTimeout := func(timeout time.Duration) confighttp.ClientConfig {
		c := confighttp.NewDefaultClientConfig()
		c.Endpoint = "localhost:1234"
		c.Timeout = timeout
		return c
	}

	wantWithScrapeTimeout := func(scrapeTimeout time.Duration) *prometheusreceiver.Config {
		return &prometheusreceiver.Config{
			PrometheusConfig: &prometheusreceiver.PromConfig{
				GlobalConfig: config.DefaultGlobalConfig,
				ScrapeConfigs: []*config.ScrapeConfig{
					{
						JobName:         "prometheus_simple/localhost:1234",
						HonorTimestamps: true,
						ScrapeInterval:  model.Duration(10 * time.Second),
						ScrapeTimeout:   model.Duration(scrapeTimeout),
						MetricsPath:     "/metrics",
						Scheme:          "https",
						ServiceDiscoveryConfigs: discovery.Configs{
							discovery.StaticConfig{
								{
									Targets: []model.LabelSet{
										{model.AddressLabel: model.LabelValue("localhost:1234")},
									},
									Labels: model.LabelSet{},
								},
							},
						},
						EnableCompression:      true,
						ScrapeFallbackProtocol: config.PrometheusText0_0_4,
						HTTPClientConfig: configutil.HTTPClientConfig{
							FollowRedirects: true,
							EnableHTTP2:     true,
							TLSConfig:       configutil.TLSConfig{MinVersion: configutil.TLSVersions["TLS12"]},
						},
					},
				},
			},
		}
	}

	clientConfigWith := func(headers configopaque.MapList, proxyURL string) confighttp.ClientConfig {
		c := confighttp.NewDefaultClientConfig()
		c.Endpoint = "localhost:1234"
		c.Headers = headers
		c.ProxyURL = proxyURL
		return c
	}

	wantWithHTTPClientConfig := func(httpConfig configutil.HTTPClientConfig) *prometheusreceiver.Config {
		want := wantWithScrapeTimeout(10 * time.Second)
		// The Prometheus HTTP client defaults are kept alongside the forwarded settings.
		httpConfig.FollowRedirects = true
		httpConfig.EnableHTTP2 = true
		// The collector's TLS defaults are forwarded even without a tls block.
		httpConfig.TLSConfig.MinVersion = configutil.TLSVersions["TLS12"]
		want.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig = httpConfig
		return want
	}

	tests := []struct {
		name     string
		config   *Config
		want     *prometheusreceiver.Config
		wantWarn bool
	}{
		{
			name: "Test without TLS",
			config: &Config{
				ClientConfig:       clientConfigTLS,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				Params:             url.Values{"foo": []string{"bar", "foobar"}},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							Scheme:          "http",
							MetricsPath:     "/metric",
							Params:          url.Values{"foo": []string{"bar", "foobar"}},
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
										Labels: map[model.LabelName]model.LabelValue{},
									},
								},
							},
							EnableCompression:      true,
							ScrapeFallbackProtocol: config.PrometheusText0_0_4,
							HTTPClientConfig: configutil.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
							},
						},
					},
				},
			},
		},
		{
			name: "Test with job name",
			config: &Config{
				ClientConfig:       clientConfigJobName,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metric",
				JobName:            "job123",
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							JobName:         "job123",
							HonorTimestamps: true,
							Scheme:          "https",
							MetricsPath:     "/metric",
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
										Labels: map[model.LabelName]model.LabelValue{},
									},
								},
							},
							EnableCompression:      true,
							ScrapeFallbackProtocol: config.PrometheusText0_0_4,
							HTTPClientConfig: configutil.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
								TLSConfig:       configutil.TLSConfig{MinVersion: configutil.TLSVersions["TLS12"]},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with TLS",
			config: &Config{
				ClientConfig:       clientConfigCA,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							MetricsPath:     "/metrics",
							Scheme:          "https",
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{model.AddressLabel: model.LabelValue("localhost:1234")},
										},
										Labels: map[model.LabelName]model.LabelValue{},
									},
								},
							},
							EnableCompression:      true,
							ScrapeFallbackProtocol: config.PrometheusText0_0_4,
							HTTPClientConfig: configutil.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
								TLSConfig: configutil.TLSConfig{
									CAFile:             "./testdata/test_cert.pem",
									InsecureSkipVerify: true,
									MinVersion:         configutil.TLSVersions["TLS12"],
								},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with TLS - default CA",
			config: &Config{
				ClientConfig:       clientConfig,
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
				Labels: map[string]string{
					"key": "value",
				},
			},
			want: &prometheusreceiver.Config{
				PrometheusConfig: &prometheusreceiver.PromConfig{
					GlobalConfig: config.DefaultGlobalConfig,
					ScrapeConfigs: []*config.ScrapeConfig{
						{
							JobName:         "prometheus_simple/localhost:1234",
							HonorTimestamps: true,
							ScrapeInterval:  model.Duration(10 * time.Second),
							ScrapeTimeout:   model.Duration(10 * time.Second),
							MetricsPath:     "/metrics",
							Scheme:          "https",
							ServiceDiscoveryConfigs: discovery.Configs{
								discovery.StaticConfig{
									{
										Targets: []model.LabelSet{
											{
												model.AddressLabel: model.LabelValue("localhost:1234"),
											},
										},
										Labels: model.LabelSet{
											model.LabelName("key"): model.LabelValue("value"),
										},
									},
								},
							},
							EnableCompression:      true,
							ScrapeFallbackProtocol: config.PrometheusText0_0_4,
							HTTPClientConfig: configutil.HTTPClientConfig{
								FollowRedirects: true,
								EnableHTTP2:     true,
								TLSConfig:       configutil.TLSConfig{MinVersion: configutil.TLSVersions["TLS12"]},
							},
						},
					},
				},
			},
		},
		{
			name: "Test with timeout unset",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(0),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithScrapeTimeout(10 * time.Second),
		},
		{
			name: "Test with timeout less than collection_interval",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(5 * time.Second),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithScrapeTimeout(5 * time.Second),
		},
		{
			name: "Test with timeout equal to collection_interval",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(10 * time.Second),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithScrapeTimeout(10 * time.Second),
		},
		{
			name: "Test with timeout greater than collection_interval",
			config: &Config{
				ClientConfig:       clientConfigWithTimeout(30 * time.Second),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want:     wantWithScrapeTimeout(10 * time.Second),
			wantWarn: true,
		},
		{
			name: "Test with no headers",
			config: &Config{
				ClientConfig:       clientConfigWith(configopaque.MapList{}, ""),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithHTTPClientConfig(configutil.HTTPClientConfig{}),
		},
		{
			name: "Test with one header",
			config: &Config{
				ClientConfig: clientConfigWith(configopaque.MapList{
					{Name: "X-Scope-OrgID", Value: "tenant-a"},
				}, ""),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithHTTPClientConfig(configutil.HTTPClientConfig{
				HTTPHeaders: &configutil.Headers{Headers: map[string]configutil.Header{
					"X-Scope-OrgID": {Secrets: []configutil.Secret{"tenant-a"}},
				}},
			}),
		},
		{
			name: "Test with several headers",
			config: &Config{
				ClientConfig: clientConfigWith(configopaque.MapList{
					{Name: "X-Scope-OrgID", Value: "tenant-a"},
					{Name: "X-Api-Key", Value: "secret"},
				}, ""),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithHTTPClientConfig(configutil.HTTPClientConfig{
				HTTPHeaders: &configutil.Headers{Headers: map[string]configutil.Header{
					"X-Scope-OrgID": {Secrets: []configutil.Secret{"tenant-a"}},
					"X-Api-Key":     {Secrets: []configutil.Secret{"secret"}},
				}},
			}),
		},
		{
			name: "Test with proxy_url",
			config: &Config{
				ClientConfig:       clientConfigWith(nil, "http://proxy.local:3128"),
				CollectionInterval: 10 * time.Second,
				MetricsPath:        "/metrics",
			},
			want: wantWithHTTPClientConfig(configutil.HTTPClientConfig{
				ProxyConfig: configutil.ProxyConfig{
					ProxyURL: configutil.URL{URL: &url.URL{Scheme: "http", Host: "proxy.local:3128"}},
				},
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.WarnLevel)
			got, err := getPrometheusConfig(tt.config, zap.New(core))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if tt.wantWarn {
				require.Equal(t, 1, logs.Len())
				assert.Equal(t, tt.config.ClientConfig.Timeout, logs.All()[0].ContextMap()["timeout"])
				assert.Equal(t, tt.config.CollectionInterval, logs.All()[0].ContextMap()["collection_interval"])
			} else {
				assert.Equal(t, 0, logs.Len())
			}

			// The prometheus receiver re-marshals its config through YAML on
			// start; the scrape timeout must survive that round trip.
			wantTimeout := got.PrometheusConfig.ScrapeConfigs[0].ScrapeTimeout
			require.NoError(t, got.PrometheusConfig.Reload())
			require.Len(t, got.PrometheusConfig.ScrapeConfigs, 1)
			assert.Equal(t, wantTimeout, got.PrometheusConfig.ScrapeConfigs[0].ScrapeTimeout)

			// Headers and proxy_url must survive that round trip unchanged.
			wantHTTPConfig := tt.want.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig
			gotHTTPConfig := got.PrometheusConfig.ScrapeConfigs[0].HTTPClientConfig
			assert.Equal(t, wantHTTPConfig.HTTPHeaders, gotHTTPConfig.HTTPHeaders)
			assert.Equal(t, wantHTTPConfig.ProxyURL, gotHTTPConfig.ProxyURL)
		})
	}
}

func TestGetPrometheusConfigScheme(t *testing.T) {
	certPem, keyPem := generateSelfSignedPEM(t)
	certFile := filepath.Join(t.TempDir(), "cert.pem")
	keyFile := filepath.Join(t.TempDir(), "key.pem")
	require.NoError(t, os.WriteFile(certFile, certPem, 0o600))
	require.NoError(t, os.WriteFile(keyFile, keyPem, 0o600))

	tests := []struct {
		name       string
		tls        configtls.ClientConfig
		wantScheme string
		wantTLS    configutil.TLSConfig
	}{
		{
			name:       "default insecure",
			tls:        configtls.ClientConfig{Insecure: true},
			wantScheme: "http",
		},
		{
			name:       "insecure false",
			tls:        configtls.ClientConfig{Insecure: false},
			wantScheme: "https",
			wantTLS:    configutil.TLSConfig{MinVersion: configutil.TLSVersions["TLS12"]},
		},
		{
			name: "ca_file with default insecure",
			tls: configtls.ClientConfig{
				Config:   configtls.Config{CAFile: "./testdata/test_cert.pem"},
				Insecure: true,
			},
			wantScheme: "https",
			wantTLS: configutil.TLSConfig{
				CAFile:     "./testdata/test_cert.pem",
				MinVersion: configutil.TLSVersions["TLS12"],
			},
		},
		{
			name: "cert_file and key_file without ca_file are not used with default insecure",
			tls: configtls.ClientConfig{
				Config: configtls.Config{
					CertFile: "./testdata/test_cert.pem",
					KeyFile:  "./testdata/test_cert.pem",
				},
				Insecure:           true,
				InsecureSkipVerify: true,
			},
			wantScheme: "http",
		},
		{
			name: "cert_file and key_file with insecure false",
			tls: configtls.ClientConfig{
				Config:   configtls.Config{CertFile: certFile, KeyFile: keyFile},
				Insecure: false,
			},
			wantScheme: "https",
			wantTLS: configutil.TLSConfig{
				CertFile:   certFile,
				KeyFile:    keyFile,
				MinVersion: configutil.TLSVersions["TLS12"],
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.TLS = tt.tls
			got, err := getPrometheusConfig(cfg, zap.NewNop())
			require.NoError(t, err)
			sc := got.PrometheusConfig.ScrapeConfigs[0]
			assert.Equal(t, tt.wantScheme, sc.Scheme)
			assert.Equal(t, tt.wantTLS, sc.HTTPClientConfig.TLSConfig)
			assert.Equal(t, sc.ScrapeInterval, sc.ScrapeTimeout)
		})
	}
}

// The prometheus receiver re-marshals its config through YAML on start, so
// every scrape setting the wrapper leaves at its Go zero value is written out
// and replaces the Prometheus default. The scrape defaults must therefore
// survive that round trip and match the prometheus receiver's own.
func TestGetPrometheusConfigScrapeDefaults(t *testing.T) {
	tests := []struct {
		name string
		tls  configtls.ClientConfig
	}{
		{
			name: "http",
			tls:  configtls.ClientConfig{Insecure: true},
		},
		{
			name: "https",
			tls: configtls.ClientConfig{
				Config: configtls.Config{CAFile: "./testdata/test_cert.pem"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.TLS = tt.tls
			got, err := getPrometheusConfig(cfg, zap.NewNop())
			require.NoError(t, err)
			require.NoError(t, got.PrometheusConfig.Reload())
			require.Len(t, got.PrometheusConfig.ScrapeConfigs, 1)
			sc := got.PrometheusConfig.ScrapeConfigs[0]

			assert.True(t, sc.HTTPClientConfig.FollowRedirects, "follow_redirects")
			assert.True(t, sc.HTTPClientConfig.EnableHTTP2, "enable_http2")
			assert.True(t, sc.EnableCompression, "enable_compression")
			assert.Equal(t, config.PrometheusText0_0_4, sc.ScrapeFallbackProtocol, "fallback_scrape_protocol")

			want := prometheusReceiverScrapeConfig(t)
			assert.Equal(t, want.HTTPClientConfig.FollowRedirects, sc.HTTPClientConfig.FollowRedirects)
			assert.Equal(t, want.HTTPClientConfig.EnableHTTP2, sc.HTTPClientConfig.EnableHTTP2)
			assert.Equal(t, want.EnableCompression, sc.EnableCompression)
			assert.Equal(t, want.ScrapeFallbackProtocol, sc.ScrapeFallbackProtocol)
		})
	}
}

// prometheusReceiverScrapeConfig returns the scrape config the prometheus
// receiver builds for a minimal user config of the same target.
func prometheusReceiverScrapeConfig(t *testing.T) *config.ScrapeConfig {
	t.Helper()
	cm := confmap.NewFromStringMap(map[string]any{
		"config": map[string]any{
			"scrape_configs": []any{
				map[string]any{
					"job_name": "prometheus",
					"static_configs": []any{
						map[string]any{"targets": []any{defaultEndpoint}},
					},
				},
			},
		},
	})
	cfg := prometheusreceiver.NewFactory().CreateDefaultConfig()
	require.NoError(t, cm.Unmarshal(cfg))
	require.NoError(t, confmap.Validate(cfg))
	scrapeConfigs := cfg.(*prometheusreceiver.Config).PrometheusConfig.ScrapeConfigs
	require.Len(t, scrapeConfigs, 1)
	return scrapeConfigs[0]
}

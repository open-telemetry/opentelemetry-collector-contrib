// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	promConfig "github.com/prometheus/common/config"
	promModel "github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/file"
	"github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	r0 := cfg.(*Config)
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "customname").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	r1 := cfg.(*Config)
	assert.Equal(t, "demo", r1.PrometheusConfig.ScrapeConfigs[0].JobName)
	assert.Equal(t, 5*time.Second, time.Duration(r1.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval))
	assert.True(t, r1.TrimMetricSuffixes)
	assert.True(t, r1.ReportExtraScrapeMetrics)

	ta := r1.TargetAllocator.Get()
	assert.Equal(t, "http://my-targetallocator-service", ta.Endpoint)
	assert.Equal(t, 30*time.Second, ta.Interval)
	assert.Equal(t, "collector-1", ta.CollectorID)
	assert.Equal(t, promModel.Duration(60*time.Second), ta.HTTPSDConfig.RefreshInterval)
	assert.Equal(t, "prometheus", ta.HTTPSDConfig.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("changeme"), ta.HTTPSDConfig.HTTPClientConfig.BasicAuth.Password)
	assert.Equal(t, "scrape_prometheus", ta.HTTPScrapeConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("scrape_changeme"), ta.HTTPScrapeConfig.BasicAuth.Password)
}

func TestLoadTargetAllocatorConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_target_allocator.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r0 := cfg.(*Config)
	assert.NotNil(t, r0.PrometheusConfig)
	ta0 := r0.TargetAllocator.Get()
	assert.Equal(t, "http://localhost:8080", ta0.Endpoint)
	assert.Equal(t, 5*time.Second, ta0.Timeout)
	assert.Equal(t, "client.crt", ta0.TLS.CertFile)
	assert.Equal(t, "client.key", ta0.TLS.KeyFile)
	assert.Equal(t, 30*time.Second, ta0.Interval)
	assert.Equal(t, "collector-1", ta0.CollectorID)
	assert.NotNil(t, r0.PrometheusConfig)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withScrape").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r1 := cfg.(*Config)
	assert.NotNil(t, r0.PrometheusConfig)
	ta1 := r0.TargetAllocator.Get()
	assert.Equal(t, "http://localhost:8080", ta1.Endpoint)
	assert.Equal(t, 30*time.Second, ta1.Interval)
	assert.Equal(t, "collector-1", ta1.CollectorID)

	assert.Len(t, r1.PrometheusConfig.ScrapeConfigs, 1)
	assert.Equal(t, "demo", r1.PrometheusConfig.ScrapeConfigs[0].JobName)
	assert.Equal(t, promModel.Duration(5*time.Second), r1.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withOnlyScrape").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r2 := cfg.(*Config)
	assert.Len(t, r2.PrometheusConfig.ScrapeConfigs, 1)
	assert.Equal(t, "demo", r2.PrometheusConfig.ScrapeConfigs[0].JobName)
	assert.Equal(t, promModel.Duration(5*time.Second), r2.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval)
}

func TestValidateConfigWithScrapeConfigFiles(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_scrape_config_files.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	require.NoError(t, xconfmap.Validate(cfg))
}

func TestLoadConfigFailsOnUnknownSection(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-section.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.Error(t, sub.Unmarshal(cfg))
}

func TestLoadConfigFailsOnNoPrometheusOrTAConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-nonexistent-scrape-config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.ErrorContains(t, xconfmap.Validate(cfg), "no Prometheus scrape_configs or target_allocator set")

	cfg = factory.CreateDefaultConfig()
	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withConfigAndTA").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	cfg = factory.CreateDefaultConfig()
	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withOnlyTA").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	cfg = factory.CreateDefaultConfig()
	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withOnlyScrape").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))
}

// As one of the config parameters is consuming prometheus
// configuration as a subkey, ensure that invalid configuration
// within the subkey will also raise an error.
func TestLoadConfigFailsOnUnknownPrometheusSection(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-section.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.Error(t, sub.Unmarshal(cfg))
}

// Renaming emits a warning
func TestConfigWarningsOnRenameDisallowed(t *testing.T) {
	// Construct the config that should emit a warning
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "warning-config-prometheus-relabel.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	// Use a fake logger
	creationSet := receivertest.NewNopSettings(metadata.Type)
	observedZapCore, observedLogs := observer.New(zap.WarnLevel)
	creationSet.Logger = zap.New(observedZapCore)
	_, err = createMetricsReceiver(t.Context(), creationSet, cfg, nil)
	require.NoError(t, err)
	// We should have received a warning
	assert.Equal(t, 1, observedLogs.Len())
}

func TestRejectUnsupportedPrometheusFeatures(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-unsupported-features.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	err = xconfmap.Validate(cfg)
	require.Error(t, err)

	wantErrMsg := `unsupported features:
        alert_config.alertmanagers
        alert_config.relabel_configs
        remote_read
        remote_write
        rule_files`

	gotErrMsg := strings.ReplaceAll(err.Error(), "\t", strings.Repeat(" ", 8))
	require.Contains(t, gotErrMsg, wantErrMsg)
}

func TestNonExistentAuthCredentialsFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-nonexistent-auth-credentials-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.ErrorContains(t,
		xconfmap.Validate(cfg),
		`error checking authorization credentials file "/nonexistentauthcredentialsfile"`)
}

func TestTLSConfigNonExistentCertFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-nonexistent-cert-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.ErrorContains(t,
		xconfmap.Validate(cfg),
		`error checking client cert file "/nonexistentcertfile"`)
}

func TestTLSConfigNonExistentKeyFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-nonexistent-key-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.ErrorContains(t,
		xconfmap.Validate(cfg),
		`error checking client key file "/nonexistentkeyfile"`)
}

func TestTLSConfigCertFileWithoutKeyFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-cert-file-without-key-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)

	assert.ErrorContains(t,
		sub.Unmarshal(cfg),
		"exactly one of key or key_file must be configured when a client certificate is configured")
}

func TestTLSConfigKeyFileWithoutCertFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-key-file-without-cert-file.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	assert.ErrorContains(t,
		sub.Unmarshal(cfg),
		"exactly one of cert or cert_file must be configured when a client key is configured")
}

func TestKubernetesSDConfigWithoutKeyFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-kubernetes-sd-config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)

	assert.ErrorContains(t,
		sub.Unmarshal(cfg),
		"exactly one of key or key_file must be configured when a client certificate is configured")
}

func TestFileSDConfigJsonNilTargetGroup(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-file-sd-config-json.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	require.NoError(t, xconfmap.Validate(cfg))
}

func TestFileSDConfigYamlNilTargetGroup(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-file-sd-config-yaml.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	require.NoError(t, xconfmap.Validate(cfg))
}

func TestTargetAllocatorInvalidHTTPScrape(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid-config-prometheus-target-allocator.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.Error(t, sub.Unmarshal(cfg))
}

func TestFileSDConfigWithoutSDFile(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "nonexistent-prometheus-sd-file-config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	require.NoError(t, xconfmap.Validate(cfg))
}

func TestLoadPrometheusAPIServerExtensionConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config_prometheus_api_server.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "withAPIEnabled").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r0 := cfg.(*Config)
	assert.NotNil(t, r0.PrometheusConfig)
	assert.True(t, r0.APIServer.Enabled)
	assert.NotNil(t, r0.APIServer.ServerConfig)
	assert.Equal(t, "localhost:9090", r0.APIServer.ServerConfig.Endpoint)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withAPIDisabled").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r1 := cfg.(*Config)
	assert.False(t, r1.APIServer.Enabled)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withoutAPI").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r2 := cfg.(*Config)
	assert.NotNil(t, r2.PrometheusConfig)
	assert.False(t, r2.APIServer.Enabled)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withInvalidAPIConfig").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	require.Error(t, xconfmap.Validate(cfg))
}

func TestReloadPromConfigSecretHandling(t *testing.T) {
	// This test verifies that the Reload() method preserves secrets instead of
	// corrupting them to "<secret>" placeholders. This is critical for authentication
	// to work properly when using configurations with basic auth or bearer tokens.

	tests := []struct {
		name       string
		configYAML string
		checkFn    func(t *testing.T, dst *PromConfig)
	}{
		{
			name: "basic auth password preservation",
			configYAML: `
scrape_configs:
  - job_name: "test-basic-auth"
    basic_auth:
      username: "testuser"
      password: "mysecretpassword"
    static_configs:
      - targets: ["localhost:8080"]
`,
			checkFn: func(t *testing.T, dst *PromConfig) {
				require.Len(t, dst.ScrapeConfigs, 1)
				scrapeConfig := dst.ScrapeConfigs[0]
				assert.Equal(t, "test-basic-auth", scrapeConfig.JobName)

				// The critical check: ensure the password is not "<secret>"
				require.NotNil(t, scrapeConfig.HTTPClientConfig.BasicAuth, "basic auth should be configured")
				password := string(scrapeConfig.HTTPClientConfig.BasicAuth.Password)
				assert.Equal(t, "mysecretpassword", password, "password should preserve original value")
				assert.Equal(t, "testuser", scrapeConfig.HTTPClientConfig.BasicAuth.Username)
			},
		},
		{
			name: "bearer token preservation",
			configYAML: `
scrape_configs:
  - job_name: "test-bearer-token"
    authorization:
      type: "Bearer"
      credentials: "mySecretBearerToken123"
    static_configs:
      - targets: ["localhost:9090"]
`,
			checkFn: func(t *testing.T, dst *PromConfig) {
				require.Len(t, dst.ScrapeConfigs, 1)
				scrapeConfig := dst.ScrapeConfigs[0]
				assert.Equal(t, "test-bearer-token", scrapeConfig.JobName)

				// Check that bearer token is preserved
				require.NotNil(t, scrapeConfig.HTTPClientConfig.Authorization, "authorization should be configured")
				credentials := string(scrapeConfig.HTTPClientConfig.Authorization.Credentials)
				assert.Equal(t, "mySecretBearerToken123", credentials, "credentials should preserve original value")
				assert.Equal(t, "Bearer", scrapeConfig.HTTPClientConfig.Authorization.Type)
			},
		},
		{
			name: "oauth2 client credentials secret preservation",
			configYAML: `
scrape_configs:
  - job_name: "test-client-secredentialscret-auth"
    oauth2:
      client_id: "id-1"
      client_secret: "mySuperSecretClientSecret"
      token_url: "https://auth.example.com/token"
    static_configs:
      - targets: ["localhost:8080"]
`,
			checkFn: func(t *testing.T, dst *PromConfig) {
				require.Len(t, dst.ScrapeConfigs, 1)
				scrapeConfig := dst.ScrapeConfigs[0]
				assert.Equal(t, "test-client-secredentialscret-auth", scrapeConfig.JobName)

				// The critical check: ensure the client_secret is not "<secret>"
				require.NotNil(t, scrapeConfig.HTTPClientConfig.OAuth2, "basic auth should be configured")
				secret := string(scrapeConfig.HTTPClientConfig.OAuth2.ClientSecret)
				assert.Equal(t, "mySuperSecretClientSecret", secret, "client_secret should preserve original value")
			},
		},
		{
			name: "oauth2 jwt-bearer certificate preservation",
			configYAML: `
scrape_configs:
  - job_name: "test-jwt-bearer-auth"
    oauth2:
      client_id: "id-1"
      client_certificate_key: "mySuperSecretCertificateKey"
      token_url: "https://auth.example.com/token"
      grant_type: "urn:ietf:params:oauth:grant-type:jwt-bearer"
    static_configs:
      - targets: ["localhost:8080"]
`,
			checkFn: func(t *testing.T, dst *PromConfig) {
				require.Len(t, dst.ScrapeConfigs, 1)
				scrapeConfig := dst.ScrapeConfigs[0]
				assert.Equal(t, "test-jwt-bearer-auth", scrapeConfig.JobName)

				// The critical check: ensure the client_certificate_key is not "<secret>"
				require.NotNil(t, scrapeConfig.HTTPClientConfig.OAuth2, "basic auth should be configured")
				key := string(scrapeConfig.HTTPClientConfig.OAuth2.ClientCertificateKey)
				assert.Equal(t, "mySuperSecretCertificateKey", key, "client_certificate_key should preserve original value")
			},
		},
		{
			name: "basic auth password starting with %",
			configYAML: `
scrape_configs:
  - job_name: "foo"
    basic_auth:
      username: "user"
      password: "%password"
    static_configs:
      - targets: ["target:8000"]
`,
			checkFn: func(t *testing.T, dst *PromConfig) {
				require.Len(t, dst.ScrapeConfigs, 1)
				scrapeConfig := dst.ScrapeConfigs[0]
				assert.Equal(t, "foo", scrapeConfig.JobName)

				// Ensure basic_auth is present
				require.NotNil(t, scrapeConfig.HTTPClientConfig.BasicAuth, "basic auth should be configured")
				password := string(scrapeConfig.HTTPClientConfig.BasicAuth.Password)
				assert.Equal(t, "%password", password, "password should preserve original value with leading %")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Load the config using promconfig.Load to simulate real usage
			initialCfg, err := promconfig.Load(tt.configYAML, slog.New(slog.NewTextHandler(os.Stderr, nil)))
			require.NoError(t, err)

			// Convert to PromConfig and test the Reload method
			// The Reload method should preserve secrets and not corrupt them
			dst := (*PromConfig)(initialCfg)
			err = dst.Reload()
			require.NoError(t, err)

			// Verify that secrets are preserved
			tt.checkFn(t, dst)
		})
	}
}

func TestReloadPromConfigStaticConfigsWithLabels(t *testing.T) {
	createGroup := func(idxArr ...int) *targetgroup.Group {
		g := &targetgroup.Group{}
		g.Labels = promModel.LabelSet{}
		for _, idx := range idxArr {
			g.Targets = append(g.Targets, promModel.LabelSet{
				promModel.AddressLabel:                    promModel.LabelValue(fmt.Sprint("localhost:", 8080+idx)),
				promModel.LabelName(fmt.Sprint("k", idx)): promModel.LabelValue(fmt.Sprint("v", idx)),
			})
			g.Labels[promModel.LabelName(fmt.Sprint("label", idx))] = promModel.LabelValue(fmt.Sprint("value", idx))
		}
		return g
	}
	create := func() *PromConfig {
		return &PromConfig{
			ScrapeConfigs: []*promconfig.ScrapeConfig{
				{
					JobName: "test_job",
					ServiceDiscoveryConfigs: discovery.Configs{
						&http.SDConfig{
							URL:             "http://localhost:8080",
							RefreshInterval: promModel.Duration(time.Second) * 5,
							HTTPClientConfig: promConfig.HTTPClientConfig{
								TLSConfig: promConfig.TLSConfig{
									InsecureSkipVerify: true,
								},
							},
						},
						discovery.StaticConfig{
							createGroup(1),
						},
						&file.SDConfig{
							Files: []string{"targets.json"},
						},
						discovery.StaticConfig{
							createGroup(2),
						},
						discovery.StaticConfig{
							createGroup(3, 4),
							createGroup(5, 6),
						},
					},
				},
				{
					JobName: "another_job",
					ServiceDiscoveryConfigs: discovery.Configs{
						discovery.StaticConfig{
							createGroup(10),
						},
						&file.SDConfig{
							Files: []string{"another_targets.json"},
						},
					},
				},
			},
		}
	}
	promConfig := create()
	assert.NoError(t, promConfig.Reload())

	for _, sc := range promConfig.ScrapeConfigs {
		for _, sd := range sc.ServiceDiscoveryConfigs {
			sc, ok := sd.(discovery.StaticConfig)
			if !ok {
				continue
			}
			for _, tg := range sc {
				for _, target := range tg.Targets {
					// Ensure that the targets have the expected labels.
					assert.NotEmpty(t, target[promModel.AddressLabel])
					assert.Greater(t, len(target), 1, "target should have more than just address label")
				}
				assert.NotEmpty(t, tg.Labels, "target group should have labels")
			}
		}
	}

	data1, _ := yaml.Marshal(promConfig)
	assert.NoError(t, promConfig.Reload())
	data2, _ := yaml.Marshal(promConfig)
	assert.Equal(t, string(data1), string(data2), "Reload should not change the config")
}

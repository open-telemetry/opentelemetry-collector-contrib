// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	promModel "github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
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
	assert.True(t, r1.UseStartTimeMetric)
	assert.True(t, r1.TrimMetricSuffixes)
	assert.Equal(t, "^(.+_)*process_start_time_seconds$", r1.StartTimeMetricRegex)
	assert.True(t, r1.ReportExtraScrapeMetrics)

	assert.Equal(t, "http://my-targetallocator-service", r1.TargetAllocator.Endpoint)
	assert.Equal(t, 30*time.Second, r1.TargetAllocator.Interval)
	assert.Equal(t, "collector-1", r1.TargetAllocator.CollectorID)
	assert.Equal(t, promModel.Duration(60*time.Second), r1.TargetAllocator.HTTPSDConfig.RefreshInterval)
	assert.Equal(t, "prometheus", r1.TargetAllocator.HTTPSDConfig.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("changeme"), r1.TargetAllocator.HTTPSDConfig.HTTPClientConfig.BasicAuth.Password)
	assert.Equal(t, "scrape_prometheus", r1.TargetAllocator.HTTPScrapeConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("scrape_changeme"), r1.TargetAllocator.HTTPScrapeConfig.BasicAuth.Password)
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
	assert.Equal(t, "http://localhost:8080", r0.TargetAllocator.Endpoint)
	assert.Equal(t, 5*time.Second, r0.TargetAllocator.Timeout)
	assert.Equal(t, "client.crt", r0.TargetAllocator.TLS.CertFile)
	assert.Equal(t, 30*time.Second, r0.TargetAllocator.Interval)
	assert.Equal(t, "collector-1", r0.TargetAllocator.CollectorID)
	assert.NotNil(t, r0.PrometheusConfig)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withScrape").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r1 := cfg.(*Config)
	assert.NotNil(t, r0.PrometheusConfig)
	assert.Equal(t, "http://localhost:8080", r0.TargetAllocator.Endpoint)
	assert.Equal(t, 30*time.Second, r0.TargetAllocator.Interval)
	assert.Equal(t, "collector-1", r0.TargetAllocator.CollectorID)

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
	_, err = createMetricsReceiver(context.Background(), creationSet, cfg, nil)
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
	assert.NotNil(t, r1.APIServer)
	assert.False(t, r1.APIServer.Enabled)

	sub, err = cm.Sub(component.NewIDWithName(metadata.Type, "withoutAPI").String())
	require.NoError(t, err)
	cfg = factory.CreateDefaultConfig()
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, xconfmap.Validate(cfg))

	r2 := cfg.(*Config)
	assert.NotNil(t, r2.PrometheusConfig)
	assert.Nil(t, r2.APIServer)

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

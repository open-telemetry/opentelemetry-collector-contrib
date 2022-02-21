// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	promModel "github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[config.NewComponentID(typeStr)]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "customname")].(*Config)
	assert.Equal(t, r1.ReceiverSettings, config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "customname")))
	assert.Equal(t, r1.PrometheusConfig.ScrapeConfigs[0].JobName, "demo")
	assert.Equal(t, time.Duration(r1.PrometheusConfig.ScrapeConfigs[0].ScrapeInterval), 5*time.Second)
	assert.Equal(t, r1.UseStartTimeMetric, true)
	assert.Equal(t, r1.StartTimeMetricRegex, "^(.+_)*process_start_time_seconds$")

	assert.Equal(t, "http://my-targetallocator-service", r1.TargetAllocator.Endpoint)
	assert.Equal(t, 30*time.Second, r1.TargetAllocator.Interval)
	assert.Equal(t, "collector-1", r1.TargetAllocator.CollectorID)
	assert.Equal(t, promModel.Duration(60*time.Second), r1.TargetAllocator.HttpSDConfig.RefreshInterval)
	assert.Equal(t, "prometheus", r1.TargetAllocator.HttpSDConfig.HTTPClientConfig.BasicAuth.Username)
	assert.Equal(t, promConfig.Secret("changeme"), r1.TargetAllocator.HttpSDConfig.HTTPClientConfig.BasicAuth.Password)
}

func TestLoadConfigFailsOnUnknownSection(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-section.yaml"), factories)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

// As one of the config parameters is consuming prometheus
// configuration as a subkey, ensure that invalid configuration
// within the subkey will also raise an error.
func TestLoadConfigFailsOnUnknownPrometheusSection(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-section.yaml"), factories)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

// Renaming is not allowed
func TestLoadConfigFailsOnRenameDisallowed(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid-config-prometheus-relabel.yaml"), factories)
	assert.Error(t, err)
	assert.NotNil(t, cfg)
}

func TestRejectUnsupportedPrometheusFeatures(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-unsupported-features.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: unsupported features:
        alert_config.alertmanagers
        alert_config.relabel_configs
        remote_read
        remote_write
        rule_files`

	gotErrMsg := strings.ReplaceAll(err.Error(), "\t", strings.Repeat(" ", 8))
	require.Equal(t, wantErrMsg, gotErrMsg)

}

func TestNonExistentAuthCredentialsFile(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-non-existent-auth-credentials-file.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: error checking authorization credentials file "/nonexistentauthcredentialsfile"`

	gotErrMsg := err.Error()
	require.True(t, strings.HasPrefix(gotErrMsg, wantErrMsg))
}

func TestTLSConfigNonExistentCertFile(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-non-existent-cert-file.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: error checking client cert file "/nonexistentcertfile"`

	gotErrMsg := err.Error()
	require.True(t, strings.HasPrefix(gotErrMsg, wantErrMsg))
}

func TestTLSConfigNonExistentKeyFile(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-non-existent-key-file.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: error checking client key file "/nonexistentkeyfile"`

	gotErrMsg := err.Error()
	require.True(t, strings.HasPrefix(gotErrMsg, wantErrMsg))
}

func TestTLSConfigCertFileWithoutKeyFile(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-cert-file-without-key-file.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: client cert file "./testdata/dummy-tls-cert-file" specified without client key file`

	gotErrMsg := err.Error()
	require.Equal(t, wantErrMsg, gotErrMsg)
}

func TestTLSConfigKeyFileWithoutCertFile(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-key-file-without-cert-file.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: client key file "./testdata/dummy-tls-key-file" specified without client cert file`

	gotErrMsg := err.Error()
	require.Equal(t, wantErrMsg, gotErrMsg)
}

func TestKubernetesSDConfigWithoutKeyFile(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-kubernetes-sd-config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: client cert file "./testdata/dummy-tls-cert-file" specified without client key file`

	gotErrMsg := err.Error()
	require.Equal(t, wantErrMsg, gotErrMsg)
}

func TestFileSDConfigJsonNilTargetGroup(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-file-sd-config-json.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: checking SD file "./testdata/sd-config-with-null-target-group.json": nil target group item found (index 1)`

	gotErrMsg := err.Error()
	require.Equal(t, wantErrMsg, gotErrMsg)
}

func TestFileSDConfigYamlNilTargetGroup(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "invalid-config-prometheus-file-sd-config-yaml.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	err = cfg.Validate()
	require.NotNil(t, err, "Expected a non-nil error")

	wantErrMsg := `receiver "prometheus" has invalid configuration: checking SD file "./testdata/sd-config-with-null-target-group.yaml": nil target group item found (index 1)`

	gotErrMsg := err.Error()
	require.Equal(t, wantErrMsg, gotErrMsg)
}

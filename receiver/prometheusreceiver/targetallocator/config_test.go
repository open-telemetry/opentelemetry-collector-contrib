// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package targetallocator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/targetallocator"

import (
	"path/filepath"
	"testing"
	"time"

	promConfig "github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestComponentConfigStruct(t *testing.T) {
	require.NoError(t, componenttest.CheckConfigStruct(Config{}))
}

func TestLoadTargetAllocatorConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	cfg := &Config{}

	sub, err := cm.Sub("target_allocator")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, component.ValidateConfig(cfg))

	assert.Equal(t, "http://localhost:8080", cfg.ClientConfig.Endpoint)
	assert.Equal(t, 5*time.Second, cfg.ClientConfig.Timeout)
	assert.Equal(t, "client.crt", cfg.ClientConfig.TLSSetting.CertFile)
	assert.Equal(t, 30*time.Second, cfg.Interval)
	assert.Equal(t, "collector-1", cfg.CollectorID)
}

func TestPromHTTPClientConfigValidateAuthorization(t *testing.T) {
	cfg := PromHTTPClientConfig{}
	require.NoError(t, component.ValidateConfig(cfg))
	cfg.Authorization = &promConfig.Authorization{}
	require.NoError(t, component.ValidateConfig(cfg))
	cfg.Authorization.CredentialsFile = "none"
	require.Error(t, component.ValidateConfig(cfg))
	cfg.Authorization.CredentialsFile = filepath.Join("testdata", "dummy-tls-cert-file")
	require.NoError(t, component.ValidateConfig(cfg))
}

func TestPromHTTPClientConfigValidateTLSConfig(t *testing.T) {
	cfg := PromHTTPClientConfig{}
	require.NoError(t, component.ValidateConfig(cfg))
	cfg.TLSConfig.CertFile = "none"
	require.Error(t, component.ValidateConfig(cfg))
	cfg.TLSConfig.CertFile = filepath.Join("testdata", "dummy-tls-cert-file")
	cfg.TLSConfig.KeyFile = "none"
	require.Error(t, component.ValidateConfig(cfg))
	cfg.TLSConfig.KeyFile = filepath.Join("testdata", "dummy-tls-key-file")
	require.NoError(t, component.ValidateConfig(cfg))
}

func TestPromHTTPClientConfigValidateMain(t *testing.T) {
	cfg := PromHTTPClientConfig{}
	require.NoError(t, component.ValidateConfig(cfg))
	cfg.BearerToken = "foo"
	cfg.BearerTokenFile = filepath.Join("testdata", "dummy-tls-key-file")
	require.Error(t, component.ValidateConfig(cfg))
}

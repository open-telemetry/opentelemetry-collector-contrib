// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateTraces(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "2").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	params := exportertest.NewNopSettings(metadata.Type)
	exporter, err := factory.CreateTraces(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}

func TestGenerateUrl(t *testing.T) {
	type generateURLTest struct {
		endpoint string
		region   string
		dataType string
		expected string
	}
	generateURLTests := []generateURLTest{
		{"", "us", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"", "us", "traces", "https://otlp-listener.logz.io/v1/traces"},
		{"", "", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"https://nonexistent.com", "", "logs", "https://nonexistent.com"},
		{"https://nonexistent.com", "us", "traces", "https://nonexistent.com"},
		{"https://nonexistent.com", "not-valid", "traces", "https://nonexistent.com"},
		{"", "not-valid", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"", "US", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"", "Us", "traces", "https://otlp-listener.logz.io/v1/traces"},
		{"", "EU", "traces", "https://otlp-listener-eu.logz.io/v1/traces"},
	}
	for _, test := range generateURLTests {
		clientConfig := confighttp.NewDefaultClientConfig()
		clientConfig.Endpoint = test.endpoint
		cfg := &Config{
			Region:       test.region,
			Token:        "token",
			ClientConfig: clientConfig,
		}
		output, _ := generateEndpoint(cfg, test.dataType)
		require.Equal(t, test.expected, output)
	}
}

func TestGetListenerURL(t *testing.T) {
	type getListenerURLTest struct {
		region   string
		dataType string
		expected string
	}
	getListenerURLTests := []getListenerURLTest{
		{"us", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"eu", "logs", "https://otlp-listener-eu.logz.io/v1/logs"},
		{"au", "logs", "https://otlp-listener-au.logz.io/v1/logs"},
		{"ca", "logs", "https://otlp-listener-ca.logz.io/v1/logs"},
		{"uk", "logs", "https://otlp-listener-uk.logz.io/v1/logs"},
		{"not-valid", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"US", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"Us", "logs", "https://otlp-listener.logz.io/v1/logs"},
		{"UK", "traces", "https://otlp-listener-uk.logz.io/v1/traces"},
	}
	for _, test := range getListenerURLTests {
		output := getListenerURL(test.region, test.dataType)
		require.Equal(t, test.expected, output)
	}
}

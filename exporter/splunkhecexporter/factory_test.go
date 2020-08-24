// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateMetricsExporterNoConfig(t *testing.T) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createMetricsExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateTraceExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createTraceExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestCreateTraceExporterNoConfig(t *testing.T) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createTraceExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateTraceExporterInvalidEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "urn:something:12345"
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createTraceExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "https://example.com:8088/services/collector"
	cfg.Token = "1234-1234"
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	exp, err := factory.CreateMetricsExporter(
		context.Background(), params,
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	cfg.Token = "testToken"
	cfg.Endpoint = "https://example.com"
	exp, err = factory.CreateMetricsExporter(
		context.Background(), params,
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestFactory_CreateMetricsExporter(t *testing.T) {
	config := &Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: typeStr,
		},
		Token:    "testToken",
		Endpoint: "https://example.com:8000",
	}

	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	te, err := createMetricsExporter(context.Background(), params, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestFactory_CreateMetricsExporterFails(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		errorMessage string
	}{
		{
			name: "empty_endpoint",
			config: &Config{
				ExporterSettings: configmodels.ExporterSettings{
					TypeVal: configmodels.Type(typeStr),
					NameVal: typeStr,
				},
				Token: "token",
			},
			errorMessage: "failed to process \"splunk_hec\" config: requires a non-empty \"endpoint\"",
		},
		{
			name: "empty_token",
			config: &Config{
				ExporterSettings: configmodels.ExporterSettings{
					TypeVal: configmodels.Type(typeStr),
					NameVal: typeStr,
				},
				Endpoint: "https://example.com:8000",
			},
			errorMessage: "failed to process \"splunk_hec\" config: requires a non-empty \"token\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := component.ExporterCreateParams{Logger: zap.NewNop()}
			te, err := createMetricsExporter(context.Background(), params, tt.config)
			assert.EqualError(t, err, tt.errorMessage)
			assert.Nil(t, te)
		})
	}
}

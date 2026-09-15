// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Tests whether or not the default Exporter factory can instantiate a properly interfaced Exporter with default conditions
func Test_createDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

// Tests whether or not a correct Metrics Exporter from the default Config parameters
func Test_createMetricsExporter(t *testing.T) {
	invalidConfig := createDefaultConfig().(*Config)
	invalidConfig.HTTP = confighttp.NewDefaultClientConfig()
	invalidTLSConfig := createDefaultConfig().(*Config)
	invalidTLSConfig.HTTP.TLS = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "nonexistent file",
			CertFile: "",
			KeyFile:  "",
		},
		Insecure:   false,
		ServerName: "",
	}
	tests := []struct {
		name                string
		cfg                 component.Config
		set                 exporter.Settings
		returnErrorOnCreate bool
		returnErrorOnStart  bool
	}{
		{
			"success_case",
			createDefaultConfig(),
			exportertest.NewNopSettings(metadata.Type),
			false,
			false,
		},
		{
			"fail_case",
			nil,
			exportertest.NewNopSettings(metadata.Type),
			true,
			false,
		},
		{
			"invalid_config_case",
			invalidConfig,
			exportertest.NewNopSettings(metadata.Type),
			true,
			false,
		},
		{
			"invalid_tls_config_case",
			invalidTLSConfig,
			exportertest.NewNopSettings(metadata.Type),
			false,
			true,
		},
		{
			"resource_constant_labels_case",
			func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.ResourceConstantLabels = resourcetotelemetry.Settings{
					Included: []string{"service*"},
				}
				return cfg
			}(),
			exportertest.NewNopSettings(metadata.Type),
			false,
			false,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := createMetricsExporter(t.Context(), tt.set, tt.cfg)
			if tt.returnErrorOnCreate {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, exp)
			err = exp.Start(t.Context(), componenttest.NewNopHost())
			if tt.returnErrorOnStart {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, exp.Shutdown(t.Context()))
		})
	}
}

func Test_createMetricsExporter_disableResourceToTelemetryConversion(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusremotewriteDisableResourceToTelemetryConversionFeatureGate, true)()

	cfg := createDefaultConfig().(*Config)
	//nolint:staticcheck // test deprecated field
	cfg.ResourceToTelemetrySettings = resourcetotelemetry.Settings{Enabled: true}

	exp, err := createMetricsExporter(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
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
	invalidConfig.ClientConfig = confighttp.NewDefaultClientConfig()
	invalidTLSConfig := createDefaultConfig().(*Config)
	invalidTLSConfig.ClientConfig.TLSSetting = configtls.ClientConfig{
		Config: configtls.Config{
			CAFile:   "non-existent file",
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
		{"success_case",
			createDefaultConfig(),
			exportertest.NewNopSettings(),
			false,
			false,
		},
		{"fail_case",
			nil,
			exportertest.NewNopSettings(),
			true,
			false,
		},
		{"invalid_config_case",
			invalidConfig,
			exportertest.NewNopSettings(),
			true,
			false,
		},
		{"invalid_tls_config_case",
			invalidTLSConfig,
			exportertest.NewNopSettings(),
			false,
			true,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := createMetricsExporter(context.Background(), tt.set, tt.cfg)
			if tt.returnErrorOnCreate {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, exp)
			err = exp.Start(context.Background(), componenttest.NewNopHost())
			if tt.returnErrorOnStart {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, exp.Shutdown(context.Background()))
		})
	}
}

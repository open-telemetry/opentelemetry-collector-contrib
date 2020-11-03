package awsprometheusremotewriteexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtls"
)

//Tests whether or not the default Exporter factory can instantiate a properly interfaced Exporter with default conditions
func Test_createDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

//Tests whether or not a correct Metrics Exporter from the default Config parameters
func Test_createMetricsExporter(t *testing.T) {
	validConfigWithAuth := createDefaultConfig().(*Config)
	validConfigWithAuth.AuthSettings = AuthSettings{Region: "region", Service: "service"}

	invalidConfigWithAuth := createDefaultConfig().(*Config)
	invalidConfigWithAuth.AuthSettings = AuthSettings{Region: "", Service: "service"}

	invalidConfig := createDefaultConfig().(*Config)
	invalidConfig.HTTPClientSettings = confighttp.HTTPClientSettings{}

	invalidTLSConfig := createDefaultConfig().(*Config)
	invalidTLSConfig.HTTPClientSettings.TLSSetting = configtls.TLSClientSetting{
		TLSSetting: configtls.TLSSetting{
			CAFile:   "non-existent file",
			CertFile: "",
			KeyFile:  "",
		},
		Insecure:   false,
		ServerName: "",
	}

	tests := []struct {
		name        string
		cfg         configmodels.Exporter
		params      component.ExporterCreateParams
		returnError bool
	}{
		{"success_case_default",
			createDefaultConfig(),
			component.ExporterCreateParams{Logger: zap.NewNop()},
			false,
		},
		{"success_case_with_auth",
			validConfigWithAuth,
			component.ExporterCreateParams{Logger: zap.NewNop()},
			false,
		},
		{"fail_case",
			nil,
			component.ExporterCreateParams{Logger: zap.NewNop()},
			true,
		},
		{"invalid_auth_case",
			invalidConfigWithAuth,
			component.ExporterCreateParams{Logger: zap.NewNop()},
			true,
		},
		{"invalid_config_case",
			invalidConfig,
			component.ExporterCreateParams{Logger: zap.NewNop()},
			true,
		},
		{"invalid_tls_config_case",
			invalidTLSConfig,
			component.ExporterCreateParams{Logger: zap.NewNop()},
			true,
		},
	}
	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := createMetricsExporter(context.Background(), tt.params, tt.cfg)
			if tt.returnError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

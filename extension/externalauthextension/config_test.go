package externalauthextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/externalauthextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr bool
	}{
		{
			id: component.NewIDWithName(metadata.Type, "valid"),
			expected: &Config{
				Endpoint:          "https://auth.example.com/validate",
				RefreshInterval:   "1h",
				Header:            "Authorization",
				ExpectedCodes:     []int{200},
				Scheme:            "Bearer",
				Method:            "POST",
				HTTPClientTimeout: 10 * time.Second,
				TelemetryType:     "traces",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metrics"),
			expected: &Config{
				Endpoint:          "https://auth.example.com/validate",
				RefreshInterval:   "1h",
				Header:            "Authorization",
				ExpectedCodes:     []int{200},
				Scheme:            "Bearer",
				Method:            "POST",
				HTTPClientTimeout: 10 * time.Second,
				TelemetryType:     "metrics",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "logs"),
			expected: &Config{
				Endpoint:          "https://auth.example.com/validate",
				RefreshInterval:   "1h",
				Header:            "Authorization",
				ExpectedCodes:     []int{200},
				Scheme:            "Bearer",
				Method:            "POST",
				HTTPClientTimeout: 10 * time.Second,
				TelemetryType:     "logs",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom_settings"),
			expected: &Config{
				Endpoint:          "https://custom-auth.example.com",
				RefreshInterval:   "30m",
				Header:            "X-Custom-Auth",
				ExpectedCodes:     []int{200, 201},
				Scheme:            "Custom",
				Method:            "GET",
				HTTPClientTimeout: 5 * time.Second,
				TelemetryType:     "traces",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "k8s_dns"),
			expected: &Config{
				Endpoint:          "dns:///auth-service.namespace.svc.cluster.local",
				RefreshInterval:   "1h",
				Header:            "Authorization",
				ExpectedCodes:     []int{200},
				Scheme:            "Bearer",
				Method:            "POST",
				HTTPClientTimeout: 10 * time.Second,
				TelemetryType:     "traces",
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missing_endpoint"),
			expectedErr: true,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_endpoint"),
			expectedErr: true,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_interval"),
			expectedErr: true,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_http_code"),
			expectedErr: true,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_telemetry_type"),
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			if tt.expectedErr {
				assert.Error(t, xconfmap.Validate(cfg))
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

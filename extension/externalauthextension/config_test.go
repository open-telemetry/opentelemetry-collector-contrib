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
	"go.opentelemetry.io/collector/component/componenttest"
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
				TokenFormat:       "raw",
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
				TokenFormat:       "raw",
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
				TokenFormat:       "raw",
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
				TokenFormat:       "raw",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "endpoint_mapping"),
			expected: &Config{
				Endpoint: "https://default-auth.example.com",
				HeaderEndpointMapping: []HeaderMapping{
					{
						Header: "Destination",
						Values: map[string]string{
							"stage": "https://stage-auth.example.com",
							"prod":  "https://prod-auth.example.com",
							"dev":   "https://dev-auth.example.com",
						},
					},
				},
				RefreshInterval:   "1h",
				Header:            "Authorization",
				ExpectedCodes:     []int{200},
				Scheme:            "Bearer",
				Method:            "POST",
				HTTPClientTimeout: 10 * time.Second,
				TelemetryType:     "traces",
				TokenFormat:       "raw",
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
				TokenFormat:       "raw",
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
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_token_format"),
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

func TestConfig_ValidateEndpointMapping(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid endpoint mapping",
			config: &Config{
				Endpoint: "https://default.example.com",
				HeaderEndpointMapping: []HeaderMapping{
					{
						Header: "Destination",
						Values: map[string]string{
							"stage": "https://stage.example.com",
							"prod":  "https://prod.example.com",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty endpoint mapping",
			config: &Config{
				Endpoint:              "https://default.example.com",
				HeaderEndpointMapping: []HeaderMapping{},
			},
			wantErr: true,
		},
		{
			name: "empty header name",
			config: &Config{
				Endpoint: "https://default.example.com",
				HeaderEndpointMapping: []HeaderMapping{
					{
						Header: "",
						Values: map[string]string{
							"stage": "https://stage.example.com",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty header value",
			config: &Config{
				Endpoint: "https://default.example.com",
				HeaderEndpointMapping: []HeaderMapping{
					{
						Header: "Destination",
						Values: map[string]string{
							"": "https://stage.example.com",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty endpoint",
			config: &Config{
				Endpoint: "https://default.example.com",
				HeaderEndpointMapping: []HeaderMapping{
					{
						Header: "Destination",
						Values: map[string]string{
							"stage": "",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid endpoint URL",
			config: &Config{
				Endpoint: "https://default.example.com",
				HeaderEndpointMapping: []HeaderMapping{
					{
						Header: "Destination",
						Values: map[string]string{
							"stage": "invalid-url",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_GetEndpointForHeaders(t *testing.T) {
	eauth := &externalauth{
		endpoint: "https://default.example.com",
		headerEndpointMapping: []HeaderMapping{
			{
				Header: "Destination",
				Values: map[string]string{
					"stage": "https://stage.example.com",
					"prod":  "https://prod.example.com",
				},
			},
		},
		telemetry: componenttest.NewNopTelemetrySettings(),
	}

	tests := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{
			name: "stage destination",
			headers: map[string][]string{
				"Destination": {"stage"},
			},
			want: "https://stage.example.com",
		},
		{
			name: "prod destination",
			headers: map[string][]string{
				"Destination": {"prod"},
			},
			want: "https://prod.example.com",
		},
		{
			name: "case insensitive header",
			headers: map[string][]string{
				"destination": {"stage"},
			},
			want: "https://stage.example.com",
		},
		{
			name: "unknown destination",
			headers: map[string][]string{
				"Destination": {"unknown"},
			},
			want: "https://default.example.com",
		},
		{
			name:    "no headers",
			headers: map[string][]string{},
			want:    "https://default.example.com",
		},
		{
			name: "unrelated header",
			headers: map[string][]string{
				"X-Custom": {"value"},
			},
			want: "https://default.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := eauth.getEndpointForHeaders(tt.headers)
			assert.Equal(t, tt.want, got)
		})
	}
}

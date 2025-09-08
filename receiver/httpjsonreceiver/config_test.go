package httpjsonreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Timeout:            10 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL:    "http://example.com/metrics",
						Method: "GET",
						Metrics: []MetricConfig{
							{
								Name:     "test_metric",
								JSONPath: "value",
								Type:     "gauge",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no endpoints",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints:          []EndpointConfig{},
			},
			wantErr: true,
		},
		{
			name: "invalid URL",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL: "://invalid-url", // This will fail parsing
						Metrics: []MetricConfig{
							{
								Name:     "test_metric",
								JSONPath: "value",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty URL",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL: "",
						Metrics: []MetricConfig{
							{
								Name:     "test_metric",
								JSONPath: "value",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no metrics",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL:     "http://example.com",
						Metrics: []MetricConfig{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty metric name",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL: "http://example.com",
						Metrics: []MetricConfig{
							{
								Name:     "",
								JSONPath: "value",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty json path",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL: "http://example.com",
						Metrics: []MetricConfig{
							{
								Name:     "test_metric",
								JSONPath: "",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid metric type",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL: "http://example.com",
						Metrics: []MetricConfig{
							{
								Name:     "test_metric",
								JSONPath: "value",
								Type:     "invalid",
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid value type",
			config: Config{
				CollectionInterval: 30 * time.Second,
				Endpoints: []EndpointConfig{
					{
						URL: "http://example.com",
						Metrics: []MetricConfig{
							{
								Name:      "test_metric",
								JSONPath:  "value",
								ValueType: "invalid",
							},
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

func TestConfigDefaults(t *testing.T) {
	config := Config{
		Endpoints: []EndpointConfig{
			{
				URL: "http://example.com",
				Metrics: []MetricConfig{
					{
						Name:     "test_metric",
						JSONPath: "value",
					},
				},
			},
		},
	}

	err := config.Validate()
	require.NoError(t, err)

	// Check defaults were applied
	assert.Equal(t, 60*time.Second, config.CollectionInterval)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, "GET", config.Endpoints[0].Method)
	assert.Equal(t, "gauge", config.Endpoints[0].Metrics[0].Type)
	assert.Equal(t, "double", config.Endpoints[0].Metrics[0].ValueType)
}

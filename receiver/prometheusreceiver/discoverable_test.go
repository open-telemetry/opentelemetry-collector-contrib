// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Discoverable(t *testing.T) {
	tests := []struct {
		name               string
		rawCfg             map[string]any
		discoveredEndpoint string
		expectError        bool
		errorContains      string
	}{
		{
			name: "valid single target matching discovered endpoint",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "discovered-app",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"10.1.2.3:8080"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        false,
		},
		{
			name: "valid target - template expansion already occurred",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "templated-app",
							"static_configs": []any{
								map[string]any{
									// Template already expanded to concrete value before validation
									"targets": []any{"10.1.2.3:9090"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:9090",
			expectError:        false,
		},
		{
			name: "valid multiple scrape configs all targeting discovered endpoint",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "app-metrics",
							"static_configs": []any{
								map[string]any{
									// Template already expanded before validation
									"targets": []any{"10.1.2.3:8080"},
								},
							},
						},
						map[string]any{
							"job_name": "app-health",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"10.1.2.3:8080"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        false,
		},
		{
			name: "invalid target not matching discovered endpoint",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "malicious-job",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"evil.com:9090"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "does not match discovered endpoint",
		},
		{
			name: "mixed valid and invalid targets",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "good-job",
							"static_configs": []any{
								map[string]any{
									// Template already expanded before validation
									"targets": []any{"10.1.2.3:8080"},
								},
							},
						},
						map[string]any{
							"job_name": "bad-job",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"attacker.com:8080"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "does not match discovered endpoint",
		},
		{
			name: "valid HTTP URL format target",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name":     "http-app",
							"scheme":       "http",
							"metrics_path": "/metrics",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"10.1.2.3:8080"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        false,
		},
		{
			name: "valid HTTPS URL format target",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name":     "https-app",
							"scheme":       "https",
							"metrics_path": "/health",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"10.1.2.3:8443"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8443",
			expectError:        false,
		},
		{
			name: "invalid HTTP URL with wrong host",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name":     "wrong-host",
							"scheme":       "http",
							"metrics_path": "/metrics",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"wrong.host:8080"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "does not match discovered endpoint",
		},
		{
			name: "missing prometheus config",
			rawCfg: map[string]any{
				"other_field": "value",
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "missing prometheus config field",
		},
		{
			name: "invalid config field format",
			rawCfg: map[string]any{
				"config": "invalid_string_instead_of_map",
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "invalid config field format",
		},
		{
			name: "invalid - scrape_config_files are not allowed",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_config_files": []any{"external.yml"},
					"scrape_configs":      []any{},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "scrape_config_files are not allowed",
		},
		{
			name: "invalid - endpoint field is set for discoverable receiver",
			rawCfg: map[string]any{
				"endpoint": "10.1.2.3:8080", // This should not be present for discoverable receivers
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "valid-job",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"10.1.2.3:8080"},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "endpoint field should not be set for discoverable receivers",
		},
		{
			name: "invalid - empty target",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "empty-target-job",
							"static_configs": []any{
								map[string]any{
									"targets": []any{""},
								},
							},
						},
					},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "target cannot be empty",
		},
		{
			name: "empty scrape configs (valid)",
			rawCfg: map[string]any{
				"config": map[string]any{
					"scrape_configs": []any{},
				},
			},
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			err := cfg.ValidateDiscovery(tt.rawCfg, tt.discoveredEndpoint)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePrometheusTarget(t *testing.T) {
	tests := []struct {
		name               string
		target             string
		discoveredEndpoint string
		expectError        bool
		errorContains      string
	}{
		{
			name:               "exact match",
			target:             "10.1.2.3:8080",
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        false,
		},
		{
			name:               "post-expansion target matches endpoint",
			target:             "10.1.2.3:8080",
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        false,
		},
		{
			name:               "template not expanded - should fail",
			target:             "`endpoint`",
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "does not match discovered endpoint",
		},
		{
			name:               "HTTP URL format matching",
			target:             "http://10.1.2.3:8080/metrics",
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        false,
		},
		{
			name:               "HTTPS URL format matching",
			target:             "https://10.1.2.3:8443/health",
			discoveredEndpoint: "10.1.2.3:8443",
			expectError:        false,
		},
		{
			name:               "mismatch host",
			target:             "evil.com:8080",
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "does not match discovered endpoint",
		},
		{
			name:               "mismatch port",
			target:             "10.1.2.3:9090",
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "does not match discovered endpoint",
		},
		{
			name:               "HTTP URL with wrong host",
			target:             "http://attacker.com:8080/metrics",
			discoveredEndpoint: "10.1.2.3:8080",
			expectError:        true,
			errorContains:      "does not match discovered endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePrometheusTarget(tt.target, tt.discoveredEndpoint)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestExtractHostFromTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected string
	}{
		{
			name:     "simple host:port",
			target:   "10.1.2.3:8080",
			expected: "10.1.2.3:8080",
		},
		{
			name:     "HTTP URL",
			target:   "http://example.com:8080/metrics",
			expected: "example.com:8080",
		},
		{
			name:     "HTTPS URL",
			target:   "https://example.com:8443/health",
			expected: "example.com:8443",
		},
		{
			name:     "IPv6 with port",
			target:   "[::1]:8080",
			expected: "[::1]:8080",
		},
		{
			name:     "hostname only",
			target:   "hostname",
			expected: "hostname",
		},
		{
			name:     "localhost with port",
			target:   "localhost:9090",
			expected: "localhost:9090",
		},
		{
			name:     "IP only",
			target:   "192.168.1.1",
			expected: "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHostFromTarget(tt.target)
			assert.Equal(t, tt.expected, result)
		})
	}
}

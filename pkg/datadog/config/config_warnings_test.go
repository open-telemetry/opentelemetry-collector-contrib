// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
)

func TestSendAggregations(t *testing.T) {
	tests := []struct {
		name              string
		cfgMap            *confmap.Conf
		expectedAggrValue bool
		warnings          []string
		err               string
	}{
		{
			name: "both metrics::histograms::send_count_sum_metrics and metrics::histograms::send_aggregation_metrics",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_count_sum_metrics":   true,
						"send_aggregation_metrics": true,
					},
				},
			}),
			err: "\"metrics::histograms::send_count_sum_metrics\" and \"metrics::histograms::send_aggregation_metrics\" can't be both set at the same time: use \"metrics::histograms::send_aggregation_metrics\" only instead",
		},
		{
			name: "metrics::histograms::send_count_sum_metrics set to true",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_count_sum_metrics": true,
					},
				},
			}),
			expectedAggrValue: true,
			warnings: []string{
				"\"metrics::histograms::send_count_sum_metrics\" has been deprecated in favor of \"metrics::histograms::send_aggregation_metrics\"",
			},
		},
		{
			name: "metrics::histograms::send_count_sum_metrics set to false",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_count_sum_metrics": false,
					},
				},
			}),
			warnings: []string{
				"\"metrics::histograms::send_count_sum_metrics\" has been deprecated in favor of \"metrics::histograms::send_aggregation_metrics\"",
			},
		},
		{
			name:              "metrics::histograms::send_count_sum_metrics and metrics::histograms::send_aggregation_metrics unset",
			cfgMap:            confmap.New(),
			expectedAggrValue: false,
		},
		{
			name: "metrics::histograms::send_aggregation_metrics set",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"metrics": map[string]any{
					"histograms": map[string]any{
						"send_aggregation_metrics": true,
					},
				},
			}),
			expectedAggrValue: true,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := CreateDefaultConfig().(*Config)
			err := testInstance.cfgMap.Unmarshal(cfg)
			if err != nil || testInstance.err != "" {
				assert.ErrorContains(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.expectedAggrValue, cfg.Metrics.HistConfig.SendAggregations)
				var warningStr []string
				for _, warning := range cfg.warnings {
					warningStr = append(warningStr, warning.Error())
				}
				assert.ElementsMatch(t, testInstance.warnings, warningStr)
			}
		})
	}
}

func TestPeerTags(t *testing.T) {
	tests := []struct {
		name                  string
		cfgMap                *confmap.Conf
		expectedPeerTagsValue bool
		warnings              []string
		err                   string
	}{
		{
			name: "both traces::peer_service_aggregation and traces::peer_tags_aggregation",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"traces": map[string]any{
					"peer_service_aggregation": true,
					"peer_tags_aggregation":    true,
				},
			}),
			err: "\"traces::peer_service_aggregation\" and \"traces::peer_tags_aggregation\" can't be both set at the same time: use \"traces::peer_tags_aggregation\" only instead",
		},
		{
			name: "traces::peer_service_aggregation set to true",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"traces": map[string]any{
					"peer_service_aggregation": true,
				},
			}),
			expectedPeerTagsValue: true,
			warnings: []string{
				"\"traces::peer_service_aggregation\" has been deprecated in favor of \"traces::peer_tags_aggregation\"",
			},
		},
		{
			name: "traces::peer_service_aggregation set to false",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"traces": map[string]any{
					"peer_service_aggregation": false,
				},
			}),
			warnings: []string{
				"\"traces::peer_service_aggregation\" has been deprecated in favor of \"traces::peer_tags_aggregation\"",
			},
		},
		{
			name:                  "traces::peer_service_aggregation and traces::peer_tags_aggregation unset",
			cfgMap:                confmap.New(),
			expectedPeerTagsValue: true,
		},
		{
			name: "traces::peer_tags_aggregation set",
			cfgMap: confmap.NewFromStringMap(map[string]any{
				"traces": map[string]any{
					"peer_tags_aggregation": true,
				},
			}),
			expectedPeerTagsValue: true,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			cfg := CreateDefaultConfig().(*Config)
			err := testInstance.cfgMap.Unmarshal(cfg)
			if err != nil || testInstance.err != "" {
				assert.ErrorContains(t, err, testInstance.err)
			} else {
				assert.Equal(t, testInstance.expectedPeerTagsValue, cfg.Traces.PeerTagsAggregation)
				var warningStr []string
				for _, warning := range cfg.warnings {
					warningStr = append(warningStr, warning.Error())
				}
				assert.ElementsMatch(t, testInstance.warnings, warningStr)
			}
		})
	}
}

func TestDeprecateHostnameSourceFirstResource(t *testing.T) {
	cfg := CreateDefaultConfig().(*Config)
	cfgMap := confmap.NewFromStringMap(map[string]any{
		"host_metadata": map[string]any{
			"hostname_source": "first_resource",
		},
	})
	err := cfgMap.Unmarshal(cfg)
	require.NoError(t, err)
	assert.Len(t, cfg.warnings, 1)
	assert.ErrorContains(t, cfg.warnings[0], "first_resource is deprecated, opt in to https://docs.datadoghq.com/opentelemetry/mapping/host_metadata/ instead")
}

func TestAddWarning(t *testing.T) {
	cfg := CreateDefaultConfig().(*Config)

	// Add a warning
	cfg.AddWarning(errors.New("test warning"))

	// Verify warning was added
	assert.Len(t, cfg.warnings, 1)
	assert.Equal(t, "test warning", cfg.warnings[0].Error())
}

func TestGetWarnings(t *testing.T) {
	cfg := CreateDefaultConfig().(*Config)
	cfg.warnings = []error{errors.New("test warning")}

	// Get warnings
	warnings := cfg.GetWarnings()

	// Verify warnings are returned correctly
	assert.Len(t, warnings, 1)
	assert.Equal(t, "test warning", warnings[0].Error())
}

func TestGetWarningsReturnsACopy(t *testing.T) {
	cfg := CreateDefaultConfig().(*Config)
	originalWarning := errors.New("original warning")
	cfg.warnings = []error{originalWarning}

	// Get warnings
	warnings := cfg.GetWarnings()

	// Modify the returned slice
	warnings = append(warnings, errors.New("new warning"))

	// Verify original warnings are unchanged
	assert.Len(t, cfg.warnings, 1)
	assert.Equal(t, "original warning", cfg.warnings[0].Error())

	// Verify the returned slice was modified
	assert.Len(t, warnings, 2)
}

func TestAddWarningf(t *testing.T) {
	cfg := CreateDefaultConfig().(*Config)

	// Add formatted warning
	cfg.AddWarningf("hostname \"%s\" is ignored", "test-host")

	// Verify warning was added with correct formatting
	assert.Len(t, cfg.warnings, 1)
	assert.Equal(t, "hostname \"test-host\" is ignored", cfg.warnings[0].Error())
}

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cardinalityguardianprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		expectedErr string
	}{
		{
			name: "valid config",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				EstimatedCostPerMetricMonth: 0.10,
			},
			expectedErr: "",
		},
		{
			name: "invalid max_cardinality_delta_per_epoch",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 0,
				EpochDurationSeconds:        300,
			},
			expectedErr: "max_cardinality_delta_per_epoch must be greater than 0",
		},
		{
			name: "invalid epoch_duration_seconds",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        5,
			},
			expectedErr: "epoch_duration_seconds must be at least 10",
		},
		{
			name: "invalid estimated_cost_per_metric_month",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				EstimatedCostPerMetricMonth: -0.5,
			},
			expectedErr: "estimated_cost_per_metric_month cannot be negative",
		},
		{
			name: "invalid top_offenders_count negative",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				TopOffendersCount:           -1,
			},
			expectedErr: "top_offenders_count must be between 0 and 500",
		},
		{
			name: "invalid top_offenders_count too large",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				TopOffendersCount:           501,
			},
			expectedErr: "top_offenders_count must be between 0 and 500",
		},
		{
			name: "invalid max_tracker_count negative",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				MaxTrackerCount:             -1,
			},
			expectedErr: "max_tracker_count must be between 0 and 10,000,000",
		},
		{
			name: "invalid max_tracker_count too large",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				MaxTrackerCount:             10000001,
			},
			expectedErr: "max_tracker_count must be between 0 and 10,000,000",
		},
		{
			name: "invalid metric_overrides empty name",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				MetricOverrides: map[string]int{
					"": 100,
				},
			},
			expectedErr: "metric_overrides contains an empty metric name",
		},
		{
			name: "invalid metric_overrides zero limit",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				MetricOverrides: map[string]int{
					"http.request": 0,
				},
			},
			expectedErr: "metric_overrides[\"http.request\"] must be greater than 0",
		},
		{
			name: "invalid drop_log_max_per_epoch",
			cfg: &Config{
				MaxCardinalityDeltaPerEpoch: 50,
				EpochDurationSeconds:        300,
				DropLogMaxPerEpoch:          -1,
			},
			expectedErr: "drop_log_max_per_epoch must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

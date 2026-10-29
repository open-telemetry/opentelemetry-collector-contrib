// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlquery

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricCfg_Validate_RowCondition(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		cfg       MetricCfg
		expectErr string
	}{
		{
			name: "valid row_condition with column set",
			cfg: MetricCfg{
				MetricName:  "my.metric",
				ValueColumn: "val",
				ValueType:   MetricValueTypeInt,
				DataType:    MetricTypeGauge,
				RowCondition: &RowCondition{
					Column: "list",
					Value:  "pools",
				},
			},
		},
		{
			name: "row_condition with empty column is invalid",
			cfg: MetricCfg{
				MetricName:  "my.metric",
				ValueColumn: "val",
				ValueType:   MetricValueTypeInt,
				DataType:    MetricTypeGauge,
				RowCondition: &RowCondition{
					Column: "",
					Value:  "pools",
				},
			},
			expectErr: "'row_condition.column' cannot be empty",
		},
		{
			name: "nil row_condition is valid",
			cfg: MetricCfg{
				MetricName:   "my.metric",
				ValueColumn:  "val",
				ValueType:    MetricValueTypeInt,
				DataType:     MetricTypeGauge,
				RowCondition: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.expectErr != "" {
				require.ErrorContains(t, err, tt.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

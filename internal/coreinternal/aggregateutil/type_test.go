// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregateutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_AggregationType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		in   AggregationType
		want bool
	}{
		{
			name: "valid",
			in:   Mean,
			want: true,
		},

		{
			name: "invalid",
			in:   AggregationType("invalid"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.in.IsValid())
		})
	}
}

func Test_AggregationType_Convert(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    AggregationType
		wantErr string
	}{
		{
			name:    "valid",
			in:      "mean",
			want:    Mean,
			wantErr: "",
		},

		{
			name:    "invalid",
			in:      "invalid",
			want:    "invalid",
			wantErr: "unsupported function: 'invalid'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToAggregationFunction(tt.in)
			require.Equal(t, tt.want, got)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_GetSupportedAggregationFunctionsList(t *testing.T) {
	require.Equal(t, "sum, mean, min, max, median, count", GetSupportedAggregationFunctionsList())
}

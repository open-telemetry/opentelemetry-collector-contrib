// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

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

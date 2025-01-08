// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshalertest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNewNopMetrics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		metrics pmetric.Metrics
		err     error
	}{
		{
			name:    "no error",
			metrics: pmetric.NewMetrics(),
			err:     nil,
		},
		{
			name:    "with error",
			metrics: pmetric.NewMetrics(),
			err:     errors.New("test error"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unmarshaler := NewNopMetrics(test.metrics, test.err)
			got, err := unmarshaler.UnmarshalMetrics(nil)
			require.Equal(t, test.err, err)
			require.Equal(t, test.metrics, got)
			require.Equal(t, typeStr, unmarshaler.Type())
		})
	}
}

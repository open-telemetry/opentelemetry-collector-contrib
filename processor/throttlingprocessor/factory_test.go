// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package throttlingprocessor

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/throttlingprocessor/internal/metadata"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestNewProcessorFactory(t *testing.T) {
	f := NewFactory()
	require.Equal(t, metadata.Type, f.Type())
	require.Equal(t, metadata.LogsStability, f.LogsStability())
	require.NotNil(t, f.CreateDefaultConfig())
	require.NotNil(t, f.CreateLogs)
}

func TestCreateLogs(t *testing.T) {
	testCases := []struct {
		name        string
		cfg         component.Config
		expectedErr string
	}{
		{
			name:        "invalid config type",
			cfg:         nil,
			expectedErr: "invalid config type",
		},
		{
			name: "valid custom condition",
			cfg: &Config{
				Interval:      5 * time.Second,
				Threshold:     1,
				KeyExpression: `Concat(["a", "b"], "_")`,
				Conditions:    []string{"false"},
			},
		},
		{
			name: "valid multiple conditions",
			cfg: &Config{
				Interval:      5 * time.Second,
				Threshold:     1,
				KeyExpression: `Concat(["a", "b"], "_")`,
				Conditions:    []string{"false", `(attributes["ID"] == 1)`},
			},
		},
		{
			name: "invalid condition",
			cfg: &Config{
				Interval:      5 * time.Second,
				Threshold:     1,
				KeyExpression: `Concat(["a", "b"], "_")`,
				Conditions:    []string{"x"},
			},
			expectedErr: "invalid condition",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFactory()
			p, err := f.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), tc.cfg, nil)
			if tc.expectedErr == "" {
				require.NoError(t, err)
				require.IsType(t, &throttlingProcessor{}, p)
			} else {
				require.ErrorContains(t, err, tc.expectedErr)
				require.Nil(t, p)
			}
		})
	}
}

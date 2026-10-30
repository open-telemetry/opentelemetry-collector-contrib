// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor/internal/metadata"
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
		conditions  []string
		invalidType bool
		expectedErr string
	}{
		{
			name: "valid config",
		},
		{
			name:        "invalid config type",
			invalidType: true,
			expectedErr: "invalid config type",
		},
		{
			name:       "valid custom condition",
			conditions: []string{"false"},
		},
		{
			name:       "valid multiple conditions",
			conditions: []string{"false", `(attributes["ID"] == 1)`},
		},
		{
			name:       "valid path-context log condition",
			conditions: []string{`log.attributes["ID"] == 1`},
		},
		{
			name:       "valid path-context resource condition",
			conditions: []string{`resource.attributes["service.name"] == "my-service"`},
		},
		{
			name:       "valid path-context body condition",
			conditions: []string{`log.body == "x"`},
		},
		{
			name:       "valid mixed legacy and path-context conditions",
			conditions: []string{`attributes["ID"] == 1`, `log.attributes["ID"] == 2`},
		},
		{
			name:        "invalid condition",
			conditions:  []string{"x"},
			expectedErr: "invalid condition",
		},
		{
			name:        "invalid context name",
			conditions:  []string{`span.attributes["x"] == 1`},
			expectedErr: "invalid condition",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig()
			cfg.(*Config).Conditions = tc.conditions
			if tc.invalidType {
				cfg = nil
			}
			f := NewFactory()
			p, err := f.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nil)
			if tc.expectedErr == "" {
				require.NoError(t, err)
				require.IsType(t, &logDedupProcessor{}, p)
			} else {
				require.ErrorContains(t, err, tc.expectedErr)
				require.Nil(t, p)
			}
		})
	}
}

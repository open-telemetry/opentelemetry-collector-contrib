// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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
	var testCases = []struct {
		name        string
		cfg         component.Config
		expectedErr string
	}{
		{
			name: "valid config",
			cfg:  createDefaultConfig().(*Config),
		},
		{
			name:        "invalid config type",
			cfg:         nil,
			expectedErr: "invalid config type",
		},
		{
			name: "valid custom condition",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{"false"},
			},
		},
		{
			name: "valid multiple conditions",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{"false", `(attributes["ID"] == 1)`},
			},
		},
		{
			name: "invalid condition",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{"x"},
			},
			expectedErr: "invalid condition",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFactory()
			p, err := f.CreateLogs(context.Background(), processortest.NewNopSettings(), tc.cfg, nil)
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

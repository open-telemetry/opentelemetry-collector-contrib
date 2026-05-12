// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
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
	testCases := []struct {
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
			name: "valid path-context log condition",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{`log.attributes["ID"] == 1`},
			},
		},
		{
			name: "valid path-context resource condition",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{`resource.attributes["service.name"] == "my-service"`},
			},
		},
		{
			name: "valid path-context body condition",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{`log.body == "x"`},
			},
		},
		{
			name: "valid mixed legacy and path-context conditions",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{`attributes["ID"] == 1`, `log.attributes["ID"] == 2`},
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
		{
			name: "invalid context name",
			cfg: &Config{
				LogCountAttribute: defaultLogCountAttribute,
				Interval:          defaultInterval,
				Timezone:          defaultTimezone,
				ExcludeFields:     []string{},
				Conditions:        []string{`span.attributes["x"] == 1`},
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
				require.IsType(t, &logDedupProcessor{}, p)
			} else {
				require.ErrorContains(t, err, tc.expectedErr)
				require.Nil(t, p)
			}
		})
	}
}

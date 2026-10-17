// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logdedupprocessor

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateDefaultProcessorConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.Equal(t, defaultInterval, cfg.Interval)
	require.Equal(t, defaultLogCountAttribute, cfg.LogCountAttribute)
	require.Equal(t, defaultTimezone, cfg.Timezone)
	require.Equal(t, defaultTimestampMode, cfg.TimestampMode)
	require.Equal(t, []string{}, cfg.ExcludeFields)
}

func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		desc        string
		configure   func(*Config)
		expectedErr error
	}{
		{
			desc: "invalid LogCountAttribute config",
			configure: func(cfg *Config) {
				cfg.LogCountAttribute = ""
			},
			expectedErr: errInvalidLogCountAttribute,
		},
		{
			desc: "invalid Interval config",
			configure: func(cfg *Config) {
				cfg.Interval = -1
			},
			expectedErr: errInvalidInterval,
		},
		{
			desc: "invalid Timezone config",
			configure: func(cfg *Config) {
				cfg.Timezone = "not a timezone"
			},
			expectedErr: errors.New("timezone is invalid"),
		},
		{
			desc: "invalid exclude entire body",
			configure: func(cfg *Config) {
				cfg.ExcludeFields = []string{bodyField}
			},
			expectedErr: errCannotExcludeBody,
		},
		{
			desc: "invalid exclude field body",
			configure: func(cfg *Config) {
				cfg.ExcludeFields = []string{"not.value"}
			},
			expectedErr: errors.New("an excludefield must start with"),
		},
		{
			desc: "invalid duplicate exclude field",
			configure: func(cfg *Config) {
				cfg.ExcludeFields = []string{"body.thing", "body.thing"}
			},
			expectedErr: errors.New("duplicate exclude_field"),
		},
		{
			desc: "invalid include_fields using entire body",
			configure: func(cfg *Config) {
				cfg.IncludeFields = []string{bodyField}
			},
			expectedErr: errors.New("cannot include the entire body"),
		},
		{
			desc: "invalid include_fields not starting with body or attributes",
			configure: func(cfg *Config) {
				cfg.IncludeFields = []string{"not.valid"}
			},
			expectedErr: errors.New("an include_fields must start with body or attributes"),
		},
		{
			desc: "empty include_fields is the default behavior",
			configure: func(cfg *Config) {
				cfg.IncludeFields = []string{}
			},
			expectedErr: nil,
		},
		{
			desc: "empty timestamp_mode",
			configure: func(cfg *Config) {
				cfg.TimestampMode = ""
			},
			expectedErr: errInvalidTimestampMode,
		},
		{
			desc: "invalid timestamp_mode",
			configure: func(cfg *Config) {
				cfg.TimestampMode = "invalid"
			},
			expectedErr: errInvalidTimestampMode,
		},
		{
			desc: "valid timestamp_mode observed",
			configure: func(cfg *Config) {
				cfg.TimestampMode = TimestampModeObserved
			},
			expectedErr: nil,
		},
		{
			desc: "valid timestamp_mode preserved",
			configure: func(cfg *Config) {
				cfg.TimestampMode = TimestampModePreserved
			},
			expectedErr: nil,
		},
		{
			desc: "valid config",
			configure: func(cfg *Config) {
				cfg.ExcludeFields = []string{"body.thing", "attributes.otherthing"}
			},
			expectedErr: nil,
		},
		{
			desc: "valid config include_fields",
			configure: func(cfg *Config) {
				cfg.IncludeFields = []string{"body.thing", "attributes.otherthing"}
			},
			expectedErr: nil,
		},
		{
			desc: "invalid config defines both exclude_fields and include_fields",
			configure: func(cfg *Config) {
				cfg.ExcludeFields = []string{"body.thing", "attributes.otherthing"}
				cfg.IncludeFields = []string{"body.thing", "attributes.otherthing"}
			},
			expectedErr: errors.New("cannot define both exclude_fields and include_fields"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tc.configure(cfg)
			err := cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
